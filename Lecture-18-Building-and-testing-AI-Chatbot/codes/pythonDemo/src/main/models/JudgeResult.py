from dataclasses import dataclass


@dataclass
class JudgeResult:
    relevance: int
    correctness: int
    faithfulness: int
    helpfulness: int
    hallucinationFree: int
    reason: str
