// JavaScript port of the ChatService in goDemo/main_without_using_sdk.go
// Talks to OpenAI over raw HTTP (built-in fetch, no SDK) and keeps the
// conversation history, the same way chat_service.py does.

const openAIResponsesURL = "https://api.openai.com/v1/responses";
const rawModel = "gpt-4o-mini";

const rawSystemPrompt = `
You are a funny AI chatbot. You reply everything sarcastically.
`;

// Reads a Server-Sent Events body line by line and yields the payload of
// every "data: ..." line, stopping at "data: [DONE]".
async function* readSseData(body) {
  const decoder = new TextDecoder();
  let buffer = "";

  for await (const chunk of body) {
    buffer += decoder.decode(chunk, { stream: true });

    const lines = buffer.split("\n");
    buffer = lines.pop(); // the last piece may be an incomplete line

    for (const rawLine of lines) {
      const line = rawLine.replace(/\r$/, "");

      if (!line.startsWith("data: ")) {
        continue;
      }

      const payload = line.slice("data: ".length);

      if (payload === "[DONE]") {
        return;
      }

      yield payload;
    }
  }
}

export class ChatServiceWithoutSdk {
  constructor(apiKey) {
    this.apiKey = apiKey;
    // Mirrors the {"role": ..., "content": ...} shape the Python history list
    // used, sent straight to the /v1/responses endpoint.
    this.history = [];
  }

  // Sends the user's message to OpenAI's Responses API with "stream": true and
  // yields every chunk of the streamed reply as it arrives.
  async *chat(message, signal) {
    // USER role
    this.history.push({ role: "user", content: message });

    const resp = await fetch(openAIResponsesURL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + this.apiKey,
      },
      body: JSON.stringify({
        model: rawModel,
        instructions: rawSystemPrompt,
        input: this.history,
        stream: true,
      }),
      signal,
    });

    if (resp.status !== 200) {
      const errBody = await resp.text();
      throw new Error(`openai request failed: ${resp.status} ${resp.statusText}: ${errBody}`);
    }

    let fullResponse = "";

    for await (const payload of readSseData(resp.body)) {
      // We only care about the "type" and "delta" fields of each event.
      let event;
      try {
        event = JSON.parse(payload);
      } catch {
        continue;
      }

      if (event.type === "response.output_text.delta") {
        fullResponse += event.delta;
        yield event.delta;
      }
    }

    // ASSISTANT role
    this.history.push({ role: "assistant", content: fullResponse });
  }
}
