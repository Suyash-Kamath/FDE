import json
import math
from pathlib import Path

from .embedding import EmbeddingClient
from .model.movie import Movie
from .model.movie_data import MovieData
from .model.movie_match import MovieMatch

RESOURCES_DIR = Path(__file__).resolve().parent / "resources"
TOP_K_MATCHES = 3


class MovieService:
    def __init__(self, embedding_client: EmbeddingClient):
        self.embedding_client = embedding_client
        self.movies_embedding: list[Movie] = []

    def initialize_movies(self) -> None:
        resource = RESOURCES_DIR / "movies.json"

        with resource.open("r", encoding="utf-8") as input_stream:
            movie_data_list = [
                MovieData(**movie_data) for movie_data in json.load(input_stream)
            ]

        for movie_data in movie_data_list:
            embedding = self.embedding_client.embed(movie_data.description)

            self.movies_embedding.append(
                Movie(
                    title=movie_data.title,
                    description=movie_data.description,
                    embedding=embedding,
                )
            )

        print(f"{len(self.movies_embedding)} movies loaded with embeddings.")

    def search(self, query: str) -> list[MovieMatch]:
        user_query_embedding = self.embedding_client.embed(query)

        matches = [
            MovieMatch(
                title=movie.title,
                description=movie.description,
                match=self._cosine_similarity(user_query_embedding, movie.embedding),
            )
            for movie in self.movies_embedding
        ]

        return self._top_k_matches(self._sort_by_similarity(matches))

    def similar_movies(self, title: str) -> list[MovieMatch]:
        selected_movie = self._find_movie(title)

        matches = [
            MovieMatch(
                title=movie.title,
                description=movie.description,
                match=self._cosine_similarity(
                    selected_movie.embedding, movie.embedding
                ),
            )
            for movie in self.movies_embedding
            if movie.title.lower() != title.lower()
        ]

        return self._top_k_matches(self._sort_by_similarity(matches))

    def _find_movie(self, title: str) -> Movie:
        for movie in self.movies_embedding:
            if movie.title.lower() == title.lower():
                return movie

        raise ValueError(f"Movie not found: {title}")

    @staticmethod
    def _cosine_similarity(a: list[float], b: list[float]) -> float:
        dot_product = sum(x * y for x, y in zip(a, b))
        norm_a = math.sqrt(sum(x * x for x in a))
        norm_b = math.sqrt(sum(y * y for y in b))

        if norm_a == 0 or norm_b == 0:
            return 0.0

        return dot_product / (norm_a * norm_b)

    @staticmethod
    def _sort_by_similarity(matches: list[MovieMatch]) -> list[MovieMatch]:
        return sorted(matches, key=lambda match: match.match, reverse=True)

    @staticmethod
    def _top_k_matches(matches: list[MovieMatch]) -> list[MovieMatch]:
        return matches[:TOP_K_MATCHES]
