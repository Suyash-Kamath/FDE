from pathlib import Path
from .models.AgentResponse import AgentResponse

RESOURCES = Path(__file__).parent / "resources"


class SupportService:
    def __init__(self, chat_client, vector_store, order_status_tool,
                 delivery_estimate_tool, refund_amount_tool, tool_tracker):
        self.chat_client = chat_client
        self.vector_store = vector_store
        self.tool_tracker = tool_tracker
        self.registry = {}
        for tool, name in [(order_status_tool, "getOrderStatus"),
                           (delivery_estimate_tool, "getDeliveryEstimate"),
                           (refund_amount_tool, "getRefundAmount")]:
            self.registry[name] = {"description": tool.description, "execute": getattr(tool, name)}

    def loadKnowledgeBase(self):
        documents = [{"text": path.read_text(encoding="utf-8"), "metadata": {"source": path.name}}
                     for path in sorted((RESOURCES / "knowledge").glob("*.txt"))]
        self.vector_store.add(documents)
        self.system_prompt = (RESOURCES / "system-prompt.txt").read_text(encoding="utf-8")
        print(f"Knowledge base loaded: {len(documents)} documents")

    def chat(self, question):
        with self.tool_tracker.scope():
            self.tool_tracker.clear()
            documents = self.vector_store.similaritySearch(question, 1)
            context = "".join(f"SOURCE: {doc['metadata']['source']}\n{doc['text']}\n\n" for doc in documents)
            answer = self.chat_client.chat(self.system_prompt + context, question, self.registry)
            return AgentResponse(answer, [doc["metadata"]["source"] for doc in documents],
                                 self.tool_tracker.getToolsUsed(), self.tool_tracker.getToolArguments(),
                                 self.tool_tracker.getToolResults(), context)
