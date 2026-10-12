import os
from contextlib import asynccontextmanager

import uvicorn
from agents import Agent, Runner
from agents.mcp import MCPServerStreamableHttp
from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException, Query
from fastapi.responses import PlainTextResponse

load_dotenv()

if not os.getenv("OPENAI_API_KEY"):
    raise RuntimeError("Set OPENAI_API_KEY in the .env file")

mcp_server = MCPServerStreamableHttp(
    name="Task MCP server",
    params={
        "url": os.getenv("MCP_SERVER_URL", "http://localhost:8081/mcp"),
    },
    cache_tools_list=True,
)

agent = Agent(
    name="Task Assistant",
    instructions="Use the MCP task tools to handle every task-management request.",
    mcp_servers=[mcp_server],
)


@asynccontextmanager
async def lifespan(_: FastAPI):
    async with mcp_server:
        yield


app = FastAPI(lifespan=lifespan)


@app.get("/ask", response_class=PlainTextResponse)
async def ask(message: str = Query(min_length=1)) -> str:
    try:
        result = await Runner.run(agent, message)
        return str(result.final_output or "")
    except Exception as error:
        raise HTTPException(status_code=500, detail="Unable to process the request") from error


if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8080)