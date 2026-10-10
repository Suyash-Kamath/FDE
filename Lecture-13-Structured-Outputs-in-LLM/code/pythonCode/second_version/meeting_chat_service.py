"""VERSION 2 (from ~38:00 in the lecture).

Instead of extracting in one shot, we keep a conversation history and first
check whether the user has given everything. If something is missing
(attendee, date, time, duration), the LLM asks for it. Only when everything
is present do we run the structured extraction into MeetingDetails.

Version 1 (single-shot extraction) stays untouched in meeting_service.py.
"""

from dataclasses import dataclass
from datetime import date
from typing import Literal, Optional

from openai import OpenAI
from pydantic import BaseModel

from meeting_details import MeetingDetails


RequiredField = Literal["attendee", "date", "time", "durationMinutes"]


class CompletenessCheck(BaseModel):
    """Structured output of the intake step."""

    complete: bool
    missingFields: list[RequiredField]
    reply: str


@dataclass
class ScheduleResult:
    """Outcome of one conversational turn. details is None while collecting."""

    reply: str
    details: Optional[MeetingDetails] = None


FIELD_LABELS = {
    "attendee": "who the meeting is with",
    "date": "the date",
    "time": "the time",
    "durationMinutes": "how long it should last",
}


def _today() -> str:
    today = date.today()
    return f"{today.isoformat()} ({today.strftime('%A')})"


class ChatMeetingService:
    def __init__(self):
        self.client = OpenAI()  # reads OPENAI_API_KEY from env
        self.history: list[dict[str, str]] = []

    def reset(self) -> None:
        """Clear the conversation so the next message starts a new meeting."""
        self.history = []

    def schedule(self, message: str) -> ScheduleResult:
        """Handle one user message.

        Step 1 (intake): check the whole history for attendee, date, time and
        duration. If something is missing, return a follow-up question.
        Step 2 (extraction): only when everything is present, map the
        conversation into MeetingDetails.
        """
        prev_len = len(self.history)
        self.history.append({"role": "user", "content": message})

        try:
            check = self._check_completeness()

            if not check.complete:
                reply = check.reply.strip() or _ask_for(check.missingFields)
                self.history.append({"role": "assistant", "content": reply})
                return ScheduleResult(reply=reply)

            details = self._extract_details()
        except Exception:
            # On an API error, drop the user's message so a retry doesn't duplicate it.
            del self.history[prev_len:]
            raise

        # Safety net: the intake step said "complete", but verify in Python anyway.
        missing = details.missing_fields()
        if missing:
            reply = _ask_for(missing)
            self.history.append({"role": "assistant", "content": reply})
            return ScheduleResult(reply=reply)

        # Meeting is fully specified; start fresh for the next one.
        self.reset()
        return ScheduleResult(
            reply="Got everything. Here are your meeting details:",
            details=details,
        )

    def _check_completeness(self) -> CompletenessCheck:
        system_prompt = f"""
            You are the intake step of a meeting scheduler. Read the WHOLE
            conversation and decide whether the user has given everything
            needed to schedule ONE meeting.

            Today's date is {_today()}.

            Required information:
            - attendee: who the meeting is with
            - date: any date expression counts (today, tomorrow, next Monday, 12 Oct)
            - time: a specific time (4 PM, 16:00). Vague words like "evening"
              or "afternoon" do NOT count as a time.
            - durationMinutes: how long the meeting lasts

            The title is NOT required. Never ask for the title.

            Rules:
            - Combine information from all earlier user messages, not just the latest one.
            - If the user corrects something, the latest value wins.
            - Never assume or invent a value.
            - If anything is missing: set complete to false, list the missing
              fields, and in reply ask for them in one short, friendly message.
              Briefly mention what you already have so the user can confirm it.
            - If everything is present: set complete to true, missingFields to
              an empty list, and reply to an empty string.
        """
        return self._structured_call(system_prompt, CompletenessCheck)

    def _extract_details(self) -> MeetingDetails:
        system_prompt = f"""
            You extract meeting information from the conversation with the user.

            Today's date is {_today()}.

            Rules:
            - Use information from ALL user messages. If the user corrected
              something, the latest value wins.
            - Convert relative dates like today and tomorrow into yyyy-MM-dd format.
            - Convert time into 24-hour HH:mm format.
            - Convert duration into minutes (e.g. "1 hour" -> 60).
            - If title is missing, create a simple title like "Meeting with <attendee>".
            - Do not invent attendee, date, time or duration.
            - If a text field is missing, use an empty string. If duration is missing, use 0.
        """
        return self._structured_call(system_prompt, MeetingDetails)

    def _structured_call(self, system_prompt: str, model: type[BaseModel]):
        """Send system prompt + full history, parse the response into `model`."""
        response = self.client.responses.parse(
            model="gpt-4o-mini",
            input=[{"role": "system", "content": system_prompt}, *self.history],
            text_format=model,
        )

        if response.output_parsed is None:
            raise RuntimeError("OpenAI returned no parsable output")

        return response.output_parsed


def _ask_for(fields: list[str]) -> str:
    """Fallback follow-up question built from field names."""
    parts = [FIELD_LABELS[f] for f in fields if f in FIELD_LABELS]
    if not parts:
        return "Could you share a few more details about the meeting?"
    return "You missed " + ", ".join(parts) + ". Could you tell me?"
