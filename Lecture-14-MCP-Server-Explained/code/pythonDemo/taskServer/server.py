from typing import Annotated

from mcp.server.mcpserver import MCPServer
from pydantic import Field

mcp = MCPServer("task-mcp-server")
tasks: list[str] = []


@mcp.tool(name="create_task", description="Create a new task")
def create_task(
    title: Annotated[str, Field(description="Title of the task")],
) -> str:
    tasks.append(title)
    return f"Task created: {title}"


@mcp.tool(name="list_tasks", description="List all pending tasks")
def list_tasks() -> list[str]:
    return list(tasks)


@mcp.tool(
    name="complete_task",
    description="Complete a task using its exact title",
)
def complete_task(
    title: Annotated[str, Field(description="Exact title of the task")],
) -> str:
    try:
        tasks.remove(title)
        return f"Task completed: {title}"
    except ValueError:
        return f"Task not found: {title}"


if __name__ == "__main__":
    mcp.run(
        transport="streamable-http",
        host="127.0.0.1",
        port=8081,
        stateless_http=True,
        json_response=True,
    )