# Lecture 13: Structured Outputs in LLM (JavaScript)

JavaScript port of `../pythonCode`. It uses the official `openai` SDK (Responses API, `responses.parse`).
The Pydantic models become [zod](https://zod.dev) schemas, and they are passed to OpenAI with `zodTextFormat` from `openai/helpers/zod`.

## Files

| JavaScript | Python | What it does |
| --- | --- | --- |
| `meetingDetails.js` | `meeting_details.py` | `MeetingDetails` zod schema (title, attendee, date, time, durationMinutes) |
| `meetingService.js` | `meeting_service.py` | `MeetingService.schedule()`: one message in, `MeetingDetails` out (gpt-4o-mini) |
| `demoApplication.js` | `demo_application.py` | Runs a single extraction and prints the result |
| `secondVersion/meetingDetails.js` | `second_version/meeting_details.py` | Same schema, plus `missingFields(details)` (the deterministic safety net) |
| `secondVersion/meetingService.js` | `second_version/meeting_service.py` | Same as version 1 |
| `secondVersion/meetingChatService.js` | `second_version/meeting_chat_service.py` | `ChatMeetingService`: keeps the chat history, runs a `CompletenessCheck` intake step that asks for missing fields, then extracts `MeetingDetails` |
| `secondVersion/demoApplication.js` | `second_version/demo_application.py` | `--version 1` runs one-shot extraction, `--version 2` runs the interactive chat loop (`reset` / `exit`) |

## Setup

Requires Node 20+.

```bash
npm install
cp .env.example .env   # then put your OPENAI_API_KEY in .env
```

## Run

```bash
npm start            # node demoApplication.js
npm run second:v1    # node secondVersion/demoApplication.js --version 1
npm run second:v2    # node secondVersion/demoApplication.js --version 2
```
