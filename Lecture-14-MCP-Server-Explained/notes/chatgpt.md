# MCP Deep Dive — Part 2: What Happens Under the Hood?

In the first part, we established the basic architecture:

User → AI Host → LLM → MCP Client → MCP Server → Business Logic

Now we're going deeper. The next goal is to understand MCP the way a backend engineer would: actual protocol messages, how a framework connects different interfaces, how failures propagate, and how an MCP system behaves with real users.

## 25. MCP has multiple layers — and each layer solves a different problem

Imagine your user says:

> Create a task called "Study Distributed Systems."

Your application uses a Spring Boot backend, an OpenAI model, and a Task MCP Server.

There are actually three different communication contracts involved.

Layer 1 — Application API

Browser → Spring Boot

HTTP POST /ask

Layer 2 — LLM Tool Calling

Spring Boot ↔ OpenAI

Model-specific tool-call protocol

Layer 3 — MCP

MCP Client ↔ MCP Server

MCP messages using JSON-RPC

Layer 4 — Business System

MCP Server → Task Service

Java function / REST / SQL

Consider what each layer knows.

Your frontend knows about `/ask`. It doesn't need to know that MCP exists.

Your LLM knows that a callable function named `createTask` exists. It doesn't necessarily know where the MCP Server is deployed.

Your MCP Client knows which MCP Server exposes that capability and how to invoke it using the MCP protocol.

Your MCP Server knows which implementation should run when `createTask` is requested.

Your database knows nothing about LLMs or MCP. It just receives ordinary database operations.

This separation is crucial.

If tomorrow you replace the OpenAI model with another compatible LLM, your underlying MCP Server may not need to change.

If you replace PostgreSQL with another persistence system, the MCP interface may remain unchanged.

If you replace Spring Boot with a Go-based host, you can still use the same MCP Server, provided your new host has a compatible MCP Client.

That's one practical meaning of loose coupling.

## 26. Is an MCP tool call the same as an LLM function call?

No. They're different messages sent through different interfaces.

Let's compare.

### A. The model requests a function

Suppose the host has advertised:

The user asks:

> Add Study Distributed Systems to my tasks.

The LLM provider might respond with a tool-call object that conceptually contains:

This is the model's proposed action.

The model API may also include a tool-call ID, response type, and other provider-specific metadata.

The application must interpret that response.

### B. The MCP Client requests execution

The host then invokes its MCP Client.

The client constructs an MCP operation:

This is the MCP protocol's representation of an operation request.

The example shows the essential method and arguments; newer protocol versions also require additional request metadata and, over HTTP, specified headers.

The distinction matters because the model-provider tool-call format and the MCP wire format are not identical.

Some component must translate between them.

In Spring AI, that translation is largely handled by its tool callback and MCP client integrations.

### C. The server executes actual code

The MCP Server maps that call to the Java method:

Then the business operation happens.

You now have three separate concepts:

| Concept                | What it represents                               |
| ---------------------- | ------------------------------------------------ |
| LLM tool call          | A request from the model to execute a capability |
| MCP `tools/call`       | A standardized request sent to an MCP Server     |
| Java method invocation | Actual execution of the operation                |

These shouldn't be treated as interchangeable.

## 27. How the Spring AI ToolCallback bridge works

This is worth understanding beyond the instructor's `defaultTools(mcpTools)` line.

Spring AI uses a tool-callback abstraction that can represent executable functionality to its model integration.

Conceptually, an individual callback needs two important capabilities:

This is illustrative pseudocode, not the exact Spring AI interface.

The first operation supplies metadata.

The second provides execution.

Now imagine an MCP-backed implementation:

The important thing isn't the class syntax.

It's the adapter pattern.

The callback converts one interface into another.

### Why is this useful?

Suppose your MCP Server exposes 30 tools.

You shouldn't need to manually write 30 separate Spring AI tool wrappers when your MCP client integration can adapt the discovered tools.

The callback provider supplies those adapters.

This is one of the strongest connections between MCP and ordinary software design patterns.

It resembles:

- The Adapter Pattern.
- Dependency inversion.
- Interface-based programming.
- Service discovery.
- RPC client stubs.

### One nuance: tool names can conflict

Imagine you connect to two MCP Servers.

Server A exposes:

Server B also exposes:

Which one should the model call?

An MCP-aware host must resolve this ambiguity.

Spring AI's current MCP client starter supports tool-name prefix generation, filtering, and multiple client connections to address these integration concerns.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://docs.spring.io\&sz=32)

Home



A clear design would expose model-facing names such as:

This is much easier for the LLM to interpret than two ambiguous `search` functions.

## 28. What happens when you change the LLM provider?

Imagine your architecture initially uses an OpenAI model.

Now you switch to another model provider.

What changes?

Potentially:

- Model configuration.
- API credentials.
- Provider-specific tool schema representation.
- Tool-calling behavior and supported features.

What can stay the same?

- Task MCP Server.
- `createTask` business logic.
- Tool input contracts, assuming compatibility.
- Underlying REST APIs.
- Database schema.

MCP abstracts how the host accesses the capability.

It does not eliminate differences between LLM providers.

Some models may support certain schema features or parallel tool calls differently from others.

So changing the LLM provider does not guarantee identical model behavior, but you generally don't have to rewrite the MCP Server solely because you changed models.

# Part XII — The actual MCP protocol in 2026

## 29. An important change since older MCP tutorials

The lecture focuses on `tools/list`, `tools/call`, and Streamable HTTP.

Those remain useful concepts.

However, MCP has evolved considerably.

The official July 28, 2026 MCP specification introduced a stateless protocol core and changed how connections are established.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://blog.modelcontextprotocol.io\&sz=32)

Model Context Protocol Blog



This is important because tutorials, SDKs, and integrations may target different protocol revisions.

### Older protocol lifecycle

In the 2025-era protocol, a typical client-server interaction started with an initialization exchange.

The exchange allowed the client and server to agree on protocol information and advertise capabilities.

With stateful HTTP transport, some implementations maintained protocol-level sessions using session identifiers.

### Newer 2026-07-28 protocol

The newer specification removes the mandatory initialization handshake and protocol-level session.

Requests can be self-describing.

The client can optionally use:

to obtain server capability information.

Importantly:

and

are not the same operation.

`server/discover` concerns server-level capabilities.

`tools/list` retrieves the specific tools the server exposes.

### Why did this change?

Consider a remote MCP Server deployed across multiple instances.

If every MCP connection requires sticky session state, the load balancer may need to route related requests to the same instance or use shared session infrastructure.

A stateless protocol core makes ordinary horizontal scaling easier.

Each request can contain the required protocol metadata.

The server can process it without depending on a transport session created by a previous request.

Stateless protocol communication doesn't mean your application cannot store persistent data.

Your Task MCP Server can still use PostgreSQL and maintain millions of tasks.

It simply needn't rely on an implicit MCP transport session to identify the state of a particular workflow.

### What does a current HTTP request look like?

The official specification illustrates headers such as:

The message body contains the JSON-RPC request and its associated metadata.

Why put the method and name in HTTP headers?

An API gateway can identify the requested operation without parsing the entire JSON body.

This helps implement routing, monitoring, and policy enforcement.

Don't manually mix protocol formats from different versions. If your Spring AI MCP libraries use an older negotiated protocol version, follow the SDK's supported lifecycle and transport behavior instead.

The official release announcement and migration documentation describe the newer request format.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://blog.modelcontextprotocol.io\&sz=32)

Model Context Protocol Blog

+1



## 30. What does a complete MCP message look like?

Let's examine a realistic exchange using the July 2026 protocol revision.

Our MCP Client wants to execute:

### Request

Let's break down the request.

| Field       | Meaning                                  |
| ----------- | ---------------------------------------- |
| `jsonrpc`   | The RPC message follows JSON-RPC 2.0     |
| `id`        | Correlates the request with its response |
| `method`    | Specifies which MCP operation to perform |
| `name`      | Identifies the exposed MCP tool          |
| `arguments` | Contains the tool's input values         |
| `_meta`     | Supplies protocol and client metadata    |

A common beginner mistake is to confuse the JSON-RPC `id` with an application's task ID.

They are not the same.

The value `42` here identifies the protocol request. It doesn't identify the task being created.

The actual task identifier would normally be generated by the underlying task service.

### Server execution

The MCP Server receives the message.

It checks that `createTask` is a registered tool, validates the arguments, applies authorization, and invokes the implementation.

Conceptually:

Suppose PostgreSQL stores:

### Response

A server supporting structured tool results might return:

There are some very important details here.

### A. The request and response have the same JSON-RPC ID

That allows the MCP Client to associate the response with its outstanding request.

This is especially useful when multiple requests are in flight.

### B. content and structuredContent serve different purposes

The `content` array can contain content blocks suitable for downstream presentation or model context.

`structuredContent` provides machine-readable result data.

The official MCP specification supports both and allows tools to declare output schemas for validating structured results.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://github.com\&sz=32)

GitHub



This is extremely useful in backend systems.

Suppose another service needs the newly created task's ID.

Parsing this sentence would be fragile:

Reading the structured field is much better:

### C. The resultType field

The 2026-07-28 specification uses `resultType` to distinguish result kinds.

For an ordinary completed operation:

Other supported workflows can require additional input or return an asynchronous task handle through an extension.

We'll discuss these shortly.

## 31. Input Schema versus Output Schema versus LLM Structured Output

This is one of the most useful connections to your previous structured-output lecture.

There are three different contracts to understand.

Three contracts in a tool-enabled application

1\. Tool Input Schema

What arguments must be supplied to execute the operation?

2\. Tool Output Schema

What result structure should the MCP Server return?

3\. LLM Structured Output

What final structured response should the model generate for the application?

All three use structured data, but they sit at different boundaries.

The first validates inputs to the tool.

The second describes outputs from the tool.

The third constrains a model-generated response.

### Why are output schemas valuable?

Imagine you define a tool:

Without an output schema, the result may simply be:

But with a defined output schema, your application could receive:

Now another tool could consume the `orderId` and `status` deterministically.

For example:

This can reduce the need to repeatedly ask an LLM to extract fields from natural-language responses.

An LLM is useful for understanding ambiguous intentions. Ordinary typed code is often preferable for deterministic transformations and business decisions.

That principle becomes increasingly important as AI workflows grow.

## 32. How MCP errors work

Suppose the model requests:

There are several ways this can fail, and they should not all be represented identically.

### Case A: The task doesn't exist

The tool exists and executes successfully as a software operation, but the business outcome is unsuccessful.

The MCP Server might return:

This is an application or tool-level error.

The model can receive this information and explain:

> I couldn't find a task with that title.

Or it could decide to use `listTasks` to investigate, provided the workflow permits it.

### Case B: The requested tool does not exist

Suppose the MCP Client requests:

But there is no such registered tool.

This is a protocol-level or request-routing error.

An appropriate JSON-RPC error response is different from an ordinary tool result.

Conceptually:

The exact error code and details should follow the relevant MCP implementation and protocol version.

### Case C: The tool times out

Imagine `createTask` calls a remote backend.

The MCP Server might not know whether the request was committed before the connection timed out.

This is a more difficult failure.

Suppose the database actually created task 107, but the acknowledgement was lost.

If the host retries blindly, the system might create a duplicate task.

That is why production systems need idempotency.

For example:

The backend associates that key with the logical operation and ensures repeated attempts follow its idempotency contract.

MCP doesn't automatically provide exactly-once execution of business operations.

The distinction between errors originating inside a tool and exceptional protocol errors is also defined in the MCP specification.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://github.com\&sz=32)

GitHub



## 33. Dynamic discovery is powerful, but caching makes it more complicated

The lecture shows this sequence:

Now imagine you have a large organization.

A server exposes 100 tools, and 20 AI applications connect to it.

If all of them frequently rediscover the complete tool catalogue, that's unnecessary work.

A host may cache tool definitions.

### Example

At 10:00 AM, discovery returns:

At 10:05 AM, the MCP Server deploys:

Should the host immediately offer `deleteTask`?

Not automatically.

It depends on when and how the host refreshes its catalogue.

In the July 2026 MCP specification, discovery results can include cache hints:

Here:

`ttlMs` means the response can be treated as fresh for the indicated duration, in milliseconds.

`cacheScope` describes whether the cached result is suitable for sharing across authorization contexts.

### Why does cacheScope matter?

Imagine an admin user and a regular user.

The admin has access to:

A regular user may have access to only:

Suppose the server accidentally treats the admin tool catalogue as globally cacheable.

A shared cache might expose tool metadata intended only for the admin.

That would be a security design problem.

The `"private"` cache scope helps prevent this type of cross-context cache sharing when respected by clients and intermediaries.

But authorization must still be enforced during execution, regardless of which tools were advertised.

The current specification includes `ttlMs` and `cacheScope` for eligible discovery and read results.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://py.sdk.modelcontextprotocol.io\&sz=32)

MCP Python SDK



### A related subtlety: tool discovery can be paginated

Some MCP Servers may expose a sufficiently large catalogue that returning all tools in one response is undesirable.

A `tools/list` response can contain a continuation cursor:

This is a shortened illustration.

The client can fetch subsequent pages using the cursor.

For large tool catalogues, a correct client needs to account for pagination instead of assuming the first response contains every available tool.

# Part XIII — Advanced MCP interactions

## 34. What if an MCP tool needs additional information from the user?

Consider a different task.

The user says:

> Schedule a meeting with Rahul.

Your calendar tool needs:

But the user hasn't provided the start time.

What should happen?

A poorly designed agent might invent a time.

A better system asks:

> At what time should I schedule the meeting?

For many simple agents, the host and LLM can manage this by having another conversation turn before invoking the tool.

But the MCP protocol also supports more sophisticated interaction patterns.

### Multi Round-Trip Requests (MRTR)

The July 2026 MCP specification introduced Multi Round-Trip Requests as part of its stateless architecture.

An operation can indicate that additional input is needed.

Conceptually:

An illustrative response is:

This is a simplified example of the concept rather than a complete protocol-valid elicitation object.

The important distinction is that the tool operation has not finished. It needs further input.

The host is responsible for handling that interaction appropriately, and an application may also require explicit human confirmation before a consequential operation.

This feature wasn't implemented in the lecture's simple task manager. It's an extension of the architecture you now understand.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://blog.modelcontextprotocol.io\&sz=32)

Model Context Protocol Blog



## 35. An MCP Task is different from the task in the lecture

This is an interesting naming coincidence.

The lecturer builds a task management application with:

Those represent the user's to-do items.

But modern MCP also has a Tasks extension for managing long-running protocol operations.

These are completely different concepts.

### Imagine an operation that takes 20 minutes

Suppose you expose a tool:

Internally, this might:

- Query several databases.
- Process millions of records.
- Generate charts.
- Create a report.
- Upload the result to storage.

This cannot always finish within a short synchronous HTTP request.

A synchronous call can run into client timeouts or network interruptions.

The MCP Tasks extension provides a way for supported clients and servers to represent longer-running work.

Conceptually:

The extension defines methods such as:

The exact supported operations and task lifecycle depend on the extension implementation.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://tasks.extensions.modelcontextprotocol.io\&sz=32)

MCP Tasks Extension



This is useful for AI infrastructure that performs lengthy operations such as generating reports, processing documents, or building large indexes.

Remember:

The lecture's task tool is ordinary business logic. MCP Tasks are protocol-level machinery for handling long-running operations.

# Part XIV — Designing MCP properly for real users

## 36. The most important thing missing from the demo: multi-user isolation

Let's look at the original implementation from the lecture:

Then:

This works as a demonstration.

But imagine two users are connected.

User A says:

> Create a task called "Prepare for interviews."

User B says:

> Show my tasks.

If the same global list is shared across requests, User B could see User A's task.

This is not an MCP bug.

It's an application design bug.

### A production data model

A better task representation is:

The task belongs to an authenticated user.

The persistence layer might contain:

| id  | user_id | title                  | status  |
| --- | ------- | ---------------------- | ------- |
| 101 | user_A  | Prepare for interviews | PENDING |
| 102 | user_B  | Buy groceries          | PENDING |

When User B requests tasks, the service should filter by User B's trusted identity.

For example:

The parameter should come from the application's trusted authentication context, not simply from an LLM-generated `userId`.

### Why is that distinction so important?

Imagine the MCP tool accepts:

An ordinary user might influence the LLM into supplying another user's ID.

A safer design is:

The underlying service combines the task title with the trusted user identity.

In a distributed deployment, the MCP Server needs a secure way to establish the caller's identity, such as validated credentials or a properly designed delegated-authorization mechanism.

Merely sending a user ID in an untrusted field is insufficient.

This is one of the distinctions between building a demonstration and building an enterprise AI application.

## 37. Should MCP Servers contain business logic or just adapters?

There are two reasonable architectural choices.

### Architecture A — MCP Server owns the operation

This is suitable when the MCP Server itself is your service boundary and owns the relevant business logic.

### Architecture B — MCP Server adapts an existing service

This works well if your organization already has a Task Service used by web and mobile applications.

Why might Architecture B be preferable?

Because you avoid creating two separate implementations of the same business rules.

For example, a task creation rule might state:

> A user cannot have more than 1,000 active tasks.

You shouldn't implement that rule independently in the MCP Server and the ordinary REST backend.

It belongs in an authoritative business layer shared by the supported interfaces.

Otherwise the MCP integration could bypass rules that your normal application enforces.

### What belongs in the MCP layer?

The MCP-facing component should be concerned with exposing and describing capabilities, handling protocol requests, validating inputs, establishing authorization, and adapting operations to the underlying business system.

The ordinary domain layer should remain responsible for the business invariants.

This is exactly where Clean Architecture and Hexagonal Architecture concepts become useful.

MCP can be treated as another inbound adapter to your domain services.

# Part XV — The same MCP Server can be written in Go

## 38. Building a minimal MCP tool using Go

The lecture uses Spring AI because the instructor is teaching Java.

But MCP is language-independent.

You can build the capability provider in Go and connect to it from a Java, Python, or TypeScript MCP-compatible host.

Here's a minimal Go version that illustrates the same idea.

This example uses the official Go MCP SDK and the STDIO transport. It maintains one in-memory task collection.

Illustrative learning example — not production persistence or authorization code.

### How this works

First, we create an MCP Server:

This constructs