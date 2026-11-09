class DeliveryEstimateTool:
    description = 'Get the expected delivery date for an order.\n            Use this when the user asks when the order will arrive,\n            when it will be delivered, or asks for a delivery estimate.'

    def __init__(self, tool_tracker):
        self.tool_tracker = tool_tracker

    def getDeliveryEstimate(self, order_id):
        normalized_order_id = order_id.strip().upper()
        orders = {'ORD-101': 'Order ORD-101 is expected to be delivered on 15 October 2026.', 'ORD-102': 'Order ORD-102 has already been delivered.', 'ORD-103': 'Order ORD-103 has already been delivered.'}
        result = orders.get(normalized_order_id, 'Delivery information not found.')
        self.tool_tracker.record("getDeliveryEstimate", normalized_order_id, result)
        return result
