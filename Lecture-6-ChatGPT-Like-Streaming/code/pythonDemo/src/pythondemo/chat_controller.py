class ChatController:
    def __init__(self, chat_service):
        self.chat_service = chat_service

    def chat(self, message):
        return self.chat_service.chat(message)