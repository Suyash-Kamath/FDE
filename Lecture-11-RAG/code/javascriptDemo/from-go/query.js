// Chat: user question -> rewrite into standalone query (OpenAI chat model)
//                     -> embed query -> Pinecone top-K
//                     -> answer from retrieved context only (OpenAI chat model)
// (port of goDemo/query.go)

import readline from "node:readline";

import { connectIndex, embedTexts, newOpenAIClient } from "./shared.js";

const chatModel = "gpt-5.4-mini"; // cheap + fast; swap for a bigger model if answers need it
const topK = 10;

const rewritePrompt = `You are a query rewriting expert. Based on the provided chat history, rephrase the "Follow Up user Question" into a complete, standalone question that can be understood without the chat history.
Only output the rewritten question and nothing else.`;

const answerPromptTemplate = (context) => `You have to behave like a Data Structure and Algorithm Expert.
You will be given a context of relevant information and a user question.
Your task is to answer the user's question based ONLY on the provided context.
If the answer is not in the context, you must say "I could not find the answer in the provided document."
Keep your answers clear, concise, and educational.

Context: ${context}`;

// RAGChat holds the clients and the conversation history.
// history holds only user/assistant turns; the system prompt is added fresh
// on every call because it differs per call (rewrite vs. answer, and the
// answer prompt contains this turn's retrieved context).
class RAGChat {
  constructor(ai, index) {
    this.ai = ai;
    this.index = index;
    this.history = [];
  }

  static async create() {
    const idx = await connectIndex();
    return new RAGChat(newOpenAIClient(), idx);
  }

  // complete runs one chat completion: system prompt + history + one new user
  // message. It never modifies this.history.
  async complete(system, user) {
    const msgs = [{ role: "system", content: system }, ...this.history, { role: "user", content: user }];

    const resp = await this.ai.chat.completions.create({
      model: chatModel,
      messages: msgs,
    });
    if (resp.choices.length === 0) {
      throw new Error("empty response from model");
    }
    return (resp.choices[0].message.content ?? "").trim();
  }

  // transformQuery rewrites a follow-up question into a standalone one.
  async transformQuery(question) {
    let rewritten;
    try {
      rewritten = await this.complete(rewritePrompt, question);
    } catch (err) {
      throw new Error(`rewrite query: ${err.message ?? err}`);
    }
    if (rewritten === "") {
      return question; // fall back to the original question
    }
    return rewritten;
  }

  async chat(question) {
    const query = await this.transformQuery(question);

    // Same embedTexts as the indexer -> same model and dimensions.
    let vecs;
    try {
      vecs = await embedTexts(this.ai, [query]);
    } catch (err) {
      throw new Error(`embed query: ${err.message ?? err}`);
    }

    let results;
    try {
      results = await this.index.query({
        vector: vecs[0],
        topK,
        includeMetadata: true,
      });
    } catch (err) {
      throw new Error(`pinecone query: ${err.message ?? err}`);
    }

    console.log("\n--- Raw Pinecone matches (JSON) ---");
    console.log(JSON.stringify(results, null, 2));

    // Optional chaining: missing metadata or "text" is simply skipped.
    const parts = [];
    for (const m of results.matches ?? []) {
      const text = m?.metadata?.text;
      if (typeof text === "string" && text !== "") {
        parts.push(text);
      }
    }
    const contextText = parts.join("\n\n---\n\n");

    console.log("\n--- Retrieved context (readable) ---");
    console.log(contextText === "" ? "(no matches found)" : contextText);
    console.log("\n------------------------------------");

    // Generate the answer; commit to history only on success.
    let answer;
    try {
      answer = await this.complete(answerPromptTemplate(contextText), query);
    } catch (err) {
      throw new Error(`generate answer: ${err.message ?? err}`);
    }

    this.history.push({ role: "user", content: query }, { role: "assistant", content: answer });

    console.log(`\n\n${answer}`);
    return contextText;
  }
}

// ==========================================================
// chat command
// ==========================================================

export async function runChat() {
  const chat = await RAGChat.create();

  const rl = readline.createInterface({ input: process.stdin });
  const lines = rl[Symbol.asyncIterator]();

  while (true) {
    process.stdout.write("Ask me anything--> ");
    const { value: line, done } = await lines.next();
    if (done) {
      console.log();
      rl.close();
      return; // Ctrl+D exits cleanly
    }
    const question = line.trim();
    if (question === "") {
      continue;
    }

    try {
      await chat.chat(question);
    } catch (err) {
      console.log("⚠️  Something went wrong:", err.message ?? err);
    }
  }
}
