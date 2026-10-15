// Text behind the plan_day prompt (port of task_promnpts.py).

package main

import "fmt"

func planDayPrompt(availableHours string) string {
	return fmt.Sprintf(`Help me plan my pending tasks for today.

First check my current pending tasks
using the available task tools.

I have %s hours available today.

Give me a concise and ordered plan.`, availableHours)
}
