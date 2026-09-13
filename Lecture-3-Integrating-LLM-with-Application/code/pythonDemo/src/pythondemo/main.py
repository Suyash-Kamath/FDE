import os

from dotenv import load_dotenv
from fastapi import FastAPI, Request, Response
from openai import AsyncOpenAI


# 1. Load .env first
load_dotenv()

# 2. Check whether API key exists
if not os.getenv("OPENAI_API_KEY"):
    raise RuntimeError("OPENAI_API_KEY is missing")

print("OPENAI_API_KEY found")

# 3. Create OpenAI client
client = AsyncOpenAI()

app = FastAPI()


async def summarize(ticket: str) -> str:
    completion = await client.chat.completions.create(
        model="gpt-4o-mini",
        messages=[
            {
                "role": "user",
                "content": (
                    "Summarize this support ticket in 2 lines:\n\n"
                    + ticket
                ),
            }
        ],
    )

    if not completion.choices:
        raise RuntimeError("no choices returned from model")

    content = completion.choices[0].message.content

    if content is None:
        raise RuntimeError("model returned empty content")

    return content


@app.post("/api/summarize")
async def summarize_handler(request: Request):
    try:
        body = await request.body()
        ticket = body.decode("utf-8")
    except UnicodeDecodeError:
        return Response(
            content="invalid request body",
            status_code=400,
            media_type="text/plain",
        )

    try:
        summary = await summarize(ticket)

        return Response(
            content=summary,
            media_type="text/plain",
        )

    except Exception as error:
        print("OpenAI error:", error)

        return Response(
            content="failed to summarize ticket",
            status_code=500,
            media_type="text/plain",
        )