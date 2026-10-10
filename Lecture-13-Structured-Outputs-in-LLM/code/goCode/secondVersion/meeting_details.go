package main

import (
	"strings"
	"time"
)

type MeetingDetails struct {
	Title           string `json:"title"`
	Attendee        string `json:"attendee"`
	Date            string `json:"date"`
	Time            string `json:"time"`
	DurationMinutes int    `json:"durationMinutes"`
}

// MissingFields returns the required fields that are empty or malformed.
// The LLM is told the rules, but we never trust it blindly: this is the
// deterministic safety net before anything gets "scheduled".
func (d MeetingDetails) MissingFields() []string {
	var missing []string

	if strings.TrimSpace(d.Attendee) == "" {
		missing = append(missing, "attendee")
	}
	if _, err := time.Parse("2006-01-02", d.Date); err != nil {
		missing = append(missing, "date")
	}
	if _, err := time.Parse("15:04", d.Time); err != nil {
		missing = append(missing, "time")
	}
	if d.DurationMinutes <= 0 {
		missing = append(missing, "durationMinutes")
	}

	return missing
}
