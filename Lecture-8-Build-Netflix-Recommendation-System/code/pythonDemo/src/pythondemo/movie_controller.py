from fastapi import APIRouter, HTTPException

from .model.movie_match import MovieMatch
from .movie_service import MovieService


def create_movie_router(movie_service: MovieService) -> APIRouter:
    router = APIRouter(prefix="/movies", tags=["movies"])

    @router.get("/search", response_model=list[MovieMatch])
    def search(query: str) -> list[MovieMatch]:
        return movie_service.search(query)

    @router.get("/{title}/similar", response_model=list[MovieMatch])
    def similar_movies(title: str) -> list[MovieMatch]:
        try:
            return movie_service.similar_movies(title)
        except ValueError as error:
            raise HTTPException(status_code=404, detail=str(error)) from error

    return router
