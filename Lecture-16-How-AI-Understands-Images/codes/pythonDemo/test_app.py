import copy
import io
import unittest
from types import SimpleNamespace

from app import create_app
from chat_service import ChatService


class ChatDemoTest(unittest.TestCase):
    def test_frontend_chat_image_history_clear_validation_and_failure(self):
        requests = []
        fail = False

        def complete(**request):
            requests.append(copy.deepcopy(request))
            if fail:
                raise RuntimeError("Simulated provider failure")
            return SimpleNamespace(choices=[SimpleNamespace(message=SimpleNamespace(content="Test reply"))])

        client = SimpleNamespace(chat=SimpleNamespace(completions=SimpleNamespace(create=complete)))
        service = ChatService(client)
        browser = create_app(service).test_client()
        response = browser.get("/")
        self.assertIn(b"Vision Chat", response.data)
        response.close()
        response = browser.post("/api/chat", data={"message": "Hello"})
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.mimetype, "text/plain")
        self.assertEqual(response.data, b"Test reply")
        self.assertEqual(requests[0]["model"], "gpt-4o-mini")
        self.assertEqual(requests[0]["temperature"], 0.4)
        self.assertEqual(requests[0]["messages"][0]["role"], "system")
        response = browser.post("/api/chat", data={
            "message": "Describe it",
            "image": (io.BytesIO(b"image bytes"), "image.png", "image/png"),
        })
        self.assertEqual(response.status_code, 200)
        self.assertEqual(len(requests[1]["messages"]), 4)
        content = requests[1]["messages"][3]["content"]
        self.assertEqual(content[0]["text"], "Describe it")
        self.assertEqual(content[1]["image_url"]["url"], "data:image/png;base64,aW1hZ2UgYnl0ZXM=")
        self.assertEqual(browser.delete("/api/chat").status_code, 200)
        browser.post("/api/chat", data={"message": "New conversation"})
        self.assertEqual(len(requests[2]["messages"]), 2)
        self.assertEqual(browser.post("/api/chat", data={}).status_code, 400)
        self.assertEqual(browser.post("/api/chat", data={
            "message": "Bad file", "image": (io.BytesIO(b"text"), "file.txt", "text/plain"),
        }).status_code, 400)
        self.assertEqual(browser.post("/api/chat", data={
            "message": "Too large", "image": (io.BytesIO(b"x" * (10 * 1024 * 1024 + 1)), "large.png"),
        }).status_code, 413)
        fail = True
        self.assertEqual(browser.post("/api/chat", data={"message": "Failure"}).status_code, 502)
        self.assertEqual(len(service.history), 2)


if __name__ == "__main__":
    unittest.main()