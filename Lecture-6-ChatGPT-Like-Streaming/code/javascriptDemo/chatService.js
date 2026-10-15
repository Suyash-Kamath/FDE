// JavaScript port of pythonDemo/src/pythondemo/chat_service.py
// (and the ChatService in goDemo/main_using_sdk.go)
// Talks to OpenAI using the official `openai` SDK and keeps the conversation history.

import OpenAI from "openai";

export class ChatService {
  static SYSTEM_PROMPT = `
        You are a funny AI chatbot. You reply everything sarcastically.
    `;

  constructor() {
    this.chatClient = new OpenAI();
    this.history = [];
  }

  // An async generator: it yields every chunk (delta) of the reply as it arrives,
  // just like the `async def chat(...)` + `yield` in Python.
  // `signal` lets the HTTP layer cancel the OpenAI request if the browser disconnects.
  async *chat(message, signal) {
    // USER role
    this.history.push({ role: "user", content: message });

    let fullResponse = "";

    // SYSTEM + Conversation History
    const response = await this.chatClient.responses.create(
      {
        model: "gpt-4o-mini",
        instructions: ChatService.SYSTEM_PROMPT,
        input: this.history,
        stream: true,
      },
      { signal },
    );

    for await (const event of response) {
      if (event.type === "response.output_text.delta") {
        fullResponse += event.delta;
        yield event.delta;
      }
    }

    // ASSISTANT role
    this.history.push({ role: "assistant", content: fullResponse });
  }
}
