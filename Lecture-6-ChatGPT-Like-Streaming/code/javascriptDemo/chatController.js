// JavaScript port of pythonDemo/src/pythondemo/chat_controller.py
// Thin layer on top of the chat service. Works with either ChatService (SDK)
// or ChatServiceWithoutSdk (raw HTTP) because both expose the same chat() generator.

export class ChatController {
  constructor(chatService) {
    this.chatService = chatService;
  }

  chat(message, signal) {
    return this.chatService.chat(message, signal);
  }
}
