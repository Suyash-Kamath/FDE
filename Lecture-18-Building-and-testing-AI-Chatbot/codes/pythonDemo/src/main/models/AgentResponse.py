from dataclasses import dataclass


@dataclass
class AgentResponse:
    answer: str
    retrievedDocuments: list[str]
    toolsUsed: list[str]
    toolArguments: dict[str, str]
    toolResults: dict[str, str]
    context: str
