from meeting_service import MeetingService


def main():
    meeting_service = MeetingService()

    meeting_details = meeting_service.schedule(
        "Schedule a project review with Aditya tomorrow at 3 PM for 45 minutes."
    )

    print(meeting_details.model_dump())


if __name__ == "__main__":
    main()