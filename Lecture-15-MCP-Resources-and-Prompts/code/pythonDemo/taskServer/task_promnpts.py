def plan_day_prompt(available_hours: str) -> str:
    return f"""Help me plan my pending tasks for today.

First check my current pending tasks
using the available task tools.

I have {available_hours} hours available today.

Give me a concise and ordered plan."""