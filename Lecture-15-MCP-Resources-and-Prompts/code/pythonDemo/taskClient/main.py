from fastapi import FastAPI
from fastapi.responses import PlainTextResponse

from chat_service import chat, plan_day

app = FastAPI(title="Task Client")


@app.get("/ask", response_class=PlainTextResponse)
async def ask(message: str) -> str:
    return await chat(message)


@app.get("/plan", response_class=PlainTextResponse)
async def plan(hours: str) -> str:
    return await plan_day(hours)