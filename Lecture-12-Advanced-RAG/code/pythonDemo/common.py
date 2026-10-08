"""Shared config, clients and the embedding call used by both commands.

Keeping embeddings in ONE place guarantees the chunks (index) and the
questions (chat) are always embedded with the same model and dimensions.
"""

from __future__ import annotations

import os
import sys

from openai import OpenAI, RateLimitError
from pinecone import Pinecone

# ==========================================================
# Shared config
# ==========================================================

EMBEDDING_MODEL = "text-embedding-3-small"
EMBEDDING_DIM = 1536  # native size of text-embedding-3-small; Pinecone index must be 1536-dim
MAX_RETRIES = 5  # SDK retries 429/5xx with exponential backoff


def must_env(key: str) -> str:
    value = os.getenv(key)
    if not value:
        sys.exit(f"missing environment variable {key}")
    return value


def namespace() -> str:
    """PINECONE_NAMESPACE, or "" for the default namespace."""
    return os.getenv("PINECONE_NAMESPACE", "")


# ==========================================================
# Shared clients
# ==========================================================


def new_openai_client() -> OpenAI:
    return OpenAI(api_key=must_env("OPENAI_API_KEY"), max_retries=MAX_RETRIES)


def connect_index():
    """Look up the index by name, check dimension + metric, return a connection."""
    pc = Pinecone(api_key=must_env("PINECONE_API_KEY"))
    index_name = must_env("PINECONE_INDEX_NAME")
    desc = pc.describe_index(index_name)

    if desc.dimension and int(desc.dimension) != EMBEDDING_DIM:
        raise RuntimeError(
            f"index {index_name!r} has dimension {desc.dimension} but embeddings are "
            f"{EMBEDDING_DIM}; create a {EMBEDDING_DIM}-dim index"
        )

    # The similarity threshold assumes higher score = more similar.
    metric = str(getattr(desc, "metric", "") or "").lower()
    if metric and "cosine" not in metric and "dotproduct" not in metric:
        print(
            f"⚠️ index metric is {metric!r}; scores are distances, so the "
            "similarity threshold in query.py will behave backwards. Use a cosine index."
        )

    return pc.Index(host=desc.host)


# ==========================================================
# Shared embedding call (used by both index and chat)
# ==========================================================
#
# - One request embeds all texts.
# - OpenAI returns L2-normalised vectors, so no manual normalise step.


def embed_texts(client: OpenAI, texts: list[str]) -> list[list[float]]:
    try:
        resp = client.embeddings.create(
            model=EMBEDDING_MODEL,
            input=texts,
            dimensions=EMBEDDING_DIM,
        )
    except RateLimitError as exc:
        raise RuntimeError(
            f"still rate limited after {MAX_RETRIES} retries "
            "(check quota / lower the batch size)"
        ) from exc

    if len(resp.data) != len(texts):
        raise RuntimeError(f"expected {len(texts)} embeddings, got {len(resp.data)}")

    # Results carry an index; sort by it so vectors line up with inputs.
    return [d.embedding for d in sorted(resp.data, key=lambda d: d.index)]