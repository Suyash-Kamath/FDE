import os
from http.server import ThreadingHTTPServer
from .infrastructure.ChatClient import ChatClient
from .infrastructure.SimpleVectorStore import SimpleVectorStore
from .ToolTracker import ToolTracker
from .tools.OrderStatusTool import OrderStatusTool
from .tools.DeliveryEstimateTool import DeliveryEstimateTool
from .tools.RefundAmountTool import RefundAmountTool
from .SupportService import SupportService
from .SupportController import SupportController


def create_application(client=None):
    client = client if client is not None else ChatClient()
    tracker = ToolTracker()
    vector_store = SimpleVectorStore(client)
    service = SupportService(client, vector_store, OrderStatusTool(tracker),
                             DeliveryEstimateTool(tracker), RefundAmountTool(tracker), tracker)
    service.loadKnowledgeBase()
    return SupportController(service), service


def main():
    controller, _ = create_application()
    port = int(os.environ.get("PORT", "8080"))
    with ThreadingHTTPServer(("127.0.0.1", port), controller.handler()) as server:
        print(f"Support agent listening on http://localhost:{port}")
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            pass


if __name__ == "__main__":
    main()
