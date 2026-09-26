#  https://certain-mechanic-42c.notion.site/RAG-System-23c3a78e0e22801caa04d16f95df1825


import os
import re
import time

from dotenv import load_dotenv

# -------------------------------
# LangChain PDF Loader
# -------------------------------
from langchain_community.document_loaders import PyPDFLoader

# -------------------------------
# LangChain Text Splitter
# -------------------------------
from langchain_text_splitters import RecursiveCharacterTextSplitter

# -------------------------------
# Gemini Embedding Model
# -------------------------------
from langchain_google_genai import GoogleGenerativeAIEmbeddings

# -------------------------------
# Pinecone Client
# -------------------------------
from pinecone import Pinecone

# -------------------------------
# LangChain Pinecone Integration
# -------------------------------
from langchain_pinecone import PineconeVectorStore

# ==========================================================
# Load Environment Variables
# ==========================================================

load_dotenv()

# ==========================================================
# Helper : Retry a batch upload on Gemini 429 rate limit errors
# ==========================================================

def upload_batch_with_backoff(vector_store, batch, max_retries=5):
    """
    The Gemini free tier caps embed_content calls at a small number
    of requests per minute. Each chunk in a batch triggers its own
    embed_content call, so a burst of batches can trip that limit
    even though our own BATCH_SIZE is small.

    Google's error response tells us exactly how long to wait
    (e.g. "Please retry in 5.48s"), so we parse that and wait a
    little longer than asked, then retry the SAME batch. Since the
    embedding call fails before anything is written to Pinecone,
    retrying is safe and won't create duplicate vectors.
    """

    for attempt in range(1, max_retries + 1):

        try:
            vector_store.add_documents(batch)
            return

        except Exception as e:

            message = str(e)

            is_rate_limit = "RESOURCE_EXHAUSTED" in message or "429" in message

            if not is_rate_limit:
                raise

            match = re.search(r"retry in ([\d.]+)s", message)

            wait_seconds = float(match.group(1)) + 2 if match else attempt * 10

            print(f"⏳ Rate limited by Gemini API. Waiting {wait_seconds:.1f}s "
                  f"(attempt {attempt}/{max_retries})...")

            time.sleep(wait_seconds)

    raise RuntimeError(
        "Gave up after repeated rate-limit errors. "
        "Wait a minute and rerun, or lower BATCH_SIZE."
    )


# ==========================================================
# Main Function
# ==========================================================

def index_document():

    # ==========================================================
    # Step 1 : Read PDF
    # ==========================================================

    PDF_PATH = "./Dsa.pdf"

    pdf_loader = PyPDFLoader(PDF_PATH)
    raw_docs = pdf_loader.load()

    print("✅ PDF Loaded")
    print(f"Pages Found : {len(raw_docs)}")

    # ==========================================================
    # Step 2 : Chunk Documents
    # ==========================================================

    """
    Why Chunking?

    LLMs and Embedding Models cannot efficiently process
    huge documents.

    Therefore we divide every document into smaller pieces.

    Chunk Size = 1000 characters
    Overlap = 200 characters

    Example

    Chunk 1
    1 -----------------------> 1000

    Chunk 2
    801 ---------------------> 1800

    Notice that characters 801 -> 1000 are present in BOTH chunks.
    This prevents context loss.
    """

    text_splitter = RecursiveCharacterTextSplitter(
        chunk_size=1000,
        chunk_overlap=200
    )

    chunked_docs = text_splitter.split_documents(raw_docs)

    print("✅ Chunking Completed")
    print(f"Chunks Created : {len(chunked_docs)}")
    print()
    print("First Chunk")
    print("----------------------------------------")
    print(chunked_docs[0].page_content[:300])
    print("----------------------------------------")

    # ==========================================================
    # Step 3 : Configure Embedding Model
    # ==========================================================

    """
    Embeddings convert text into vectors.

    Here we only configure the model.
    Actual embedding generation happens later when
    add_documents() is called.

    gemini-embedding-001 outputs 3072 dimensions by default. Our Pinecone
    index was created with dimension 768, so we ask for a truncated
    768-dim output. This is safe because the model is trained with
    Matryoshka Representation Learning (MRL): a shorter prefix of the
    full embedding is still a valid, meaningful embedding on its own.

    We pass output_dimensionality AND truncate manually in the wrapper
    below, so this keeps working even if a given langchain-google-genai
    version doesn't honor the constructor argument on its own.
    """

    class Gemini768Embeddings(GoogleGenerativeAIEmbeddings):
        def embed_documents(self, texts):
            return [v[:768] for v in super().embed_documents(texts)]

        def embed_query(self, text):
            return super().embed_query(text)[:768]

    embeddings = Gemini768Embeddings(
        google_api_key=os.getenv("GEMINI_API_KEY"),
        model="models/gemini-embedding-001",
        output_dimensionality=768
    )

    print("✅ Embedding Model Configured (768-dim)")

    # ==========================================================
    # Step 4 : Connect to Pinecone
    # ==========================================================

    pinecone = Pinecone(
        api_key=os.getenv("PINECONE_API_KEY")
    )

    index_name = os.getenv("PINECONE_INDEX_NAME")

    print()
    print("Index Name :", index_name)

    pinecone_index = pinecone.Index(index_name)

    print("✅ Pinecone Connected")

    stats = pinecone_index.describe_index_stats()

    print()
    print("Index Statistics")
    print(stats)

    # ==========================================================
    # Step 5 : Create Vector Store
    # ==========================================================

    """
    PineconeVectorStore is simply a wrapper around Pinecone.

    It knows
    1. Which embedding model to use.
    2. Which Pinecone index to use.

    Whenever we insert documents, LangChain automatically does:

    Text -> Embedding -> Pinecone

    We never manually generate vectors.
    """

    vector_store = PineconeVectorStore(
        index=pinecone_index,
        embedding=embeddings
    )

    print("✅ Vector Store Created")

    # ==========================================================
    # Step 6 : Store Documents
    # ==========================================================

    """
    Internally LangChain performs

    Chunk -> Gemini Embedding API -> Numerical Vector -> Pinecone

    along with metadata. Every chunk gets stored as one vector.

    We upload in batches instead of all at once so a failure
    partway through doesn't lose all the work, and so we don't
    send oversized requests to the embedding / Pinecone APIs.

    Each batch still triggers one embed_content call PER chunk under
    the hood, so we wrap every batch in retry-with-backoff to survive
    the Gemini free tier's requests-per-minute limit.
    """

    BATCH_SIZE = 20

    for i in range(0, len(chunked_docs), BATCH_SIZE):
        batch = chunked_docs[i:i + BATCH_SIZE]
        upload_batch_with_backoff(vector_store, batch)
        print(f"Uploaded {i + len(batch)} documents")

    print()
    print("===================================")
    print("🎉 Successfully Indexed Documents")
    print(f"Stored {len(chunked_docs)} Chunks")
    print("===================================")


# ==========================================================
# Entry Point
# ==========================================================

if __name__ == "__main__":
    index_document()