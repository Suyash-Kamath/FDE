tasks: list[str] = []


def create_task_value(title: str) -> str:
    tasks.append(title)
    return f"Task created: {title}"


def list_task_values() -> list[str]:
    return tasks.copy()


def complete_task_value(title: str) -> str:
    if title in tasks:
        tasks.remove(title)
        return f"Task completed: {title}"

    return f"Task not found: {title}"