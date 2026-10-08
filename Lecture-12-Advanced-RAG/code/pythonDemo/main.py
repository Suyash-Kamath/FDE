"""Python RAG system using OpenAI + Pinecone (port of the Go version).

    python main.py index   knowledge/*.pdf -> chunks -> embeddings -> Pinecone + chunks.json
    python main.py chat    category -> rewrite -> semantic + BM25 -> RRF -> rerank -> answer

Files:
    common.py  shared config, clients, embedding call
    index.py   PDF loading, chunking, upload, chunk store
    hybrid.py  Candidate, BM25 keyword search, Reciprocal Rank Fusion
    query.py   the chat pipeline
"""

from __future__ import annotations

import sys

USAGE = """Usage:
  python main.py index   Index the PDFs in ./knowledge into Pinecone
  python main.py chat    Start the RAG chat loop"""


def main() -> int:
    try:
        from dotenv import load_dotenv  # optional; real env vars also work

        load_dotenv()
    except ImportError:
        pass

    if len(sys.argv) < 2 or sys.argv[1] not in ("index", "chat"):
        print(USAGE, file=sys.stderr)
        return 2

    try:
        if sys.argv[1] == "index":
            from index import run_index

            run_index()
        else:
            from query import run_chat

            run_chat()
    except KeyboardInterrupt:
        print("\nBye!")
    except Exception as exc:  # same behaviour as Go's log.Fatal(err)
        print(f"❌ {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())