package main

// VERSION 2 (from ~38:00 in the lecture)
//
// Instead of extracting in one shot, we keep a conversation history and
// first check whether the user has given everything. If something is missing
// (attendee, date, time, duration), the LLM asks for it. Only when everything
// is present do we run the strict-schema extraction into MeetingDetails.
//
// Version 1 (single-shot extraction) stays untouched in meeting_service.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn of the conversation we keep as history.
type Message struct {
	Role    Role
	Content string
}

// ScheduleResult is the outcome of one conversational turn.
// Details is nil while we are still collecting information.
type ScheduleResult struct {
	Reply   string
	Details *MeetingDetails
}

// completenessCheck is the structured output of the intake step.
type completenessCheck struct {
	Complete      bool     `json:"complete"`
	MissingFields []string `json:"missingFields"`
	Reply         string   `json:"reply"`
}

type ChatMeetingService struct {
	client  openai.Client
	history []Message
}

func NewChatMeetingService() *ChatMeetingService {
	return &ChatMeetingService{
		client: openai.NewClient(), // reads OPENAI_API_KEY from env
	}
}

// Reset clears the conversation so the next message starts a new meeting.
func (s *ChatMeetingService) Reset() {
	s.history = nil
}

// Schedule handles one user message.
//
// Step 1 (intake): checks the whole history for attendee, date, time and
// duration. If something is missing, returns a follow-up question.
// Step 2 (extraction): only when everything is present, maps the
// conversation into MeetingDetails using a strict JSON schema.
func (s *ChatMeetingService) Schedule(
	ctx context.Context,
	message string,
) (ScheduleResult, error) {
	prevLen := len(s.history)
	s.history = append(s.history, Message{Role: RoleUser, Content: message})

	// On an API error, drop the user's message so a retry doesn't duplicate it.
	rollback := func() { s.history = s.history[:prevLen] }

	check, err := s.checkCompleteness(ctx)
	if err != nil {
		rollback()
		return ScheduleResult{}, err
	}

	if !check.Complete {
		reply := check.Reply
		if strings.TrimSpace(reply) == "" {
			reply = askFor(check.MissingFields)
		}
		s.history = append(s.history, Message{Role: RoleAssistant, Content: reply})
		return ScheduleResult{Reply: reply}, nil
	}

	details, err := s.extractDetails(ctx)
	if err != nil {
		rollback()
		return ScheduleResult{}, err
	}

	// Safety net: the intake step said "complete", but verify in Go anyway.
	if missing := details.MissingFields(); len(missing) > 0 {
		reply := askFor(missing)
		s.history = append(s.history, Message{Role: RoleAssistant, Content: reply})
		return ScheduleResult{Reply: reply}, nil
	}

	// Meeting is fully specified; start fresh for the next one.
	s.Reset()

	return ScheduleResult{
		Reply:   "Got everything. Here are your meeting details:",
		Details: &details,
	}, nil
}

func (s *ChatMeetingService) checkCompleteness(
	ctx context.Context,
) (completenessCheck, error) {
	var check completenessCheck

	instructions := fmt.Sprintf(`
You are the intake step of a meeting scheduler. Read the WHOLE conversation
and decide whether the user has given everything needed to schedule ONE meeting.

Today's date is %s.

Required information:
- attendee: who the meeting is with
- date: any date expression counts (today, tomorrow, next Monday, 12 Oct)
- time: a specific time (4 PM, 16:00). Vague words like "evening" or
  "afternoon" do NOT count as a time.
- durationMinutes: how long the meeting lasts

The title is NOT required. Never ask for the title.

Rules:
- Combine information from all earlier user messages, not just the latest one.
- If the user corrects something, the latest value wins.
- Never assume or invent a value.
- If anything is missing: set complete to false, list the missing fields,
  and in reply ask for them in one short, friendly message. Briefly mention
  what you already have so the user can confirm it.
- If everything is present: set complete to true, missingFields to an empty
  list, and reply to an empty string.
`, time.Now().Format("2006-01-02 (Monday)"))

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"complete": map[string]any{
				"type": "boolean",
			},
			"missingFields": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
					"enum": []string{"attendee", "date", "time", "durationMinutes"},
				},
			},
			"reply": map[string]any{
				"type": "string",
			},
		},
		"required":             []string{"complete", "missingFields", "reply"},
		"additionalProperties": false,
	}

	if err := s.structuredCall(ctx, instructions, "completeness_check", schema, &check); err != nil {
		return check, fmt.Errorf("check completeness: %w", err)
	}

	return check, nil
}

func (s *ChatMeetingService) extractDetails(
	ctx context.Context,
) (MeetingDetails, error) {
	var details MeetingDetails

	instructions := fmt.Sprintf(`
You extract meeting information from the conversation with the user.

Today's date is %s.

Rules:
- Use information from ALL user messages. If the user corrected something,
  the latest value wins.
- Convert relative dates like today and tomorrow into yyyy-MM-dd format.
- Convert time into 24-hour HH:mm format.
- Convert duration into minutes (e.g. "1 hour" -> 60).
- If title is missing, create a simple title like "Meeting with <attendee>".
- Do not invent attendee, date, time or duration.
- If a text field is missing, use an empty string. If duration is missing, use 0.
`, time.Now().Format("2006-01-02 (Monday)"))

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":           map[string]any{"type": "string"},
			"attendee":        map[string]any{"type": "string"},
			"date":            map[string]any{"type": "string"},
			"time":            map[string]any{"type": "string"},
			"durationMinutes": map[string]any{"type": "integer"},
		},
		"required": []string{
			"title", "attendee", "date", "time", "durationMinutes",
		},
		"additionalProperties": false,
	}

	if err := s.structuredCall(ctx, instructions, "meeting_details", schema, &details); err != nil {
		return details, fmt.Errorf("extract meeting details: %w", err)
	}

	return details, nil
}

// structuredCall sends the full history plus a system prompt and decodes the
// strict-schema JSON response into out.
func (s *ChatMeetingService) structuredCall(
	ctx context.Context,
	instructions string,
	schemaName string,
	schema map[string]any,
	out any,
) error {
	input := make(responses.ResponseInputParam, 0, len(s.history))
	for _, m := range s.history {
		role := responses.EasyInputMessageRoleUser
		if m.Role == RoleAssistant {
			role = responses.EasyInputMessageRoleAssistant
		}
		input = append(input, responses.ResponseInputItemParamOfMessage(m.Content, role))
	}

	response, err := s.client.Responses.New(
		ctx,
		responses.ResponseNewParams{
			Model:        openai.ChatModelGPT4oMini,
			Instructions: openai.String(instructions),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: input,
			},
			Text: responses.ResponseTextConfigParam{
				Format: responses.ResponseFormatTextConfigUnionParam{
					OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
						Name:   schemaName,
						Schema: schema,
						Strict: openai.Bool(true),
					},
				},
			},
		},
	)
	if err != nil {
		return err
	}

	text := response.OutputText()
	if text == "" {
		return fmt.Errorf("OpenAI returned empty output")
	}

	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	return nil
}

// askFor builds a fallback follow-up question from field names.
func askFor(fields []string) string {
	labels := map[string]string{
		"attendee":        "who the meeting is with",
		"date":            "the date",
		"time":            "the time",
		"durationMinutes": "how long it should last",
	}

	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if l, ok := labels[f]; ok {
			parts = append(parts, l)
		}
	}
	if len(parts) == 0 {
		return "Could you share a few more details about the meeting?"
	}

	return "You missed " + strings.Join(parts, ", ") + ". Could you tell me?"
}
