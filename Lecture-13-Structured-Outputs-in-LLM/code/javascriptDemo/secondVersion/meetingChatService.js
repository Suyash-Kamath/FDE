/**
 * VERSION 2 (from ~38:00 in the lecture).
 *
 * Instead of extracting in one shot, we keep a conversation history and first
 * check whether the user has given everything. If something is missing
 * (attendee, date, time, duration), the LLM asks for it. Only when everything
 * is present do we run the structured extraction into MeetingDetails.
 *
 * Version 1 (single-shot extraction) stays untouched in meetingService.js.
 */

import OpenAI from "openai";
import { zodTextFormat } from "openai/helpers/zod";
import { z } from "zod";

import { MeetingDetails, missingFields } from "./meetingDetails.js";

// Python: Literal["attendee", "date", "time", "durationMinutes"]
const RequiredField = z.enum(["attendee", "date", "time", "durationMinutes"]);

// Structured output of the intake step.
export const CompletenessCheck = z.object({
  complete: z.boolean(),
  missingFields: z.array(RequiredField),
  reply: z.string(),
});

/**
 * Outcome of one conversational turn. details is null while collecting.
 * (Python uses a @dataclass ScheduleResult; a plain object does the job here.)
 */
function scheduleResult(reply, details = null) {
  return { reply, details };
}

const FIELD_LABELS = {
  attendee: "who the meeting is with",
  date: "the date",
  time: "the time",
  durationMinutes: "how long it should last",
};

// e.g. "2026-10-15 (Thursday)", using the local date like Python's date.today().
function today() {
  const now = new Date();
  const yyyy = now.getFullYear();
  const mm = String(now.getMonth() + 1).padStart(2, "0");
  const dd = String(now.getDate()).padStart(2, "0");
  const weekday = now.toLocaleDateString("en-US", { weekday: "long" });
  return `${yyyy}-${mm}-${dd} (${weekday})`;
}

export class ChatMeetingService {
  constructor() {
    this.client = new OpenAI(); // reads OPENAI_API_KEY from env
    this.history = [];
  }

  // Clear the conversation so the next message starts a new meeting.
  reset() {
    this.history = [];
  }

  /**
   * Handle one user message.
   *
   * Step 1 (intake): check the whole history for attendee, date, time and
   * duration. If something is missing, return a follow-up question.
   * Step 2 (extraction): only when everything is present, map the
   * conversation into MeetingDetails.
   */
  async schedule(message) {
    const prevLen = this.history.length;
    this.history.push({ role: "user", content: message });

    let details;
    try {
      const check = await this.#checkCompleteness();

      if (!check.complete) {
        const reply = check.reply.trim() || askFor(check.missingFields);
        this.history.push({ role: "assistant", content: reply });
        return scheduleResult(reply);
      }

      details = await this.#extractDetails();
    } catch (error) {
      // On an API error, drop the user's message so a retry doesn't duplicate it.
      this.history.splice(prevLen);
      throw error;
    }

    // Safety net: the intake step said "complete", but verify in JavaScript anyway.
    const missing = missingFields(details);
    if (missing.length > 0) {
      const reply = askFor(missing);
      this.history.push({ role: "assistant", content: reply });
      return scheduleResult(reply);
    }

    // Meeting is fully specified; start fresh for the next one.
    this.reset();
    return scheduleResult("Got everything. Here are your meeting details:", details);
  }

  async #checkCompleteness() {
    const systemPrompt = `
            You are the intake step of a meeting scheduler. Read the WHOLE
            conversation and decide whether the user has given everything
            needed to schedule ONE meeting.

            Today's date is ${today()}.

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
        `;
    return this.#structuredCall(systemPrompt, CompletenessCheck, "completeness_check");
  }

  async #extractDetails() {
    const systemPrompt = `
            You extract meeting information from the conversation with the user.

            Today's date is ${today()}.

            Rules:
            - Use information from ALL user messages. If the user corrected
              something, the latest value wins.
            - Convert relative dates like today and tomorrow into yyyy-MM-dd format.
            - Convert time into 24-hour HH:mm format.
            - Convert duration into minutes (e.g. "1 hour" -> 60).
            - If title is missing, create a simple title like "Meeting with <attendee>".
            - Do not invent attendee, date, time or duration.
            - If a text field is missing, use an empty string. If duration is missing, use 0.
        `;
    return this.#structuredCall(systemPrompt, MeetingDetails, "meeting_details");
  }

  // Send system prompt + full history, parse the response into `schema`.
  async #structuredCall(systemPrompt, schema, name) {
    const response = await this.client.responses.parse({
      model: "gpt-4o-mini",
      input: [{ role: "system", content: systemPrompt }, ...this.history],
      text: { format: zodTextFormat(schema, name) },
    });

    if (response.output_parsed == null) {
      throw new Error("OpenAI returned no parsable output");
    }

    return response.output_parsed;
  }
}

// Fallback follow-up question built from field names.
function askFor(fields) {
  const parts = fields.filter((f) => f in FIELD_LABELS).map((f) => FIELD_LABELS[f]);
  if (parts.length === 0) {
    return "Could you share a few more details about the meeting?";
  }
  return "You missed " + parts.join(", ") + ". Could you tell me?";
}
