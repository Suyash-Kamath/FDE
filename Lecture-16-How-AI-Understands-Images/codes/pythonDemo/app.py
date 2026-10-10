import os
from pathlib import Path

from dotenv import load_dotenv
from flask import Flask, Response, request
from openai import OpenAI
from werkzeug.exceptions import RequestEntityTooLarge

from chat_service import ChatService

DIRECTORY = Path(__file__).resolve().parent
load_dotenv(DIRECTORY / ".env")


def create_app(chat_service=None):
    app = Flask(__name__, static_folder="static", static_url_path="/static")
    app.config["MAX_CONTENT_LENGTH"] = 10 * 1024 * 1024
    app.config["MAX_FORM_MEMORY_SIZE"] = 10 * 1024 * 1024

    if chat_service is None:
        api_key = os.getenv("OPENAI_API_KEY")
        if not api_key:
            raise RuntimeError("Set OPENAI_API_KEY in .env before starting the demo.")
        chat_service = ChatService(
            OpenAI(api_key=api_key, timeout=60.0, max_retries=0),
            os.getenv("OPENAI_MODEL", "gpt-4o-mini"),
        )

    @app.get("/")
    @app.get("/index.html")
    def index():
        return app.send_static_file("index.html")

    @app.post("/api/chat")
    def chat():
        message = request.form.get("message")
        if message is None:
            return Response("The message field is required.", status=400, mimetype="text/plain")
        image = request.files.get("image")
        if image is not None and not image.mimetype.startswith("image/"):
            return Response("Please upload an image file.", status=400, mimetype="text/plain")
        try:
            reply = chat_service.chat(message, image)
            return Response(reply, mimetype="text/plain")
        except Exception:
            # Avoid exposing credentials or provider internals to the frontend.
            return Response(
                "Unable to get an AI response. Check your API key and try again.",
                status=502,
                mimetype="text/plain",
            )

    @app.delete("/api/chat")
    def clear_chat():
        chat_service.clear_history()
        return Response(status=200)

    @app.errorhandler(RequestEntityTooLarge)
    def upload_too_large(error):
        return Response("Maximum request size is 10 MB.", status=413, mimetype="text/plain")

    return app


if __name__ == "__main__":
    create_app().run(host="127.0.0.1", port=int(os.getenv("PORT", "8080")))