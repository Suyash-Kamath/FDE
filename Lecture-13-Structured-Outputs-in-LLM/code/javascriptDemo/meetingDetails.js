import { z } from "zod";

// Pydantic BaseModel -> zod schema.
// The same schema is sent to OpenAI (as a JSON schema) and used to parse the reply.
export const MeetingDetails = z.object({
  title: z.string(),
  attendee: z.string(),
  date: z.string(),
  time: z.string(),
  durationMinutes: z.number().int(),
});
