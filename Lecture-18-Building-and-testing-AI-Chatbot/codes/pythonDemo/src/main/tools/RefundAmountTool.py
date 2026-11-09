class RefundAmountTool:
    description = 'Get the approved refund amount for an order.\n            Use this when the user asks how much money\n            will be refunded.'

    def __init__(self, tool_tracker):
        self.tool_tracker = tool_tracker

    def getRefundAmount(self, order_id):
        normalized_order_id = order_id.strip().upper()
        orders = {'ORD-103': 'The approved refund amount for ORD-103 is INR 4999.', 'ORD-101': 'No refund has been approved for ORD-101.', 'ORD-102': 'No refund has been approved for ORD-102.'}
        result = orders.get(normalized_order_id, 'Refund information not found.')
        self.tool_tracker.record("getRefundAmount", normalized_order_id, result)
        return result
