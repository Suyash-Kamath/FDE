from datetime import datetime

from pydantic import BaseModel


class MeetingDetails(BaseModel):
    title: str
    attendee: str
    date: str
    time: str
    durationMinutes: int

    def missing_fields(self) -> list[str]:
        """Required fields that are empty or malformed.

        The LLM is told the rules, but we never trust it blindly:
        this is the deterministic safety net used by version 2.
        """
        missing = []

        if not self.attendee.strip():
            missing.append("attendee")
        if not _parses(self.date, "%Y-%m-%d"):
            missing.append("date")
        if not _parses(self.time, "%H:%M"):
            missing.append("time")
        if self.durationMinutes <= 0:
            missing.append("durationMinutes")

        return missing


def _parses(value: str, fmt: str) -> bool:
    try:
        datetime.strptime(value, fmt)
        return True
    except ValueError:
        return False
