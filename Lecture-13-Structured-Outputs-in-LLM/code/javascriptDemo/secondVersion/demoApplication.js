import path from "node:path";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";

import dotenv from "dotenv";

import { ChatMeetingService } from "./meetingChatService.js";
import { MeetingService } from "./meetingService.js";

// Load OPENAI_API_KEY from javascriptDemo/.env (one level up)
const __dirname = path.dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: path.join(__dirname, "..", ".env"), quiet: true });

const printJSON = (value) => console.log(JSON.stringify(value, null, 2));

// Your original flow: one message in, MeetingDetails out.
async function runV1() {
  const meetingService = new MeetingService();

  const meetingDetails = await meetingService.schedule(
    "Schedule a project review with Aditya tomorrow at 3 PM for 45 minutes."
  );

  printJSON(meetingDetails);
}

// The back-and-forth flow from ~38:00 in the lecture.
async function runV2() {
  const chatService = new ChatMeetingService();

  console.log("Meeting scheduler (v2). Describe your meeting.");
  console.log("Type 'reset' to start over, 'exit' to quit.");

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });

  // Ctrl+D (EOF) or Ctrl+C ends the loop, like EOFError / KeyboardInterrupt in Python.
  let interrupted = true;
  rl.on("SIGINT", () => rl.close());

  process.stdout.write("\nYou: ");
  for await (const line of rl) {
    const userInput = line.trim();
    const command = userInput.toLowerCase();

    if (!userInput) {
      process.stdout.write("\nYou: ");
      continue;
    }
    if (command === "exit" || command === "quit") {
      interrupted = false;
      break;
    }
    if (command === "reset") {
      chatService.reset();
      console.log("Bot: Okay, starting over.");
      process.stdout.write("\nYou: ");
      continue;
    }

    try {
      const result = await chatService.schedule(userInput);

      console.log("Bot:", result.reply);
      if (result.details !== null) {
        printJSON(result.details);
      }
    } catch (error) {
      console.log("Error:", error.message ?? error);
    }

    process.stdout.write("\nYou: ");
  }

  rl.close();
  if (interrupted) {
    console.log();
  }
}

async function main() {
  // Python: argparse --version {1,2}, default 1
  const { values } = parseArgs({
    options: {
      version: { type: "string", default: "1" },
    },
  });

  if (values.version !== "1" && values.version !== "2") {
    console.error(`argument --version: invalid choice: '${values.version}' (choose from 1, 2)`);
    process.exit(2);
  }

  if (values.version === "1") {
    await runV1();
  } else {
    await runV2();
  }
}

main();
