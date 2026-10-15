import assert from "node:assert/strict";
import { test } from "node:test";

import request from "supertest";

import { createApp } from "../app.js";
import { ChatService } from "../chatService.js";

test("frontend, chat, image, history, clear, validation and failure", async () => {
  const requests = [];
  let fail = false;

  // A fake OpenAI client: records every request and returns a canned reply.
  async function complete(req) {
    requests.push(structuredClone(req));
    if (fail) {
      throw new Error("Simulated provider failure");
    }
    return { choices: [{ message: { content: "Test reply" } }] };
  }

  const client = { chat: { completions: { create: complete } } };
  const service = new ChatService(client);
  const browser = request(createApp(service));

  let response = await browser.get("/");
  assert.ok(response.text.includes("Vision Chat"));

  response = await browser.post("/api/chat").type("form").send({ message: "Hello" });
  assert.equal(response.status, 200);
  assert.equal(response.type, "text/plain");
  assert.equal(response.text, "Test reply");
  assert.equal(requests[0].model, "gpt-4o-mini");
  assert.equal(requests[0].temperature, 0.4);
  assert.equal(requests[0].messages[0].role, "system");

  response = await browser
    .post("/api/chat")
    .field("message", "Describe it")
    .attach("image", Buffer.from("image bytes"), {
      filename: "image.png",
      contentType: "image/png",
    });
  assert.equal(response.status, 200);
  assert.equal(requests[1].messages.length, 4);
  const content = requests[1].messages[3].content;
  assert.equal(content[0].text, "Describe it");
  assert.equal(content[1].image_url.url, "data:image/png;base64,aW1hZ2UgYnl0ZXM=");

  assert.equal((await browser.delete("/api/chat")).status, 200);
  await browser.post("/api/chat").type("form").send({ message: "New conversation" });
  assert.equal(requests[2].messages.length, 2);

  assert.equal((await browser.post("/api/chat").type("form").send({})).status, 400);
  assert.equal(
    (
      await browser
        .post("/api/chat")
        .field("message", "Bad file")
        .attach("image", Buffer.from("text"), {
          filename: "file.txt",
          contentType: "text/plain",
        })
    ).status,
    400,
  );
  assert.equal(
    (
      await browser
        .post("/api/chat")
        .field("message", "Too large")
        .attach("image", Buffer.alloc(10 * 1024 * 1024 + 1, "x"), {
          filename: "large.png",
        })
    ).status,
    413,
  );

  fail = true;
  assert.equal(
    (await browser.post("/api/chat").type("form").send({ message: "Failure" })).status,
    502,
  );
  assert.equal(service.history.length, 2);
});
