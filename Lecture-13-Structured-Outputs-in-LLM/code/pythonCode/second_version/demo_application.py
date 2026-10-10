import argparse
import json

from meeting_chat_service import ChatMeetingService
from meeting_service import MeetingService


def run_v1():
    """Your original flow: one message in, MeetingDetails out."""
    meeting_service = MeetingService()

    meeting_details = meeting_service.schedule(
        "Schedule a project review with Aditya tomorrow at 3 PM for 45 minutes."
    )

    print(json.dumps(meeting_details.model_dump(), indent=2))


def run_v2():
    """The back-and-forth flow from ~38:00 in the lecture."""
    chat_service = ChatMeetingService()

    print("Meeting scheduler (v2). Describe your meeting.")
    print("Type 'reset' to start over, 'exit' to quit.")

    while True:
        try:
            user_input = input("\nYou: ").strip()
        except (EOFError, KeyboardInterrupt):
            print()
            break

        command = user_input.lower()
        if not user_input:
            continue
        if command in ("exit", "quit"):
            break
        if command == "reset":
            chat_service.reset()
            print("Bot: Okay, starting over.")
            continue

        try:
            result = chat_service.schedule(user_input)
        except Exception as error:
            print("Error:", error)
            continue

        print("Bot:", result.reply)
        if result.details is not None:
            print(json.dumps(result.details.model_dump(), indent=2))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--version",
        type=int,
        choices=[1, 2],
        default=1,
        help="1 = one-shot extraction, 2 = conversational with history",
    )
    args = parser.parse_args()

    if args.version == 1:
        run_v1()
    else:
        run_v2()


if __name__ == "__main__":
    main()
