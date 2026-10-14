import json
import os
from typing import Any

from mcp import Client
from mcp.types import TextContent, TextResourceContents
from openai import AsyncOpenAI

openai = AsyncOpenAI()
model = os.getenv("OPENAI_MODEL", "gpt-5.4-mini")
mcp_url = os.getenv("MCP_SERVER_URL", "http://127.0.0.1:8081/mcp")


def to_openai_tools(mcp_tools: list[Any]) -> list[dict[str, Any]]:
    return [
        {
            "type": "function",
            "name": tool.name,
            "description": tool.description,
            "parameters": tool.input_schema,
        }
        for tool in mcp_tools
    ]


def tool_result_text(result: Any) -> str:
    values = []
    for block in result.content:
        if isinstance(block, TextContent):
            values.append(block.text)
        else:
            values.append(json.dumps(block.model_dump(mode="json")))
    return "\n".join(values)


async def ask_model(mcp_client: Client, message: str, instructions: str | None = None) -> str:
    listed_tools = await mcp_client.list_tools()
    tools = to_openai_tools(listed_tools.tools)
    inputs: list[Any] = [{"role": "user", "content": message}]

    for _ in range(10):
        response = await openai.responses.create(
            model=model,
            instructions=instructions,
            tools=tools,
            input=inputs,
        )

        inputs.extend(response.output)
        tool_calls = [item for item in response.output if item.type == "function_call"]

        if not tool_calls:
            return response.output_text

        for tool_call in tool_calls:
            result = await mcp_client.call_tool(
                tool_call.name,
                json.loads(tool_call.arguments),
            )
            inputs.append(
                {
                    "type": "function_call_output",
                    "call_id": tool_call.call_id,
                    "output": tool_result_text(result),
                }
            )

    raise RuntimeError("The model used too many tool-calling rounds.")


async def chat(message: str) -> str:
    async with Client(mcp_url) as mcp_client:
        result = await mcp_client.read_resource("task://guidelines")
        content = result.contents[0]

        if not isinstance(content, TextResourceContents):
            raise TypeError("The task guidelines resource must contain text.")

        instructions = f"""You are a task management assistant.

Use the following task management guidelines whenever they are relevant
to the user's question.

TASK GUIDELINES:

{content.text}"""

        return await ask_model(mcp_client, message, instructions)


async def plan_day(hours: str) -> str:
    async with Client(mcp_url) as mcp_client:
        result = await mcp_client.get_prompt(
            "plan_day",
            {"availableHours": hours},
        )
        content = result.messages[0].content

        if not isinstance(content, TextContent):
            raise TypeError("The plan_day prompt must contain text.")

        return await ask_model(mcp_client, content.text)