from pydantic import BaseModel


class MeetingDetails(BaseModel):
    title: str
    attendee: str
    date: str
    time: str
    durationMinutes: int