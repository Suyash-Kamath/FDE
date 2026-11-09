from dataclasses import dataclass


@dataclass
class EvalCase:
    id: str
    question: str
    expectedAnswer: str
    expectedBehavior: str
    expectedTool: str | None
    expectedToolArgument: str | None
    expectedSource: str | None
