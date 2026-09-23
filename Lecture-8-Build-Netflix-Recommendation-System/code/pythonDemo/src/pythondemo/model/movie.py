from pydantic import BaseModel


class Movie(BaseModel):
    title: str
    description: str
    embedding: list[float]
