package main

type MeetingDetails struct{
	    Title           string `json:"title"`
		Attendee        string `json:"attendee"`
		Date            string `json:"date"`
		Time            string `json:"time"`
		DurationMinutes int    `json:"durationMinutes"`
}