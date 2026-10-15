export const SYSTEM_PROMPT =
  "You are a helpful AI assistant. Answer questions clearly and simply.";

/**
 * Like the Spring singleton service, history is shared and kept in memory.
 */
export class ChatService {
  constructor(client, model = "gpt-4o-mini") {
    this.client = client;
    this.model = model;
    this.history = [];
    // A promise chain acts as our "lock": every turn waits for the previous one.
    this.queue = Promise.resolve();
  }

  // Run `task` only after every earlier task has finished (success or failure).
  #withLock(task) {
    const run = this.queue.then(task, task);
    this.queue = run.catch(() => {});
    return run;
  }

  /**
   * @param {string} message
   * @param {{ buffer: Buffer, mimetype: string } | undefined} image
   */
  chat(message, image) {
    // Keep simultaneous requests and clearing in a consistent turn order.
    return this.#withLock(async () => {
      let content = message;
      if (image) {
        const imageBytes = image.buffer;
        if (imageBytes && imageBytes.length > 0) {
          const encodedImage = imageBytes.toString("base64");
          content = [
            { type: "text", text: message },
            {
              type: "image_url",
              image_url: {
                url: `data:${image.mimetype};base64,${encodedImage}`,
              },
            },
          ];
        }
      }
      const userMessage = { role: "user", content };
      const response = await this.client.chat.completions.create({
        model: this.model,
        temperature: 0.4,
        messages: [
          { role: "system", content: SYSTEM_PROMPT },
          ...this.history,
          userMessage,
        ],
      });
      const reply = response.choices[0].message.content;
      if (typeof reply !== "string") {
        throw new Error("The model returned no text response.");
      }
      // Only commit a complete turn after the API call succeeds.
      this.history.push(userMessage, { role: "assistant", content: reply });
      return reply;
    });
  }

  clearHistory() {
    return this.#withLock(() => {
      this.history.length = 0;
    });
  }
}
