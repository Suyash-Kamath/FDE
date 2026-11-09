import json
import unittest
from dataclasses import asdict
from src.main.TestingDemoApplication import create_application
from src.main.SupportService import RESOURCES
from src.main.infrastructure.ChatClient import ChatClient
from src.main.models.EvalCase import EvalCase
from src.main.models.JudgeResult import JudgeResult

METRICS = ["relevance", "correctness", "faithfulness", "helpfulness", "hallucinationFree"]


class SupportAgentEvalTest(unittest.TestCase):
    def test_runGoldenDataset(self):
        _, service = create_application()
        judge_client = ChatClient()
        cases = json.loads((RESOURCES / "golden-dataset.json").read_text(encoding="utf-8"))
        rubric = (RESOURCES / "judge-rubric.txt").read_text(encoding="utf-8")
        totals = dict.fromkeys(METRICS, 0)
        failures = []
        for data in cases:
            case = EvalCase(**data)
            response = service.chat(case.question)
            checks = {
                "Response exists": bool(response.answer and response.answer.strip()),
                "Correct tool": (not response.toolsUsed if case.expectedTool is None
                                 else case.expectedTool in response.toolsUsed),
                "Correct tool argument": (case.expectedTool is None or
                                          response.toolArguments.get(case.expectedTool) == case.expectedToolArgument),
                "Expected source retrieved": (case.expectedSource is None or
                                              case.expectedSource in response.retrievedDocuments),
            }
            prompt = (f"Evaluate the AI answer below.\n\nUSER QUESTION:\n{case.question}\n\n"
                      f"EXPECTED ANSWER:\n{case.expectedAnswer}\n\nEXPECTED BEHAVIOR:\n{case.expectedBehavior}\n\n"
                      f"RETRIEVED POLICY CONTEXT:\n{response.context}\n\n"
                      f"TOOL RESULTS:\n{json.dumps(response.toolResults)}\n\nACTUAL ANSWER:\n{response.answer}\n\n"
                      f"{rubric}\n\nReturn a JSON object with integer fields {', '.join(METRICS)} and a string reason.")
            message = judge_client.complete([
                {"role": "system", "content": "You are a strict evaluator of AI application responses. Return JSON."},
                {"role": "user", "content": prompt},
            ], json_output=True)
            judge = JudgeResult(**json.loads(message["content"]))
            for metric in METRICS:
                score = getattr(judge, metric)
                self.assertTrue(type(score) is int and 1 <= score <= 5, f"Invalid judge score: {metric}")
                totals[metric] += score
            passed = all(checks.values()) and all(getattr(judge, metric) >= 4 for metric in METRICS)
            if not passed:
                failures.append(case.id)
            print(json.dumps({"test": case.id, "question": case.question, "answer": response.answer,
                              "deterministic": checks, "quality": asdict(judge),
                              "final": "PASS" if passed else "FAIL"}, indent=2))
        print("FINAL EVALUATION REPORT", {"total": len(cases), "passed": len(cases) - len(failures),
              "failed": len(failures), "averages": {metric: round(totals[metric] / len(cases), 2)
                                                   for metric in METRICS}})
        self.assertEqual(failures, [], f"Failed Eval Cases: {failures}")


if __name__ == "__main__":
    unittest.main()
