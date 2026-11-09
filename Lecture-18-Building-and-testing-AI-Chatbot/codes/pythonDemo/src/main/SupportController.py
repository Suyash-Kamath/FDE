import json
from dataclasses import asdict
from http.server import BaseHTTPRequestHandler
from urllib.parse import urlsplit, parse_qs


class SupportController:
    def __init__(self, support_service):
        self.support_service = support_service

    def chat(self, message):
        return self.support_service.chat(message)

    def handler(self):
        controller = self

        class Handler(BaseHTTPRequestHandler):
            def send_json(self, status, body):
                data = json.dumps(body).encode()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):
                self.send_json(404, {"error": "Not found"})

            def do_POST(self):
                url = urlsplit(self.path)
                if url.path != "/chat":
                    self.send_json(404, {"error": "Not found"})
                    return
                parameters = parse_qs(url.query)
                if "message" not in parameters and self.headers.get("Content-Type", "").startswith(
                        "application/x-www-form-urlencoded"):
                    try:
                        length = int(self.headers.get("Content-Length", "0"))
                        if not 0 <= length <= 65536:
                            self.send_json(413, {"error": "Request too large"})
                            return
                        parameters = parse_qs(self.rfile.read(length).decode())
                    except (ValueError, UnicodeError):
                        self.send_json(400, {"error": "Invalid form data"})
                        return
                message = parameters.get("message", [""])[0]
                if not message.strip():
                    self.send_json(400, {"error": "message is required"})
                    return
                try:
                    response = controller.chat(message)
                except Exception as error:
                    self.log_error("Support response failed: %s", type(error).__name__)
                    self.send_json(502, {"error": "Unable to generate a support response"})
                    return
                self.send_json(200, asdict(response))

        return Handler
