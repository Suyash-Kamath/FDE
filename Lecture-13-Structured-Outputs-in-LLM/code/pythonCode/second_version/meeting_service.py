from datetime import date

from openai import OpenAI

from meeting_details import MeetingDetails


class MeetingService:
    def __init__(self):
        self.client = OpenAI()

    def schedule(self, message: str) -> MeetingDetails:
        system_prompt = f"""
                You extract meeting information from the user's request.

                Today's date is {date.today().isoformat()}.

                Rules:

                - Convert relative dates like today and tomorrow
                  into yyyy-MM-dd format.

                - Convert time into 24-hour HH:mm format.

                - If title is missing, create a simple title.

                - If duration is missing, use 30 minutes.

                - Do not invent attendee, date or time.

                - If information is missing, keep it blank.
        """

        response = self.client.responses.parse(
            model="gpt-4o-mini",
            input=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": message},
            ],
            text_format=MeetingDetails,
        )

        return response.output_parsed