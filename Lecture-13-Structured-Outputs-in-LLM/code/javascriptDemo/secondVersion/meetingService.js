import OpenAI from "openai";
import { zodTextFormat } from "openai/helpers/zod";

import { MeetingDetails } from "./meetingDetails.js";

// Today's date in local time as yyyy-MM-dd (like Python's date.today().isoformat()).
function todayIso() {
  const today = new Date();
  const yyyy = today.getFullYear();
  const mm = String(today.getMonth() + 1).padStart(2, "0");
  const dd = String(today.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

export class MeetingService {
  constructor() {
    this.client = new OpenAI(); // reads OPENAI_API_KEY from env
  }

  async schedule(message) {
    const systemPrompt = `
                You extract meeting information from the user's request.

                Today's date is ${todayIso()}.

                Rules:

                - Convert relative dates like today and tomorrow
                  into yyyy-MM-dd format.

                - Convert time into 24-hour HH:mm format.

                - If title is missing, create a simple title.

                - If duration is missing, use 30 minutes.

                - Do not invent attendee, date or time.

                - If information is missing, keep it blank.
        `;

    const response = await this.client.responses.parse({
      model: "gpt-4o-mini",
      input: [
        { role: "system", content: systemPrompt },
        { role: "user", content: message },
      ],
      text: { format: zodTextFormat(MeetingDetails, "meeting_details") },
    });

    return response.output_parsed;
  }
}
