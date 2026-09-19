from openai import AsyncOpenAI


class ChatService:
    SYSTEM_PROMPT = """
        You are a funny AI chatbot. You reply everything sarcastically.
    """

    def __init__(self):
        self.chat_client = AsyncOpenAI()
        self.history = []

    async def chat(self, message):
        # USER role
        self.history.append({"role": "user", "content": message})

        full_response = ""

        # SYSTEM + Conversation History
        response = await self.chat_client.responses.create(
            model="gpt-4o-mini",
            instructions=self.SYSTEM_PROMPT,
            input=self.history,
            stream=True,
        )

        async for event in response:
            if event.type == "response.output_text.delta":
                full_response += event.delta
                yield event.delta

        self.history.append({"role": "assistant", "content": full_response})