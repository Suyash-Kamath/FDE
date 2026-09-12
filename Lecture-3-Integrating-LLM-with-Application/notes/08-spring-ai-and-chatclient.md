# 08 — What Spring AI Actually Does

The video calls the `ChatClient` setup "boilerplate, don't focus on it." Understanding
it is exactly what separates copying a tutorial from designing a system. This file
explains what the framework buys you, what it costs you, and the correct configuration.

---

## 1. The one-sentence answer

> **Spring AI is to LLM providers what JDBC is to databases**: a vendor-neutral
> interface, auto-configuration, and a set of cross-cutting concerns (retries,
> observability, conversation memory, RAG, tool execution) implemented once instead
> of per-project.

Without it, calling an LLM from Spring Boot means: hand-build the JSON, `RestClient`
POST, hand-parse the envelope, hand-roll retry/backoff, hand-manage history, hand-wire
the tool loop, and rewrite all of it when you add a second provider.

---

## 2. The layers

```
        your @Service
              │
              ▼
      ┌───────────────┐    fluent API, advisors, entity mapping,
      │  ChatClient   │    system/user templating, tool execution loop
      └───────┬───────┘
              ▼
      ┌───────────────┐    the portable interface:
      │   ChatModel   │    ChatResponse call(Prompt)
      └───────┬───────┘
              ▼
   ┌──────────┴───────────┐
   │ OpenAiChatModel      │  Anthropic · Ollama · Azure · Bedrock ·
   │ (+ OpenAiApi)        │  Vertex · Mistral · Groq · DeepSeek …
   └──────────┬───────────┘
              ▼
        HTTP + JSON
```

**`ChatModel`** is the low-level portable interface — one call in, one response out.
**`ChatClient`** is the ergonomic layer on top: fluent builder, advisors, memory,
structured output, tool-call orchestration.

Use `ChatClient` for applications. Drop to `ChatModel` when you want to drive the loop
yourself.

---

## 3. Setup

### Dependency

```xml
<dependency>
    <groupId>org.springframework.ai</groupId>
    <artifactId>spring-ai-starter-model-openai</artifactId>
</dependency>
```

Plus the BOM in `dependencyManagement` so the version is managed centrally. (Older
tutorials use `spring-ai-openai-spring-boot-starter`; the artifact was renamed to
`spring-ai-starter-model-openai`.)

### Configuration — **with a correction to the video**

The video writes `spring.ai.openai.chat.model`. For the OpenAI starter, the model
property lives under **`options`**:

```properties
# ✓ correct
spring.ai.openai.api-key=${OPENAI_API_KEY}
spring.ai.openai.chat.options.model=gpt-5.6-luna
spring.ai.openai.chat.options.temperature=0.2
spring.ai.openai.chat.options.max-tokens=200
```

```yaml
# same thing in YAML, which is easier to read
spring:
  ai:
    openai:
      api-key: ${OPENAI_API_KEY}
      chat:
        options:
          model: gpt-5.6-luna
          temperature: 0.2
          max-tokens: 200
    retry:
      max-attempts: 3
      backoff:
        initial-interval: 2s
        multiplier: 2
      on-client-errors: false     # don't retry 4xx
```

`spring.ai.retry.*` is a genuine freebie — exponential backoff on transient failures
that you'd otherwise hand-write.

### Never hardcode the key

```properties
# ✗ this WILL end up in git
spring.ai.openai.api-key=sk-proj-abc123realkey

# ✓ from the environment
spring.ai.openai.api-key=${OPENAI_API_KEY}
```

The video pastes the key into `application.properties` on screen. Fine for a demo,
disastrous in a repo. Use an env var, and in production a secret manager. Add a
secret scanner to CI.

### Pointing at another provider

Because most providers clone the OpenAI wire format, this often works with **no code
change**:

```yaml
spring:
  ai:
    openai:
      base-url: https://api.groq.com/openai
      api-key: ${GROQ_API_KEY}
      chat:
        options:
          model: llama-3.3-70b-versatile
```

That portability is the strongest argument for the framework.

---

## 4. `ChatClient` — the builder, demystified

The video's constructor:

```java
private final ChatClient chatClient;

public SummarizeService(ChatClient.Builder builder) {
    this.chatClient = builder.build();
}
```

What's happening:

- Auto-configuration creates a **`ChatClient.Builder` bean** pre-wired with your
  configured `ChatModel`, default options, and observability.
- You inject the *builder*, not the client, so each service can apply its **own**
  defaults before calling `.build()`.
- `ChatClient` is thread-safe and immutable once built — build it once in the
  constructor, never per request.

### Why constructor injection beats `@Autowired` on the field

The video shows both and picks `@Autowired` "to simplify". For real code, prefer the
constructor:

- The field can be `final` → immutable, no accidental reassignment.
- Dependencies are explicit in the signature → you notice when a class grows 7 of them.
- Trivially testable without Spring — `new SummarizeService(mockBuilder)`.
- No possibility of an NPE from a not-yet-injected field.
- No circular-dependency surprises at runtime.

With a single constructor Spring injects automatically; you don't even need the
annotation. With Lombok, `@RequiredArgsConstructor` gives you it for free.

### Per-service defaults

```java
public SummarizeService(ChatClient.Builder builder) {
    this.chatClient = builder
        .defaultSystem(SYSTEM_PROMPT)                       // applied to every call
        .defaultOptions(OpenAiChatOptions.builder()
            .model("gpt-5.6-luna")
            .temperature(0.2)
            .maxTokens(200)
            .build())
        .defaultAdvisors(new SimpleLoggerAdvisor())
        .build();
}
```

---

## 5. The fluent API

```java
String answer = chatClient
        .prompt()                         // start building
        .system("You are a concise assistant.")
        .user("Explain Docker in two lines")
        .options(ChatOptions.builder().temperature(0.7).build())  // per-request override
        .call()                           // execute, blocking
        .content();                       // extract the text
```

What each step maps to:

| Spring AI | Raw HTTP |
|---|---|
| `.system(...)` | a `{"role":"system"}` message |
| `.user(...)` | a `{"role":"user"}` message |
| `.options(...)` | top-level body fields (`temperature`, `max_tokens`, …) |
| `.call()` | the `POST` |
| `.content()` | walking `output[].content[].text` for you |

### Beyond `.content()`

```java
// 1. the full response, including usage — USE THIS IN PRODUCTION
ChatResponse resp = chatClient.prompt().user(text).call().chatResponse();
Usage u = resp.getMetadata().getUsage();
log.info("tokens in={} out={}", u.getPromptTokens(), u.getCompletionTokens());
String finishReason = resp.getResult().getMetadata().getFinishReason();

// 2. map straight into a typed object (structured output)
record TicketSummary(String summary, String category, String urgency) {}
TicketSummary s = chatClient.prompt().user(text).call().entity(TicketSummary.class);

// 3. a list
List<String> tags = chatClient.prompt().user(text)
        .call().entity(new ParameterizedTypeReference<List<String>>() {});

// 4. streaming
Flux<String> stream = chatClient.prompt().user(text).stream().content();
```

`.entity()` is a bigger deal than it looks: Spring AI generates a JSON schema from
your record, appends format instructions to the prompt, and deserialises the reply.
It replaces the "parse prose with regex" anti-pattern that causes most production
LLM bugs.

### Prompt templates

```java
.user(u -> u.text("Summarise this {channel} ticket in {n} lines:\n\n{ticket}")
            .param("channel", "email")
            .param("n", 2)
            .param("ticket", ticketText))
```

Cleaner than string concatenation — **but note it does not make you safe from prompt
injection.** See file 09 §3.

---

## 6. Advisors — the interceptor chain

Advisors are the `HandlerInterceptor`/middleware of Spring AI: they wrap the call and
can modify the request going out and the response coming back.

```java
chatClient.prompt()
    .advisors(
        MessageChatMemoryAdvisor.builder(chatMemory).build(),   // conversation history
        QuestionAnswerAdvisor.builder(vectorStore).build(),     // RAG
        new SimpleLoggerAdvisor())                              // logging
    .user(question)
    .call().content();
```

The first one **is the answer to the video's cliffhanger** — "why did it forget my
name?" A chat-memory advisor stores the transcript (in memory, JDBC, Cassandra, Redis)
and re-injects it into every request. It doesn't make the API stateful; it automates
the resending described in file 10.

```java
@Bean
ChatMemory chatMemory(ChatMemoryRepository repo) {
    return MessageWindowChatMemory.builder()
            .chatMemoryRepository(repo)
            .maxMessages(20)          // ← your token-budget lever
            .build();
}
```

Then pass a conversation ID per user so transcripts don't cross-contaminate:

```java
.advisors(a -> a.param(ChatMemory.CONVERSATION_ID, userId))
```

`QuestionAnswerAdvisor` is RAG in one line: embed the question, search the
`VectorStore`, inject the top-k chunks into the prompt. That's the whole pattern —
worth knowing it's this small, because people talk about RAG as if it were a product.

---

## 7. Tool calling in Spring AI

```java
@Component
public class OrderTools {

    @Tool(description = "Look up the current delivery status of an order by its ID")
    public OrderStatus getOrderStatus(
            @ToolParam(description = "The order ID, e.g. 'ORD-8842'") String orderId) {
        return orderRepository.statusOf(orderId);
    }
}
```

```java
chatClient.prompt()
    .user("Where is my order ORD-8842?")
    .tools(orderTools)
    .call().content();
```

Spring AI generates the JSON schema from the method signature, sends the catalogue,
detects the model's tool-call response, **invokes your method**, feeds the result back,
and loops — the whole of file 03, automated.

Do not let that convenience hide the security model. **The model chose the arguments.**
Authorize inside the method, validate the ID, and never annotate a destructive
operation with `@Tool` without an explicit approval step.

---

## 8. Framework vs raw HTTP — the honest trade-off

**Use Spring AI when:**
- You may switch or A/B providers
- You want memory, RAG, and tool calling without hand-rolling them
- You want typed structured output
- You want retries and Micrometer observability for free
- You're already a Spring shop

**Skip it when:**
- You make one kind of call to one provider — a `RestClient` and a record is ~40 lines
  and you control every byte
- You need a brand-new provider feature the abstraction hasn't exposed yet
- The abstraction leaks: provider-specific options (`OpenAiChatOptions`, reasoning
  effort, cache control) are where portability quietly ends
- You're not on the JVM

Note that abstraction layers cannot make models portable. Swapping provider with one
config line is the *easy* half; prompts tuned for one model often need re-tuning for
another, and your eval suite is the thing that tells you. **Portability of the API is
not portability of behaviour.**

---

## 9. If you're not on Java

Same concepts, different names:

| Concern | Java | Python | Node | Go |
|---|---|---|---|---|
| Official SDK | — | `openai` | `openai` | `openai-go` |
| Framework | Spring AI | LangChain / LlamaIndex / Pydantic-AI | LangChain.js / Vercel AI SDK | langchaingo, or plain `net/http` |
| Structured output | `.entity(Record.class)` | Pydantic models | Zod schemas | struct + `json.Unmarshal` |
| Memory | `ChatMemory` advisor | `ConversationBufferMemory` | message array | your own slice |

In Go the honest answer is usually **no framework**: the whole client is a struct, an
`http.Client` with a timeout, and `encoding/json`. The Go ecosystem's frameworks are
thinner than Spring AI's, and the raw API is simple enough that the abstraction often
costs more than it saves. Where you do want help, it's usually for the *tool loop* and
*memory*, not the HTTP call.

```go
type Request struct {
    Model           string    `json:"model"`
    Input           []Message `json:"input"`
    Temperature     float64   `json:"temperature"`
    MaxOutputTokens int       `json:"max_output_tokens"`
}
// …that's most of it.
```

---

## 10. Checkpoint questions

1. What's the difference between `ChatModel` and `ChatClient`, and when would you use
   the lower one?
2. Why inject `ChatClient.Builder` rather than `ChatClient`?
3. Correct this: `spring.ai.openai.chat.model=gpt-5.6-luna`
4. Which advisor answers the "it forgot my name" problem, and what does it physically do?
5. Your team switches from OpenAI to Anthropic with one config line. Name two things
   that will still break.

→ Next: [`09-support-ticket-summarizer.md`](09-support-ticket-summarizer.md)
