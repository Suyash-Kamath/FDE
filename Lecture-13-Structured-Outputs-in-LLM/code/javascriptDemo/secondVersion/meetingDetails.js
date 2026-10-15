import { z } from "zod";

export const MeetingDetails = z.object({
  title: z.string(),
  attendee: z.string(),
  date: z.string(),
  time: z.string(),
  durationMinutes: z.number().int(),
});

/**
 * Required fields that are empty or malformed.
 *
 * The LLM is told the rules, but we never trust it blindly:
 * this is the deterministic safety net used by version 2.
 *
 * (Pydantic lets us put this method on the model; with zod the schema only
 * describes data, so it is a plain function that takes the parsed object.)
 */
export function missingFields(details) {
  const missing = [];

  if (!details.attendee.trim()) {
    missing.push("attendee");
  }
  if (!parsesDate(details.date)) {
    missing.push("date");
  }
  if (!parsesTime(details.time)) {
    missing.push("time");
  }
  if (details.durationMinutes <= 0) {
    missing.push("durationMinutes");
  }

  return missing;
}

// Same idea as datetime.strptime(value, "%Y-%m-%d"): right shape AND a real calendar date.
function parsesDate(value) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return false;

  const [year, month, day] = match.slice(1).map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
  );
}

// Same idea as datetime.strptime(value, "%H:%M"): 00:00 - 23:59.
function parsesTime(value) {
  return /^([01]\d|2[0-3]):[0-5]\d$/.test(value);
}
