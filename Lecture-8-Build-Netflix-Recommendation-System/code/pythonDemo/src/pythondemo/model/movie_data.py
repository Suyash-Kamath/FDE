from pydantic import BaseModel


class MovieData(BaseModel):
    title: str
    description: str
