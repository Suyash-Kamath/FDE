from mcp.server import MCPServer

from task_prompts import plan_day_prompt
from task_resources import get_guidelines
from task_tools import complete_task_value, create_task_value, list_task_values

mcp = MCPServer("task-mcp-server", version="1.0.0")


@mcp.tool()
def create_task(title: str) -> str:
    """Create a new task. The title is required."""
    return create_task_value(title)


@mcp.tool()
def list_tasks() -> list[str]:
    """List all pending tasks."""
    return list_task_values()


@mcp.tool()
def complete_task(title: str) -> str:
    """Complete a task using its exact title."""
    return complete_task_value(title)


@mcp.resource(
    "task://guidelines",
    name="task-guidelines",
    description="Guidelines for managing and prioritizing tasks",
    mime_type="text/plain",
)
def task_guidelines() -> str:
    return get_guidelines()


@mcp.prompt(name="plan_day", description="Plan pending tasks for the available time")
def plan_day(availableHours: str) -> str:
    """Plan pending tasks for the available time."""
    return plan_day_prompt(availableHours)


if __name__ == "__main__":
    mcp.run(transport="streamable-http", host="127.0.0.1", port=8081)