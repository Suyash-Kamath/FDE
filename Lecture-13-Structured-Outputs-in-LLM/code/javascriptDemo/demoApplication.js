import path from "node:path";
import { fileURLToPath } from "node:url";

import dotenv from "dotenv";

import { MeetingService } from "./meetingService.js";

// Load OPENAI_API_KEY from javascriptDemo/.env
const __dirname = path.dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: path.join(__dirname, ".env"), quiet: true });

async function main() {
  const meetingService = new MeetingService();

  const meetingDetails = await meetingService.schedule(
    "Schedule a project review with Aditya tomorrow at 3 PM for 45 minutes."
  );

  console.log(meetingDetails);
}

main();
