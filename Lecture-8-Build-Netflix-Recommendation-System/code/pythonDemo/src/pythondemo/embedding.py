from typing import Protocol

from openai import OpenAI


class EmbeddingClient(Protocol):
    def embed(self, text: str) -> list[float]: ...


class OpenAIEmbeddingClient:
    def __init__(self, client: OpenAI, model: str):
        self._client = client
        self._model = model

    def embed(self, text: str) -> list[float]:
        response = self._client.embeddings.create(model=self._model, input=text)
        return response.data[0].embedding
