from dotenv import load_dotenv
from fastapi import FastAPI
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

from .chat_controller import ChatController
from .chat_service import ChatService

load_dotenv()

app = FastAPI()

chat_service = ChatService()
chat_controller = ChatController(chat_service)


class ChatRequest(BaseModel):
    message: str


@app.post("/chat")
async def chat(request: ChatRequest):
    return StreamingResponse(
        chat_controller.chat(request.message),
        media_type="text/event-stream",
    )
