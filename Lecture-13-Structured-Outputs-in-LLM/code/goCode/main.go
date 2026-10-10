
// package main

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"time"
// )

// func main() {
// 	meetingService, err := NewMeetingService()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	ctx, cancel := context.WithTimeout(
// 		context.Background(),
// 		60*time.Second,
// 	)
// 	defer cancel()

// 	meetingDetails, err := meetingService.Schedule(
// 		ctx,
// 		"Schedule a project review with Aditya tomorrow at 3 PM for 45 minutes.",
// 	)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	output, err := json.MarshalIndent(
// 		meetingDetails,
// 		"",
// 		"  ",
// 	)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	fmt.Println(string(output))
// }



package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

func main() {
	meetingService := NewMeetingService()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		60*time.Second,
	)
	defer cancel()

	details, err := meetingService.Schedule(
		ctx,
		"Schedule a project review with Aditya tomorrow at 3 PM for 45 minutes.",
	)
	if err != nil {
		log.Fatal(err)
	}

	output, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(output))
}
