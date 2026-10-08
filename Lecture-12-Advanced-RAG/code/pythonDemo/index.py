"""Indexing: knowledge/*.pdf -> pages -> recursive chunks -> category metadata
          -> OpenAI embeddings -> Pinecone  (+ local chunk store for BM25)

Every chunk is stored twice, under the SAME ID:
  1. Pinecone   : dense vector + metadata (text, source, page, category)
                  -> semantic search, filterable by category
  2. chunks.json: chunk text + the same metadata
                  -> BM25 keyword search at query time

Category comes from the PDF file name: knowledge/graphs.pdf -> "graphs".

IDs, metadata and chunks.json use exactly the same format as the Go version,
so an index built by either one can be queried by the other.
"""

from __future__ import annotations

import glob
import json
import os
from collections import Counter
from dataclasses import asdict, dataclass, field

from pypdf import PdfReader

from common import (
    EMBEDDING_DIM,
    EMBEDDING_MODEL,
    connect_index,
    embed_texts,
    namespace,
    new_openai_client,
)

KNOWLEDGE_GLOB = "./knowledge/*.pdf"  # file name (without .pdf) becomes the category
CHUNK_STORE_PATH = "./chunks.json"  # local copy of every chunk, used for BM25
CHUNK_SIZE = 1000
CHUNK_OVERLAP = 200
EMBED_BATCH_SIZE = 100  # 100 chunks ≈ 25k tokens, well under OpenAI's per-request limit
UPSERT_BATCH_SIZE = 50  # 1536 floats as JSON ≈ 20 KB/vector; keeps requests under Pinecone's 2 MB cap


@dataclass
class Document:
    """Mirrors LangChain's Document: text plus metadata, plus a stable ID."""

    page_content: str
    metadata: dict = field(default_factory=dict)
    id: str = ""


@dataclass
class ChunkRecord:
    """One chunk as persisted in chunks.json."""

    id: str
    text: str
    source: str
    page: int
    category: str


# ==========================================================
# Step 1 : Read PDFs (one Document per page, like PyPDFLoader)
# ==========================================================


def category_from_path(path: str) -> str:
    """'./knowledge/Dynamic Programming.pdf' -> 'dynamic_programming'."""
    base = os.path.splitext(os.path.basename(path))[0].lower()
    out: list[str] = []
    last_underscore = False
    for ch in base:
        if ch.isalnum():
            out.append(ch)
            last_underscore = False
        elif not last_underscore:
            out.append("_")
            last_underscore = True
    return "".join(out).strip("_") or "general"


def load_pdf(path: str) -> list[Document]:
    try:
        reader = PdfReader(path)
    except Exception as exc:
        raise RuntimeError(f"open pdf {path}: {exc}") from exc

    category = category_from_path(path)
    docs = []
    for i, page in enumerate(reader.pages):
        docs.append(
            Document(
                page_content=page.extract_text() or "",
                metadata={
                    "source": path,
                    "page": i,  # 0-indexed, like PyPDFLoader
                    "category": category,
                },
            )
        )
    return docs


def load_knowledge_base(pattern: str) -> list[Document]:
    paths = sorted(glob.glob(pattern))
    if not paths:
        raise RuntimeError(
            f"no PDFs match {pattern!r} (put your PDFs in ./knowledge; "
            "the file name becomes the category)"
        )

    all_docs: list[Document] = []
    for p in paths:
        docs = load_pdf(p)
        print(f"  📄 {os.path.basename(p):<35} category={category_from_path(p):<20} pages={len(docs)}")
        all_docs.extend(docs)
    return all_docs


# ==========================================================
# Step 2 : Recursive Character Text Splitter
# ==========================================================
#
# Port of LangChain's RecursiveCharacterTextSplitter:
#   - try separators in order ("\n\n", "\n", " ", "")
#   - split on the first one present, keeping the separator at the start
#     of each following piece
#   - pieces still larger than chunk_size are split recursively
#   - small pieces are merged back into chunks of <= chunk_size with
#     chunk_overlap characters of overlap


class RecursiveSplitter:
    def __init__(self, chunk_size: int, chunk_overlap: int, separators: list[str] | None = None):
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap
        self.separators = separators or ["\n\n", "\n", " ", ""]

    def split_documents(self, docs: list[Document]) -> list[Document]:
        out = []
        for d in docs:
            for chunk in self._split_text(d.page_content, self.separators):
                out.append(Document(page_content=chunk, metadata=dict(d.metadata)))
        return out

    def _split_text(self, text: str, separators: list[str]) -> list[str]:
        separator = separators[-1]
        remaining: list[str] = []
        for i, sep in enumerate(separators):
            if sep == "":
                separator = ""
                break
            if sep in text:
                separator = sep
                remaining = separators[i + 1 :]
                break

        if separator == "":
            splits = list(text)
        else:
            splits = []
            for i, part in enumerate(text.split(separator)):
                if i > 0:
                    part = separator + part
                if part:
                    splits.append(part)

        final: list[str] = []
        good: list[str] = []
        for piece in splits:
            if len(piece) < self.chunk_size:
                good.append(piece)
                continue
            if good:
                final.extend(self._merge_splits(good, ""))
                good = []
            if not remaining:
                final.append(piece)
            else:
                final.extend(self._split_text(piece, remaining))
        if good:
            final.extend(self._merge_splits(good, ""))
        return final

    def _merge_splits(self, splits: list[str], separator: str) -> list[str]:
        sep_len = len(separator)
        docs: list[str] = []
        current: list[str] = []
        total = 0

        def join_current() -> None:
            doc = separator.join(current).strip()
            if doc:
                docs.append(doc)

        def extra_sep() -> int:
            return sep_len if current else 0

        for d in splits:
            length = len(d)
            if total + length + extra_sep() > self.chunk_size and current:
                join_current()
                while current and (
                    total > self.chunk_overlap
                    or (total + length + extra_sep() > self.chunk_size and total > 0)
                ):
                    total -= len(current[0]) + (sep_len if len(current) > 1 else 0)
                    current.pop(0)
            current.append(d)
            total += length
            if len(current) > 1:
                total += sep_len
        join_current()
        return docs


def assign_chunk_ids(docs: list[Document]) -> None:
    """Deterministic IDs like 'graphs-p12-c3'.

    Re-indexing overwrites vectors instead of duplicating them, and
    Pinecone + chunks.json always agree on IDs (needed to fuse results).
    """
    counters: Counter[str] = Counter()
    for d in docs:
        key = f"{d.metadata['category']}-p{d.metadata['page']}"
        d.id = f"{key}-c{counters[key]}"
        counters[key] += 1


# ==========================================================
# Step 3 : Local chunk store (for BM25 keyword search)
# ==========================================================


def save_chunk_store(path: str, docs: list[Document]) -> None:
    records = [
        ChunkRecord(
            id=d.id,
            text=d.page_content,
            source=str(d.metadata.get("source", "")),
            page=int(d.metadata.get("page", 0)),
            category=str(d.metadata.get("category", "")),
        )
        for d in docs
    ]
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump([asdict(r) for r in records], f, ensure_ascii=False, indent=2)
    os.replace(tmp, path)  # atomic: a crash never leaves a half-written store


def load_chunk_store(path: str) -> list[ChunkRecord]:
    try:
        with open(path, encoding="utf-8") as f:
            raw = json.load(f)
    except FileNotFoundError:
        raise RuntimeError(f"{path} not found, run the index command first") from None
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"decode chunk store: {exc}") from exc
    return [ChunkRecord(**r) for r in raw]


def categories_of(records: list[ChunkRecord]) -> list[str]:
    """Sorted, unique categories present in the store."""
    return sorted({r.category for r in records if r.category})


# ==========================================================
# Step 4 : Upsert one batch into Pinecone
# ==========================================================


def upsert_batch(index, batch: list[Document], vecs: list[list[float]]) -> None:
    vectors = [
        {
            "id": doc.id,
            "values": vec,
            # "text" is LangChain's default text_key
            "metadata": {"text": doc.page_content, **doc.metadata},
        }
        for doc, vec in zip(batch, vecs)
    ]
    for i in range(0, len(vectors), UPSERT_BATCH_SIZE):
        index.upsert(vectors=vectors[i : i + UPSERT_BATCH_SIZE], namespace=namespace())


# ==========================================================
# index command
# ==========================================================


def run_index() -> None:
    # Read PDFs
    print("Loading knowledge base...")
    raw_docs = load_knowledge_base(KNOWLEDGE_GLOB)
    print("✅ PDFs Loaded")
    print(f"Pages Found : {len(raw_docs)}")

    # Chunk
    chunked = RecursiveSplitter(CHUNK_SIZE, CHUNK_OVERLAP).split_documents(raw_docs)
    if not chunked:
        raise RuntimeError("no text extracted from PDFs (are they scanned/image-only?)")
    assign_chunk_ids(chunked)

    print("✅ Chunking Completed")
    print(f"Chunks Created : {len(chunked)}")
    per_category = Counter(d.metadata["category"] for d in chunked)
    for cat in sorted(per_category):
        print(f"  • {cat:<20} {per_category[cat]} chunks")

    print()
    print(f"First Chunk (id={chunked[0].id})")
    print("----------------------------------------")
    print(chunked[0].page_content[:300])
    print("----------------------------------------")

    # Clients
    ai = new_openai_client()
    print(f"✅ Embedding Model Configured ({EMBEDDING_MODEL}, {EMBEDDING_DIM}-dim)")

    index = connect_index()
    print("✅ Pinecone Connected")
    print()
    print("Index Statistics")
    print(index.describe_index_stats())

    # Embed + store in batches
    for i in range(0, len(chunked), EMBED_BATCH_SIZE):
        batch = chunked[i : i + EMBED_BATCH_SIZE]
        try:
            vecs = embed_texts(ai, [d.page_content for d in batch])
        except Exception as exc:
            raise RuntimeError(f"embed batch starting at {i}: {exc}") from exc
        try:
            upsert_batch(index, batch, vecs)
        except Exception as exc:
            raise RuntimeError(f"upsert batch starting at {i}: {exc}") from exc
        print(f"Uploaded {i + len(batch)} documents")

    # Local copy for keyword (BM25) search
    save_chunk_store(CHUNK_STORE_PATH, chunked)
    print(f"✅ Chunk store written to {CHUNK_STORE_PATH}")

    print()
    print("===================================")
    print("🎉 Successfully Indexed Documents")
    print(f"Stored {len(chunked)} Chunks across {len(per_category)} categories")
    print("===================================")