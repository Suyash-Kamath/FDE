import json
import os
from urllib.request import Request, urlopen
from urllib.error import HTTPError


class ChatClient:
    def __init__(self, api_key=None, model=None, embedding_model=None,
                 base_url="https://api.openai.com/v1"):
        self.api_key = api_key or os.environ.get("OPENAI_API_KEY")
        if not self.api_key:
            raise ValueError("Set OPENAI_API_KEY before starting the application")
        self.model = model or os.environ.get("OPENAI_CHAT_MODEL", "gpt-4o-mini")
        self.embedding_model = embedding_model or os.environ.get(
            "OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
        self.base_url = base_url

    def post(self, path, body):
        request = Request(f"{self.base_url}/{path}", data=json.dumps(body).encode(), headers={
            "Authorization": f"Bearer {self.api_key}", "Content-Type": "application/json",
        }, method="POST")
        try:
            with urlopen(request, timeout=60) as response:
                return json.load(response)
        except HTTPError as error:
            raise RuntimeError(f"OpenAI {path} failed (HTTP {error.code})") from None

    def embed(self, texts):
        result = self.post("embeddings", {"model": self.embedding_model, "input": texts})
        return [item["embedding"] for item in sorted(result["data"], key=lambda item: item["index"])]

    def complete(self, messages, tools=None, json_output=False):
        body = {"model": self.model, "temperature": 0.2, "messages": messages}
        if tools:
            body["tools"] = tools
        if json_output:
            body["response_format"] = {"type": "json_object"}
        return self.post("chat/completions", body)["choices"][0]["message"]

    def chat(self, system, question, registry):
        messages = [{"role": "system", "content": system}, {"role": "user", "content": question}]
        tools = [{"type": "function", "function": {
            "name": name, "description": tool["description"], "parameters": {
                "type": "object", "properties": {"orderId": {
                    "type": "string", "description": "Order ID such as ORD-101"}},
                "required": ["orderId"], "additionalProperties": False,
            },
        }} for name, tool in registry.items()]
        for _ in range(10):
            message = self.complete(messages, tools)
            if not message.get("tool_calls"):
                if not (message.get("content") or "").strip():
                    raise RuntimeError("Model returned an empty answer")
                return message["content"]
            messages.append(message)
            for call in message["tool_calls"]:
                tool = registry.get(call["function"]["name"])
                args = json.loads(call["function"]["arguments"])
                order_id = args.get("orderId")
                if not tool or not isinstance(order_id, str) or not order_id.strip():
                    raise ValueError("Invalid model tool call")
                messages.append({"role": "tool", "tool_call_id": call["id"],
                                 "content": tool["execute"](order_id)})
        raise RuntimeError("Tool-call limit exceeded")
