package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// go run .            -> version 1: one-shot extraction (defaults for missing fields)
// go run . -version=2 -> version 2: conversation that asks for missing details
func main() {
	version := flag.Int("version", 1, "1 = one-shot extraction, 2 = conversational with history")
	flag.Parse()

	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("OPENAI_API_KEY is not set")
	}

	switch *version {
	case 1:
		runV1()
	case 2:
		runV2()
	default:
		log.Fatalf("unknown version %d (use 1 or 2)", *version)
	}
}

// runV1 is your original flow: one message in, MeetingDetails out.
func runV1() {
	meetingService := NewMeetingService()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	details, err := meetingService.Schedule(
		ctx,
		"Schedule a project review with Aditya tomorrow at 3 PM for 45 minutes.",
	)
	if err != nil {
		log.Fatal(err)
	}

	printJSON(details)
}

// runV2 is the back-and-forth flow from ~38:00 in the lecture.
func runV2() {
	chatService := NewChatMeetingService()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Meeting scheduler (v2). Describe your meeting.")
	fmt.Println("Type 'reset' to start over, 'exit' to quit.")

	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		switch strings.ToLower(input) {
		case "":
			continue
		case "exit", "quit":
			return
		case "reset":
			chatService.Reset()
			fmt.Println("Bot: Okay, starting over.")
			continue
		}

		// Timeout per turn, not for the whole session.
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		result, err := chatService.Schedule(ctx, input)
		cancel()

		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		fmt.Println("Bot:", result.Reply)
		if result.Details != nil {
			printJSON(*result.Details)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func printJSON(details MeetingDetails) {
	output, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(output))
}
