from pydantic import BaseModel


class MovieMatch(BaseModel):
    title: str
    description: str
    match: float
