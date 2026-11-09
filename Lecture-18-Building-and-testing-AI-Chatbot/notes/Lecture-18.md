Here's the full walkthrough, built from the transcript, the PDF notes, and your Excalidraw board (which mirrors the whiteboard from the video).

---

## 1. The problem this video solves

In every app built earlier in the series, the flow was:

```
Client → Server → LLM → Server → Client
```

Whatever the LLM said went straight back to the user, and nobody checked it. The previous video covered the theory of how to check it. This video applies that theory to a real application.

The key point in this video is that **an AI application is a pipeline, not a function.** A traditional function goes `input → logic → output`, so you assert on the output. An AI app goes:

```
Question → Retrieval → Tool decision → Tool call (with arguments) → Tool result → Generation → Answer
```

Any of those stages can fail. If you only look at the final answer, a wrong answer tells you something broke but not where. The video's approach is to (a) build an app that exercises every stage, (b) make each stage visible, and (c) test each stage with the right kind of check.

---

## 2. The starting point: a plain Spring AI chatbot

The baseline code has three layers:

- **SupportController**: exposes `POST /chat`, takes the user's message, and passes it to the service.
- **SupportService**: holds a `ChatClient`, injected via the constructor. It does `chatClient.prompt().user(question).call().content()` and returns the string.
- **ChatClient**: Spring AI's wrapper over whichever LLM provider is configured.

He tests it in Postman ("Hello, my name is Aditya" → "Hello Aditya, how can I assist you today?"). It works, but it has no system prompt, no knowledge, no tools, and nothing to evaluate beyond the raw text. The rest of the video turns this into something worth testing.

---

## 3. The three question categories, and why they were chosen

The app is designed so each type of question exercises a different part of the pipeline, which gives each one different things to test.

### Type A: Knowledge-based questions (RAG)

Example: *"How many days do I have to return a product?"*

The LLM can't know your company's private policy, so the answer must come from your documents. There are three small text files (`refund-policy.txt`, `return-policy.txt`, `shipping-policy.txt`). They are embedded and stored in a vector store. At query time, the most similar document is retrieved and handed to the LLM as context.

RAG creates **two separate failure points**, and that is what makes it good for teaching evals:

1. **Retrieval failure**: the vector store returned the wrong document (e.g., shipping policy for a returns question). The LLM then answers from the wrong facts, and it isn't really the LLM's fault.
2. **Generation failure**: the right document was retrieved, but the LLM still misread or ignored it.

From the outside, both look the same: a wrong answer. Debugging requires knowing which one happened. This is the PDF's "Retrieval Evaluation vs Generation Evaluation" split.

### Type B: Tool-based questions (live, order-specific data)

Example: *"Where is ORD-101?"*

No policy document can answer this. A policy says "orders usually arrive in 3–5 business days"; it can't say "*your* order shipped yesterday." That is **dynamic** information, so the LLM needs tools:

| Tool | Purpose | Input |
|---|---|---|
| `getOrderStatus` | shipped / delivered / cancelled | orderId |
| `getDeliveryEstimate` | ETA for delivery | orderId |
| `getRefundAmount` | refund amount / status | orderId |

The video makes the distinction explicit: **documents hold static information, and tools fetch dynamic information.** Every real support system has both.

Tools also create two new things to test:

1. **Tool selection**: for "when will my order arrive?", did the model call `getDeliveryEstimate`, or did it wrongly call `getOrderStatus`?
2. **Tool arguments**: it picked the right tool, but did it pass `ORD-101`, or hallucinate `ORD-999`?

These are separate behaviours. The PDF's example shows the right tool with the wrong argument, giving **Tool Selection: PASS, Tool Argument: FAIL**. If you didn't record arguments, you'd never see that distinction.

### Type C: Combined questions (RAG + tools + reasoning)

Example: *"My order ORD-102 was delivered 20 days ago. Can I still return it?"*

Answering this needs three things:

- **From RAG:** the return policy says 30 days.
- **From a tool:** `getOrderStatus("ORD-102")` confirms it was actually delivered. The user *claimed* it was, but the system should verify that rather than trust it.
- **Reasoning:** 20 < 30, so the order is within the window, and the answer is yes.

Type C tests everything at once: correct document retrieved, correct tool called, correct argument passed, and a correct combination of the two. As you'll see in section 10, this is the case that failed.

---

## 4. Building the application

### 4.1 The knowledge base

The files live in `src/main/resources/knowledge/`. Each is a few lines long. For example, the return policy says items can be returned within 30 days of delivery and must be unused and in original condition.

`SupportService` has a `loadKnowledgeBase()` method that runs at startup, before any chat request. It works like this:

```java
List<Document> documents = new ArrayList<>();
for (Resource resource : resources) {
    String fileName = resource.getFilename();             // "return-policy.txt"
    String content  = resource.getContentAsString(UTF_8);
    Map<String, Object> metadata = Map.of("source", fileName);
    documents.add(new Document(content, metadata));
}
vectorStore.add(documents);   // embeddings are computed here
System.out.println("Knowledge base loaded");
```

Two details matter here.

**Metadata `source` is what makes retrieval testable.** A vector store `Document` has text (the content) and metadata (information about the content). By storing the filename under `source`, every retrieved chunk carries its origin. Later the code can record "I retrieved `return-policy.txt`" and compare that string against the golden dataset. Without this metadata, you'd have a blob of text and no reliable way to say which document it came from.

**No chunking.** The documents are tiny, so each whole file is treated as one chunk. That's why retrieval uses `topK = 1`, since one document equals one chunk, which equals one relevant hit. He notes that with real documents you'd chunk and raise topK. When you do, the "expected source" check has to become "is the expected source *among* the retrieved chunks."

### 4.2 The tools

Each tool lives in its own class under `tools/`. Here is the delivery estimate tool, reconstructed from the transcript:

```java
@Component
public class DeliveryEstimateTool {
    private final ToolTracker toolTracker;

    public DeliveryEstimateTool(ToolTracker toolTracker) { this.toolTracker = toolTracker; }

    @Tool(description = "Get the expected delivery date for an order")
    public String getDeliveryEstimate(String orderId) {
        String normalized = orderId.trim().toUpperCase();   // " ord-101 " → "ORD-101"

        String result;
        if (normalized.equals("ORD-101"))      result = "Order ORD-101 is expected to be delivered on 15 October 2026";
        else if (normalized.equals("ORD-102")) result = "...";
        else if (normalized.equals("ORD-103")) result = "...";
        else                                   result = "Delivery information not found";

        toolTracker.record("getDeliveryEstimate", normalized, result);
        return result;
    }
}
```

(The transcript's speech-to-text renders the IDs as "ORD 1001" and "ORD 102"; the PDF uses `ORD-101`, `ORD-102`, `ORD-103`.)

**The `@Tool` description is effectively a prompt.** The LLM chooses tools based on these descriptions. If two descriptions are vague or overlap, tool-selection failures follow. When a tool-selection eval fails, the description is one of the first things to fix.

**Normalization** (trim plus uppercase) means `ord-101` and `ORD-101 ` match. This also makes the argument eval fairer: you're comparing canonical forms, not penalizing whitespace.

**The results are hard-coded** because there's no real order database. That's fine for the lesson, and it has a useful side effect: tool outputs are fully deterministic, so the tool result itself could also be asserted exactly. He mentions an "expected tool result" field for this, though it doesn't make it into the final dataset.

The **most important line** is `toolTracker.record(...)`. Each tool reports its own invocation.

### 4.3 The ToolTracker, which is how traces are captured

The flow is: the LLM decides to call a tool, Spring AI executes the Java method on the LLM's behalf, the method returns a string, and Spring AI feeds that string back to the LLM. **Your code never sees this happen.** `.call().content()` gives you only the final text. The tool calls happen inside the framework's loop.

The ToolTracker solves this by having the tool itself report each call:

```java
@Component
public class ToolTracker {
    private final List<String> toolsUsed = new ArrayList<>();
    private final Map<String, String> toolArguments = new HashMap<>();
    private final Map<String, String> toolResults   = new HashMap<>();

    public void record(String toolName, String argument, String result) {
        if (!toolsUsed.contains(toolName)) toolsUsed.add(toolName);
        toolArguments.put(toolName, argument);
        toolResults.put(toolName, result);
    }

    public void clear() { toolsUsed.clear(); toolArguments.clear(); toolResults.clear(); }
    // getters...
}
```

It keeps a list of tool names, plus two maps: tool name to argument, and tool name to result. `clear()` is called at the start of every chat request so the previous request's data doesn't leak into the next one.

### 4.4 AgentResponse: the trace object

`chat()` now returns `AgentResponse` instead of a `String`:

```java
public class AgentResponse {
    String answer;                       // what the user sees
    List<String> retrievedDocuments;     // ["return-policy.txt"]
    List<String> toolsUsed;              // ["getOrderStatus"]
    Map<String, String> toolArguments;   // {getOrderStatus: "ORD-102"}
    Map<String, String> toolResults;     // {getOrderStatus: "Delivered on ..."}
    String context;                      // the exact policy text given to the LLM
}
```

The PDF diagram shows it as one execution with two outputs: the **user answer** and the **eval trace**. The user doesn't need the trace. Testing, debugging, and observability do.

> **One correction to how the video phrases this.** He repeatedly says "we'll tell the LLM to return the traces." That isn't what the code does, and it shouldn't. The LLM only produces `answer`. Every other field is filled in **by your own code**: `retrievedDocuments` by the service during retrieval, the tool fields by the ToolTracker, and `context` by the service. This matters. If you asked the LLM to *report* which tools it used, it could misreport, and you'd be testing an LLM with another LLM-generated claim. The trace is only trustworthy because it's recorded by instrumentation, not self-reported. Keep that distinction clear in your head.

### 4.5 The full `chat()` method, step by step

```java
public AgentResponse chat(String question) {
    // 1. Fresh trace for this request
    toolTracker.clear();

    // 2. Retrieval
    List<Document> docs = vectorStore.similaritySearch(
        SearchRequest.builder().query(question).topK(1).build());

    // 3. Build context + record which sources were retrieved
    List<String> retrievedDocuments = new ArrayList<>();
    StringBuilder contextBuilder = new StringBuilder();
    for (Document d : docs) {
        String source = (String) d.getMetadata().get("source");
        retrievedDocuments.add(source);
        contextBuilder.append(source).append("\n").append(d.getText()).append("\n");
    }
    String context = contextBuilder.toString();

    // 4. System prompt with the context embedded in it
    String systemPrompt = """
        You are an e-commerce customer support agent...
        Use the policy context below...
        POLICY CONTEXT:
        """ + context;

    // 5. Call the LLM with tools available
    String answer = chatClient.prompt()
        .system(systemPrompt)
        .user(question)
        .tools(orderStatusTool, deliveryEstimateTool, refundAmountTool)
        .call()
        .content();

    // 6. Assemble the trace
    return new AgentResponse(answer, retrievedDocuments,
        toolTracker.getToolsUsed(), toolTracker.getToolArguments(),
        toolTracker.getToolResults(), context);
}
```

During step 5, the LLM may emit a tool-call request (as JSON with arguments). Spring AI runs the Java method, the tool calls `toolTracker.record(...)`, and the result goes back to the LLM. This can loop several times before the final text comes out. By step 6, the tracker holds everything that happened.

Note that **retrieval always runs**, even for pure tool questions. That's why "When will ORD-101 be delivered?" came back with `retrievedDocuments: [shipping-policy.txt]`. It was the closest match, even though the answer came from the tool. Keep this in mind for the eval section.

### 4.6 The Postman demo

He asks "When will my order ORD-101 be delivered?" and gets back:

- `answer`: "Your order ORD-101 is expected to be delivered on October 15, 2026"
- `retrievedDocuments`: `["shipping-policy.txt"]`
- `toolsUsed`: `["getDeliveryEstimate"]`
- `toolArguments`: `{getDeliveryEstimate: "ORD-101"}`
- `toolResults`: the hard-coded string
- `context`: the full shipping policy text

Every stage of the pipeline is now visible. Compare that to the earlier "Yes, you can return it" response, which told you nothing about how the answer was produced.

---

## 5. The golden dataset

A golden dataset is a fixed set of test questions where **you already know what correct behaviour looks like.** It's the AI equivalent of a test fixture. It lives at `src/test/resources/golden-dataset.json`. Eight cases cover returns, refunds, shipping, tool calls, and the combined case.

A RAG case:

```json
{
  "id": "RAG-RETURN-01",
  "question": "How many days do I have to return a product?",
  "expectedAnswer": "Products can be returned within 30 days of delivery.",
  "expectedBehavior": "Explain that the product may be returned within 30 days of delivery.",
  "expectedTool": null,
  "expectedToolArgument": null,
  "expectedSource": "return-policy.txt"
}
```

A tool case:

```json
{
  "id": "TOOL-STATUS-01",
  "question": "Where is order ORD-101?",
  "expectedTool": "getOrderStatus",
  "expectedToolArgument": "ORD-101",
  "expectedSource": null
}
```

A combined case:

```json
{
  "id": "COMBINED-RETURN-01",
  "question": "My order ORD-102 was delivered 20 days ago. Can I still return it?",
  "expectedTool": "getOrderStatus",
  "expectedToolArgument": "ORD-102",
  "expectedSource": "return-policy.txt"
}
```

The fields split cleanly into two groups, and this split is the core idea of the whole video:

| Field | Who checks it | Why |
|---|---|---|
| `expectedTool` | code | exact string match |
| `expectedToolArgument` | code | exact string match |
| `expectedSource` | code | exact string match |
| `expectedAnswer` | LLM judge | same meaning can use different words |
| `expectedBehavior` | LLM judge | describes intent, not exact text |

Notice the second RAG question: *"I received an item three weeks ago, is it too late to send it back?"* It never says "return" or "30 days." It's paraphrased on purpose, to test whether retrieval works on meaning rather than keyword overlap. Good golden datasets include paraphrases like this, not just questions that copy the document's wording.

The JSON is deserialized into an `EvalCase` Java class with Jackson's `ObjectMapper`, so the test works with typed objects.

---

## 6. The test class: SupportAgentEvaluationTest

It lives under `src/test/java` and runs with `mvn test`. It's a `@SpringBootTest`, so the **real** application starts: real vector store, real embeddings, real LLM, real tools. The PDF stresses that **the goal is not to mock the AI layer.** You can't evaluate an LLM's behaviour with a fake LLM.

Its dependencies:

- `SupportService`: the system under test.
- `ChatClient.Builder`: used to build a second client, the **judge**.
- `ObjectMapper`: to load the golden dataset.

The overall flow of the single `@Test` method:

```
load golden dataset → for each case:
    run real app → deterministic checks → LLM judge → threshold → final pass/fail
    accumulate scores, record failures
→ print averages, pass/fail summary
```

---

## 7. Layer 1: Deterministic evaluation

The principle, from the PDF: **anything you can reliably check with normal code, check with normal code.** It's free, instant, and gives the same answer every time. An LLM judge is slower, costs money, and is itself non-deterministic. Asking an LLM "did it call getOrderStatus?" when `toolsUsed` already tells you is wasteful and less reliable.

There are four checks.

**1. Response exists**

```java
boolean responsePass = response.getAnswer() != null && !response.getAnswer().isBlank();
```

This is the most basic smoke test. It catches crashes, empty generations, and timeouts that return nothing.

**2. Correct tool**

```java
boolean toolPass;
if (evalCase.getExpectedTool() == null) {
    toolPass = response.getToolsUsed().isEmpty();          // no tool expected → none should be called
} else {
    toolPass = response.getToolsUsed().contains(evalCase.getExpectedTool());
}
```

The null branch is easy to overlook, and it matters. For "How many days to return?", calling `getOrderStatus` would be wrong: it adds latency and cost, and possibly irrelevant content. So the check is "no tool expected" means "no tool called," which is stricter than "anything goes."

**3. Correct argument**

```java
boolean argumentPass = true;
if (evalCase.getExpectedToolArgument() != null) {
    String actual = response.getToolArguments().get(evalCase.getExpectedTool());
    argumentPass = evalCase.getExpectedToolArgument().equals(actual);
}
```

It defaults to `true` because if no argument is expected, there's nothing to fail.

**4. Correct source**

```java
boolean sourcePass = true;
if (evalCase.getExpectedSource() != null) {
    sourcePass = response.getRetrievedDocuments().contains(evalCase.getExpectedSource());
}
```

This is the **retrieval eval**, and it tests your vector store and embeddings, not the LLM.

**Combined:**

```java
boolean deterministicPass = responsePass && toolPass && argumentPass && sourcePass;
```

---

## 8. Layer 2: LLM-as-a-Judge (quality evaluation)

Some things can't be checked with `equals()`. "Products can be returned within 30 days of delivery" and "You've got a month from delivery to send it back" mean the same thing with almost no shared words. Questions like "is it correct?", "is it grounded?", and "is it helpful?" require understanding language. For those, a second LLM call grades the first.

### The five metrics

| Metric | Question the judge answers | What it compares |
|---|---|---|
| **Relevance** | Did it answer what was actually asked? | question vs answer |
| **Correctness** | Does it match the expected behaviour? | expected answer/behaviour vs answer |
| **Faithfulness** | Is every claim supported by the evidence given? | context + tool results vs answer |
| **Helpfulness** | Is it clear and useful to the customer? | answer quality |
| **Hallucination-free** | Did it avoid inventing anything? | answer vs all evidence |

Correctness and faithfulness are easy to confuse, so here's the difference. **Correctness** compares against *the truth* (the golden answer). **Faithfulness** compares against *what the model was given* (the context). An answer can be faithful but incorrect: if retrieval fetched the wrong document and the model faithfully summarized it, that answer is faithful to bad evidence and wrong overall. An answer can also be correct but unfaithful: the model guessed "30 days" from general world knowledge while the context said something else, and happened to be right. That second case is dangerous, because it will be wrong the day your policy changes. Scoring both is what lets you tell these situations apart.

This is also why storing `context` in `AgentResponse` matters. A faithfulness judge needs the question, the exact context the model saw, and the answer. Without `context`, the judge can't tell grounded from guessed.

### Why "hallucinationFree" instead of "hallucination"

If "hallucination: 5" meant "hallucinated a lot," that one metric would run in the opposite direction from the other four. A single rule like "all scores ≥ 4" or "average the scores" would then break. Renaming it so that **5 is always good** keeps every metric pointing the same way, which makes thresholds and aggregation trivial.

### Same model, different role

The judge uses the **same LLM** as the support agent, built from the same `ChatClient.Builder`. He points out that an LLM has no persistent identity between calls. It only sees what you send in that request. Change the system prompt to "You are a strict evaluator of AI application responses," and it acts as a judge.

### The judge prompt

The system prompt is short ("strict evaluator"). The detailed instructions go in the user message:

```
Evaluate the AI support agent's answer.

User question:        {question}
Expected answer:      {expectedAnswer}
Expected behavior:    {expectedBehavior}
Retrieved policy context: {response.context}
Tools used:           {response.toolsUsed}
AI answer:            {response.answer}

Score each 1–5:
- relevance: 5 = directly answers the question ... 1 = off-topic
- correctness: 5 = fully consistent with expected answer ...
- faithfulness: 5 = every claim supported by context/tool results ...
- helpfulness: ...
- hallucinationFree: 5 = no unsupported information ...

Use ONLY the expected answer, expected behavior and policy details.
Do not use outside knowledge.
Return JSON with relevance, correctness, faithfulness, helpfulness, hallucinationFree, reason.
```

Section 25 of the PDF makes the key point here: **the judge is only as good as its prompt.** "Tell me if this answer is good" gives inconsistent, vague scores. The judge needs to know what evidence it may use, what each metric means, what each score level means, and how to treat unsupported claims. The line "do not use outside knowledge" is especially important. Without it, the judge might mark an answer "correct" because it matches typical e-commerce policies in general rather than *your* policy.

### Structured output

```java
JudgeResult result = judgeClient.prompt()
    .system("You are a strict evaluator of AI application responses.")
    .user(judgePrompt)
    .call()
    .entity(JudgeResult.class);
```

`.entity()` instead of `.content()` makes Spring AI append format instructions and parse the reply into a Java object. A free-text verdict like "The answer seems pretty good overall..." can't be thresholded, averaged, compared across prompt versions, or stored. Integers can. The `reason` field is for humans: it's what you read when a case fails.

---

## 9. Layers 3 and 4: Thresholds and the final verdict

```java
boolean qualityPass = result.getRelevance() >= 4 && result.getCorrectness() >= 4
                   && result.getFaithfulness() >= 4 && result.getHelpfulness() >= 4
                   && result.getHallucinationFree() >= 4;

boolean finalPass = deterministicPass && qualityPass;
if (!finalPass) failures.add(evalCase.getId());
```

(He says "above four" verbally, but he clearly means ≥ 4, since a 4/5 on helpfulness passed.)

**Why 4?** No universal reason. The PDF is explicit that **thresholds are product decisions, not constants.** For e-commerce support, 4 is fine. For medical or banking information, he'd require 5/5 on correctness and faithfulness. For a creative-writing app, you might relax faithfulness entirely, since inventing things is the point. You'd also usually threshold metrics differently. For example, hallucinationFree must be 5 while helpfulness ≥ 3 is acceptable.

**Why AND both layers?** Because a response can fail in ways only one layer catches:

| Situation | Deterministic | Judge |
|---|---|---|
| Right answer, but wrong tool called (lucky guess) | ❌ catches it | ✅ misses it |
| Right document retrieved, but wrong conclusion | ✅ misses it | ❌ catches it |
| Right tool and source, but extra invented details | ✅ misses it | ❌ catches it |

Neither layer alone is enough. Together they tell you exactly where the failure happened.

**Aggregation:** inside the loop, each metric's score is added to a running total (`relevanceTotal += ...`). After the loop, each total is divided by the number of cases to get an average per metric. Then it prints the failed case IDs and the pass/fail counts. The averages are what you track over time. When you change the system prompt or swap models, you rerun and compare: did faithfulness drop from 4.9 to 4.3? That's how evals become a regression suite for prompts and models.

---

## 10. The run, and what the failure teaches

**First run:** most cases passed. The first case scored 5, 5, 5, 4, 5, with the judge's reason noting the answer was correct but could have mentioned the product-condition rules from the policy. That's a useful helpfulness note, still above threshold.

**One failure: `COMBINED-RETURN-01`** ("Can I still return ORD-102?"):

- response exists: PASS
- correct tool: **FAIL**
- correct argument: **FAIL**
- source retrieved: PASS
- helpfulness: **3/5**

Retrieval worked (return policy found), but the model **didn't call `getOrderStatus` correctly**. The user said "delivered 20 days ago," and the model likely just *trusted the user's claim* and answered from the policy alone, never verifying the order. The answer may well have read fine to a human ("Yes, 20 is within 30 days"). Only the deterministic trace exposed that it skipped verification. This failure demonstrates the entire argument of the video.

**Second run:** all 8 passed. Averages were all 5 except helpfulness at 4.88.

He concludes "it may have hallucinated a little the first time, and a better system prompt could fix it." That's partly right, but there's a bigger lesson in this result: **the same test, unchanged, passed once and failed once.** The agent is non-deterministic, so a single green run proves very little. The correct takeaway is that this case is **flaky**, which is a real finding. The fix is to make the system prompt explicitly say "always verify order status with the tool before answering return-eligibility questions, even if the customer states a delivery date," then run the case many times and measure a pass *rate*.

---

## 11. What the demo simplifies (and what a production version needs)

The video is a teaching demo, and these are the gaps worth knowing about if you build this for real.

**ToolTracker is shared mutable state.** It's a singleton Spring bean holding lists and maps, cleared at the start of each request. With two concurrent `/chat` requests, their traces mix together. That's fine in a sequential test but broken in production. Use request-scoped state, or pass a trace object through the tool context.

**The maps overwrite.** If the model calls `getOrderStatus` twice (say, for two orders in one question), the second call replaces the first in `toolArguments`. A list of `{tool, args, result}` records, in order, is the right structure. Ordering itself can be worth asserting.

**`contains` is lenient.** `toolsUsed.contains(expected)` passes even if the model also called two unnecessary tools. You may want an exact-set match, or at least an "unexpected tools" check.

**Null source means unchecked, not "nothing retrieved."** Retrieval always runs, so tool-only questions still retrieve something. That's acceptable here, but it means `expectedSource: null` silently skips the check rather than asserting anything.

**Self-grading bias.** Using the same model as both agent and judge is convenient, but LLM judges tend to favour outputs that resemble their own style. Use a different, ideally stronger, model as the judge, run it at low temperature, and spot-check its scores against your own human ratings on a sample. The judge needs to be validated too.

**One run per case.** Given the flakiness above, run each case N times and threshold the pass rate.

**Tool results are deterministic, so assert on them.** He mentions an expected-tool-result check but doesn't implement it. Since the tools are plain code, it's a free deterministic check.

**Cost and speed.** Every case means one agent call plus one judge call against a real LLM. That's why eval suites usually run on prompt/model changes or nightly, not on every commit like unit tests.

---

## 12. The mental model in one picture

```
                      Golden Dataset (question + expectations)
                                   │
                                   ▼
                     Real AI app runs (LLM + RAG + Tools)
                                   │
                                   ▼
               AgentResponse = answer + trace (recorded by YOUR code)
                     ┌─────────────┴─────────────┐
                     ▼                           ▼
          Deterministic evals              LLM-as-a-Judge
          (exact behaviour)                (semantic quality)
          response exists?                 relevance
          right tool?                      correctness
          right argument?                  faithfulness
          right source?                    helpfulness
                     │                     hallucination-free
                     │                           │ threshold ≥ 4
                     └─────────────┬─────────────┘
                                   ▼
                     PASS only if both layers pass
```

**Test exact behaviour with exact code. Use an LLM judge only where understanding meaning is genuinely needed.**

---

## 13. Bringing it to your stack

Nothing here is Spring-specific. In Go, the same structure is a table-driven test:

```go
type Trace struct {
    Answer        string
    RetrievedDocs []string
    ToolCalls     []ToolCall      // ordered slice, not a map: fixes the overwrite issue
    Context       string
}
type ToolCall struct{ Name, Arg, Result string }

func TestGoldenDataset(t *testing.T) {
    cases := loadGolden(t, "testdata/golden.json")
    for _, tc := range cases {
        t.Run(tc.ID, func(t *testing.T) {
            tr := agent.Chat(context.Background(), tc.Question) // trace carried per request, not global

            // Layer 1: deterministic
            if tc.ExpectedTool == "" && len(tr.ToolCalls) > 0 { t.Errorf("unexpected tool: %v", tr.ToolCalls) }
            if tc.ExpectedTool != "" && !called(tr, tc.ExpectedTool, tc.ExpectedArg) {
                t.Errorf("want %s(%s), got %v", tc.ExpectedTool, tc.ExpectedArg, tr.ToolCalls)
            }
            if tc.ExpectedSource != "" && !slices.Contains(tr.RetrievedDocs, tc.ExpectedSource) {
                t.Errorf("retrieval miss: want %s, got %v", tc.ExpectedSource, tr.RetrievedDocs)
            }

            // Layer 2: judge (separate model, temp 0, JSON output)
            score := judge(t, tc, tr)
            if score.Min() < 4 { t.Errorf("quality fail: %+v — %s", score, score.Reason) }
        })
    }
}
```

Passing the trace through `context.Context`, or returning it from the agent loop, avoids the concurrency issue of a global tracker.

The design also maps directly onto an insurance support assistant. Policy wording, grace periods, and claim procedures are Type A (RAG over policy documents). Live policy status, renewal premium due, and claim status by policy number are Type B tools. A question like "my policy lapsed 10 days ago, can I still renew without a fresh inspection?" is Type C. It needs the grace-period clause from the documents *and* the actual lapse date from a tool, and the system should verify the date rather than trust the customer's claim, which is exactly the failure this video caught. In a domain like insurance, you'd set correctness and faithfulness thresholds at 5, not 4.