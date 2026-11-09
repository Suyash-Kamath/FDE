from math import sqrt


def cosine_similarity(a, b):
    if not a or len(a) != len(b):
        raise ValueError("Embedding dimensions must match")
    norm = sqrt(sum(value * value for value in a) * sum(value * value for value in b))
    return sum(x * y for x, y in zip(a, b)) / norm if norm else 0


class SimpleVectorStore:
    def __init__(self, embedding_model):
        self.embedding_model = embedding_model
        self.documents = []

    def add(self, documents):
        vectors = self.embedding_model.embed([document["text"] for document in documents])
        self.documents = [dict(document, embedding=vector)
                          for document, vector in zip(documents, vectors, strict=True)]

    def similaritySearch(self, query, top_k=1):
        embedding = self.embedding_model.embed([query])[0]
        return sorted(self.documents, key=lambda doc: cosine_similarity(embedding, doc["embedding"]),
                      reverse=True)[:top_k]
