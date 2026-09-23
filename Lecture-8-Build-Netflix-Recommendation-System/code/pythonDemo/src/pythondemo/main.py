from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from openai import OpenAI

from .config import settings
from .embedding import OpenAIEmbeddingClient
from .movie_controller import create_movie_router
from .movie_service import MovieService

STATIC_DIR = Path(__file__).resolve().parent / "static"

client = OpenAI(api_key=settings.openai_api_key)
embedding_client = OpenAIEmbeddingClient(client, settings.embedding_model)
movie_service = MovieService(embedding_client)


@asynccontextmanager
async def lifespan(app: FastAPI):
    movie_service.initialize_movies()
    yield


app = FastAPI(title="MovieMatch", lifespan=lifespan)
app.include_router(create_movie_router(movie_service))
app.mount("/", StaticFiles(directory=STATIC_DIR, html=True), name="static")
