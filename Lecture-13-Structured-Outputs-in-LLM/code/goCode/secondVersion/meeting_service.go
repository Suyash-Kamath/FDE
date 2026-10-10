
// package main

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"os"
// 	"strings"
// 	"time"
// )

// type MeetingService struct {
// 	client *http.Client
// 	apiKey string
// }

// func NewMeetingService() (*MeetingService, error) {
// 	apiKey := os.Getenv("OPENAI_API_KEY")
// 	if apiKey == "" {
// 		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
// 	}

// 	return &MeetingService{
// 		client: &http.Client{Timeout: 60 * time.Second},
// 		apiKey: apiKey,
// 	}, nil
// }

// func (s *MeetingService) Schedule(
// 	ctx context.Context,
// 	message string,
// ) (MeetingDetails, error) {
// 	var details MeetingDetails

// 	systemPrompt := fmt.Sprintf(`
// You extract meeting information from the user's request.

// Today's date is %s.

// Rules:
// - Convert relative dates like today and tomorrow into yyyy-MM-dd format.
// - Convert time into 24-hour HH:mm format.
// - If title is missing, create a simple title.
// - If duration is missing, use 30 minutes.
// - Do not invent attendee, date or time.
// - If information is missing, keep it blank.
// - Return only a valid JSON object with these fields:
//   title, attendee, date, time, durationMinutes.
// `, time.Now().Format("2006-01-02"))

// 	payload := map[string]any{
// 		"model": "gpt-4o-mini",
// 		"messages": []map[string]string{
// 			{"role": "system", "content": systemPrompt},
// 			{"role": "user", "content": message},
// 		},
// 		"response_format": map[string]string{
// 			"type": "json_object",
// 		},
// 	}

// 	body, err := json.Marshal(payload)
// 	if err != nil {
// 		return details, fmt.Errorf("encode request: %w", err)
// 	}

// 	req, err := http.NewRequestWithContext(
// 		ctx,
// 		http.MethodPost,
// 		"https://api.openai.com/v1/chat/completions",
// 		bytes.NewReader(body),
// 	)
// 	if err != nil {
// 		return details, fmt.Errorf("create request: %w", err)
// 	}

// 	req.Header.Set("Authorization", "Bearer "+s.apiKey)
// 	req.Header.Set("Content-Type", "application/json")

// 	resp, err := s.client.Do(req)
// 	if err != nil {
// 		return details, fmt.Errorf("call OpenAI API: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
// 		var apiError struct {
// 			Error struct {
// 				Message string `json:"message"`
// 			} `json:"error"`
// 		}
// 		_ = json.NewDecoder(resp.Body).Decode(&apiError)

// 		return details, fmt.Errorf(
// 			"OpenAI API returned HTTP %d: %s",
// 			resp.StatusCode,
// 			apiError.Error.Message,
// 		)
// 	}

// 	var result struct {
// 		Choices []struct {
// 			Message struct {
// 				Content string `json:"content"`
// 			} `json:"message"`
// 		} `json:"choices"`
// 	}

// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		return details, fmt.Errorf("decode API response: %w", err)
// 	}

// 	if len(result.Choices) == 0 {
// 		return details, fmt.Errorf("OpenAI returned no choices")
// 	}

// 	content := strings.TrimSpace(result.Choices[0].Message.Content)

// 	if err := json.Unmarshal([]byte(content), &details); err != nil {
// 		return details, fmt.Errorf("decode meeting details: %w", err)
// 	}

// 	return details, nil
// }



package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

type MeetingService struct {
	client openai.Client
}

func NewMeetingService() *MeetingService {
	return &MeetingService{
		client: openai.NewClient(),
	}
}

func (s *MeetingService) Schedule(
	ctx context.Context,
	message string,
) (MeetingDetails, error) {
	var details MeetingDetails

	systemPrompt := fmt.Sprintf(`
You extract meeting information from the user's request.

Today's date is %s.

Rules:
- Convert relative dates like today and tomorrow into yyyy-MM-dd format.
- Convert time into 24-hour HH:mm format.
- If title is missing, create a simple title.
- If duration is missing, use 30 minutes.
- Do not invent attendee, date or time.
- If information is missing, use an empty string.
- Return the extracted meeting details using the required schema.
`, time.Now().Format("2006-01-02"))

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type": "string",
			},
			"attendee": map[string]any{
				"type": "string",
			},
			"date": map[string]any{
				"type": "string",
			},
			"time": map[string]any{
				"type": "string",
			},
			"durationMinutes": map[string]any{
				"type": "integer",
			},
		},
		"required": []string{
			"title",
			"attendee",
			"date",
			"time",
			"durationMinutes",
		},
		"additionalProperties": false,
	}

	response, err := s.client.Responses.New(
		ctx,
		responses.ResponseNewParams{
			Model: openai.ChatModelGPT4oMini,
			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(
					systemPrompt + "\n\nUser request: " + message,
				),
			},
			Text: responses.ResponseTextConfigParam{
				Format: responses.ResponseFormatTextConfigUnionParam{
					OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
						Name:   "meeting_details",
						Schema: schema,
						Strict: openai.Bool(true),
					},
				},
			},
		},
	)
	if err != nil {
		return details, fmt.Errorf("extract meeting details: %w", err)
	}

	if response.OutputText() == "" {
		return details, fmt.Errorf("OpenAI returned empty output")
	}

	if err := json.Unmarshal(
		[]byte(response.OutputText()),
		&details,
	); err != nil {
		return details, fmt.Errorf("parse meeting details: %w", err)
	}

	return details, nil
}
