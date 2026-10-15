// Text behind the plan_day prompt (port of task_promnpts.py).

export function planDayPrompt(availableHours) {
  return `Help me plan my pending tasks for today.

First check my current pending tasks
using the available task tools.

I have ${availableHours} hours available today.

Give me a concise and ordered plan.`;
}
