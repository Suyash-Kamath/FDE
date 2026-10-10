import base64
from threading import Lock

SYSTEM_PROMPT = "You are a helpful AI assistant. Answer questions clearly and simply."


class ChatService:
    """Like the Spring singleton service, history is shared and kept in memory."""

    def __init__(self, client, model="gpt-4o-mini"):
        self.client = client
        self.model = model
        self.history = []
        self.lock = Lock()

    def chat(self, message, image=None):
        # Keep simultaneous requests and clearing in a consistent turn order.
        with self.lock:
            content = message
            if image is not None:
                image_bytes = image.read()
                if image_bytes:
                    encoded_image = base64.b64encode(image_bytes).decode("ascii")
                    content = [
                        {"type": "text", "text": message},
                        {
                            "type": "image_url",
                            "image_url": {
                                "url": f"data:{image.mimetype};base64,{encoded_image}"
                            },
                        },
                    ]
            user_message = {"role": "user", "content": content}
            response = self.client.chat.completions.create(
                model=self.model,
                temperature=0.4,
                messages=[
                    {"role": "system", "content": SYSTEM_PROMPT},
                    *self.history,
                    user_message,
                ],
            )
            reply = response.choices[0].message.content
            if not isinstance(reply, str):
                raise ValueError("The model returned no text response.")
            # Only commit a complete turn after the API call succeeds.
            self.history.extend([user_message, {"role": "assistant", "content": reply}])
            return reply

    def clear_history(self):
        with self.lock:
            self.history.clear()