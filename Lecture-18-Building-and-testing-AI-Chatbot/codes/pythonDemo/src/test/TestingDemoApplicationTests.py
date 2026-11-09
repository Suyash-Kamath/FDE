import copy
import json
import re
import threading
import unittest
from concurrent.futures import ThreadPoolExecutor
from dataclasses import asdict
from http.server import ThreadingHTTPServer
from urllib.request import Request, urlopen
from urllib.error import HTTPError
from src.main.TestingDemoApplication import create_application
from src.main.infrastructure.ChatClient import ChatClient


class FakeClient(ChatClient):
    def __init__(self):
        super().__init__(api_key="offline-test")
        self.requests = []

    def embed(self, texts):
        return [[1, 0, 0] if re.search(r"\breturn\b|send it back", text, re.I)
                else [0, 1, 0] if re.search(r"refund", text, re.I) else [0, 0, 1] for text in texts]

    def complete(self, messages, tools=None, json_output=False):
        self.requests.append(copy.deepcopy(messages))
        question = next(message["content"] for message in messages if message["role"] == "user")
        results = [message["content"] for message in messages if message["role"] == "tool"]
        if results:
            return {"role": "assistant", "content": " ".join(results)}
        order_id = re.search(r"ord-\d+", question, re.I)
        if order_id:
            name = ("getDeliveryEstimate" if "arrive" in question else
                    "getRefundAmount" if "refund" in question else "getOrderStatus")
            assert len(tools) == 3
            return {"role": "assistant", "content": None, "tool_calls": [{
                "id": "call_1", "type": "function", "function": {
                    "name": name, "arguments": json.dumps({"orderId": order_id[0]})}}]}
        return {"role": "assistant", "content": "Products can be returned within 30 days of delivery."}


class TestingDemoApplicationTests(unittest.TestCase):
    def setUp(self):
        self.client = FakeClient()
        self.controller, self.service = create_application(self.client)

    def test_retrieval_response_fields_and_tool_loop(self):
        response = self.service.chat("How many days do I have to return a product?")
        self.assertEqual(response.retrievedDocuments, ["return-policy.txt"])
        self.assertIn("within 30 days", response.context)
        self.assertEqual(response.toolsUsed, [])
        self.assertEqual(set(asdict(response)), {"answer", "context", "retrievedDocuments",
                                               "toolArguments", "toolResults", "toolsUsed"})
        for question, name, expected in [
            ("Where is ord-101?", "getOrderStatus", "SHIPPED"),
            ("When will ORD-101 arrive?", "getDeliveryEstimate", "15 October 2026"),
            ("How much refund for ORD-103?", "getRefundAmount", "INR 4999"),
            ("Where is ORD-999?", "getOrderStatus", "Order not found"),
        ]:
            with self.subTest(question=question):
                result = self.service.chat(question)
                self.assertEqual(result.toolsUsed, [name])
                self.assertIn(expected, result.answer)
                self.assertEqual(result.toolArguments[name], re.search(r"ord-\d+", question, re.I)[0].upper())
                self.assertEqual(result.toolResults[name], result.answer)

    def test_concurrent_traces_are_isolated(self):
        with ThreadPoolExecutor(max_workers=3) as executor:
            responses = list(executor.map(self.service.chat, ["Where is ORD-101?",
                                  "How much refund for ORD-103?", "What is the return policy?"]))
        self.assertEqual([response.toolsUsed for response in responses],
                         [["getOrderStatus"], ["getRefundAmount"], []])

    def test_http_query_form_and_validation(self):
        server = ThreadingHTTPServer(("127.0.0.1", 0), self.controller.handler())
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            url = f"http://127.0.0.1:{server.server_port}"
            with urlopen(Request(url + "/chat?message=Where%20is%20ORD-101", method="POST")) as response:
                self.assertEqual(response.status, 200)
                self.assertEqual(json.load(response)["toolsUsed"], ["getOrderStatus"])
            with urlopen(Request(url + "/chat", data=b"message=return+policy", headers={
                    "Content-Type": "application/x-www-form-urlencoded"})) as response:
                self.assertEqual(json.load(response)["retrievedDocuments"], ["return-policy.txt"])
            for path, status in [("/chat", 400), ("/missing", 404)]:
                with self.assertRaises(HTTPError) as caught:
                    urlopen(Request(url + path, method="POST"))
                self.assertEqual(caught.exception.code, status)
                caught.exception.close()
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

    def test_invalid_tools_and_loop_limit(self):
        self.client.complete = lambda *args: {"tool_calls": [{"id": "bad", "function": {
            "name": "notAllowed", "arguments": "{}"}}]}
        with self.assertRaisesRegex(ValueError, "Invalid model tool call"):
            self.service.chat("Where is ORD-101?")
        self.client.complete = lambda *args: {"role": "assistant", "tool_calls": [{
            "id": "repeat", "type": "function", "function": {
                "name": "getOrderStatus", "arguments": '{"orderId":"ORD-101"}'}}]}
        with self.assertRaisesRegex(RuntimeError, "Tool-call limit"):
            self.service.chat("Where is ORD-101?")


if __name__ == "__main__":
    unittest.main()
