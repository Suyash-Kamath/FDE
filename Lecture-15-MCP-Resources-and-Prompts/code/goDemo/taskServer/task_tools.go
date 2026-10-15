// Plain task logic (port of task_tools.py) - no MCP code in here.
// main.go wraps these functions as MCP tools.

package main

import (
	"slices"
	"sync"
)

// HTTP requests are served concurrently in Go, so a mutex guards the list
// (Python's single event loop doesn't need one).
var (
	mu    sync.Mutex
	tasks []string
)

func createTaskValue(title string) string {
	mu.Lock()
	defer mu.Unlock()

	tasks = append(tasks, title)
	return "Task created: " + title
}

func listTaskValues() []string {
	mu.Lock()
	defer mu.Unlock()

	return append([]string{}, tasks...) // a copy (never nil, so JSON is [] not null)
}

func completeTaskValue(title string) string {
	mu.Lock()
	defer mu.Unlock()

	if i := slices.Index(tasks, title); i != -1 {
		tasks = slices.Delete(tasks, i, i+1)
		return "Task completed: " + title
	}

	return "Task not found: " + title
}
