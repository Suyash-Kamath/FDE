class OrderStatusTool:
    description = 'Get the current status of an order.\n           Use this only when the user asks whether the order\n           is shipped, delivered, cancelled or its current status.\n           Do not use this for expected delivery dates.'

    def __init__(self, tool_tracker):
        self.tool_tracker = tool_tracker

    def getOrderStatus(self, order_id):
        normalized_order_id = order_id.strip().upper()
        orders = {'ORD-101': 'Order ORD-101 status is SHIPPED. It has not been delivered yet.', 'ORD-102': 'Order ORD-102 status is DELIVERED. It was delivered 20 days ago.', 'ORD-103': 'Order ORD-103 status is DELIVERED. A refund has been approved.'}
        result = orders.get(normalized_order_id, 'Order not found.')
        self.tool_tracker.record("getOrderStatus", normalized_order_id, result)
        return result
