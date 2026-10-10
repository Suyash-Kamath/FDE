# Structured Outputs in Generative AI — Complete Deep-Dive Notes

Based on the 40-minute lecture transcript, the 16-page PDF, and the Excalidraw diagrams

The central idea of this lecture is much more important than simply making an LLM return JSON.

It is about turning an LLM from a system that generates human-readable answers into a component that produces predictable data for your backend applications.

The instructor demonstrates this using Java, Spring Boot, Spring AI, and a meeting-scheduling example.

We will work through the same progression as the lecture, from the fundamentals of client-server communication to the final `MeetingDetails` object. Then we'll explore the engineering details behind schema enforcement, serialization, validation, error handling, and integration with tools and MCP.

## 1. What are we actually trying to solve?

Imagine you are building an ordinary chatbot.

A user sends:

> Explain what Docker is.

Your Spring AI application calls an LLM:

```
String response = chatClient.prompt()
    .user("Explain what Docker is")
    .call()
    .content();
```

And receives something like:

```
Docker is a containerization platform that
allows developers to package applications
and their dependencies into containers.
```

This is perfectly useful when a human is going to read the response.

But now suppose the user says:

> Schedule a Docker discussion with Rahul tomorrow at 4 PM for 30 minutes.

Your backend has a different problem.

It doesn't need the LLM to explain the meeting in English. It needs to know:

| Field    | Value                                |
| -------- | ------------------------------------ |
| Title    | Docker Discussion                    |
| Attendee | Rahul                                |
| Date     | Tomorrow, resolved to an actual date |
| Time     | 16:00                                |
| Duration | 30 minutes                           |

The application needs these values separately because it may eventually store them in PostgreSQL, verify availability, calculate the end time, or create a Google Calendar event.

A natural-language sentence contains the information, but the backend still needs a reliable way to extract and represent it.

Structured output solves this representation problem.

Traditional LLM interaction

Human prompt

LLM

Natural-language answer

Designed primarily for human interpretation

Structured-output interaction

Natural-language request

LLM + expected schema

Structured `MeetingDetails`

Java business logic

Designed for programmatic consumption

Notice the important difference: the LLM isn't necessarily doing the business operation. It is preparing information that makes the operation possible.

This is precisely why the instructor introduces structured outputs before MCP.

## 2. Why is natural language insufficient for an ordinary backend?

Lecture: 00:01:00–00:05:15

The instructor begins by comparing two communication patterns.

### Case A: Human communicating with an LLM

The flow in your Excalidraw diagram is:

```
Client
   |
   | "Hi, my name is Aditya"
   v
Application Server
   |
   v
LLM
   |
   | "Hello Aditya, I am an LLM"
   v
Application Server
   |
   v
Client
```

Your server is primarily forwarding the user's text to an LLM and returning the generated answer.

The output may have different phrasing every time. For conversational applications, that is often acceptable.

### Case B: One software application communicating with another

The instructor then introduces a normal REST API.

For example:

```
GET /user/details
```

A server might return:

```
{
  "name": "Aditya",
  "email": "aditya@example.com",
  "mobile": "9876543210"
}
```

The frontend can now access individual properties:

```
const user = await response.json();

console.log(user.name);
console.log(user.email);
console.log(user.mobile);
```

Why is this easier than returning:

```
The user's name is Aditya.
Their email address is aditya@example.com.
Their mobile number is 9876543210.
```

Because the JSON creates an agreed-upon structure.

In the first case:

```
user.email
```

gives the email.

In the second case, your application needs additional logic to discover the email inside arbitrary text.

A regular expression might work for some examples. But real natural language creates difficult edge cases: different word orders, missing details, multiple people, ambiguous dates, and corrections.

One important technical clarification: conventional software can process natural language through parsers, rules, regex, and other NLP techniques. The instructor's point is that ordinary backend code does not automatically understand arbitrary human instructions. An LLM provides a flexible mechanism for interpreting them.

The deeper distinction is:

Natural language provides semantic flexibility. A schema provides structural predictability.

## 3. The meeting-scheduler example: why introduce an LLM at all?

Lecture: 00:05:18–00:13:42

Before introducing an LLM, the instructor asks us to imagine building a traditional meeting scheduler.

The frontend contains a form where users manually enter a title, people attending, date, time, and other details.

When the user submits it, the frontend sends a JSON request to the backend.

```
{
  "title": "Docker Discussion",
  "attendees": ["Rahul"],
  "date": "2026-10-12",
  "time": "16:00",
  "durationMinutes": 30
}
```

The backend can perform ordinary business logic:

1. Validate the request.
2. Check whether the meeting time is valid.
3. Store the meeting in a database.
4. Create a calendar event.
5. Publish a notification event or schedule a reminder.

This architecture works without artificial intelligence.

But a natural-language interface lets the user skip manually filling in the form.

They simply write:

> Schedule a Docker discussion with Rahul on October 12 at 4 PM for 30 minutes.

Your backend sends that sentence, together with the expected output schema, to an LLM.

The model extracts:

```
{
  "title": "Docker Discussion",
  "attendees": ["Rahul"],
  "date": "2026-10-12",
  "time": "16:00",
  "durationMinutes": 30
}
```

Now the ordinary backend can continue working with well-defined data.

The important architectural boundary is:

LLM = natural-language interpretation.

Backend = validation, authorization, persistence, integrations, and business execution.

The instructor's actual demonstration stops after returning the extracted JSON. It does not create a Google Calendar event. That distinction matters throughout the lecture.

## 4. What exactly is a schema?

Lecture: 00:17:03–00:24:19 | PDF: pages 7–8

This is one of the most important concepts in the entire lecture.

A schema is a formal description of the structure and constraints that data must follow.

It can define:

- Which fields must exist.
- What names those fields must have.
- Which data types are allowed.
- Which fields are required or optional.
- Which values or ranges are permitted.
- How nested objects and arrays are organized.

### 4.1 Understanding the instructor's paper-and-form analogy

The instructor gives a simple example.

Imagine you ask 100 people:

> Write anything about yourself on a blank sheet of paper.

Everyone will answer differently.

One person might begin with their education. Another might write their work history, followed by their name.

Extracting the same information from all 100 responses becomes difficult because their structures are different.

Now imagine handing out a form.

### Personal Information

Name

Rahul

Age

28

Email

rahul\@example.com

Skills

Java, Spring Boot, Docker

Illustrative form based on the lecture's analogy.

Everyone now provides information through the same predefined fields.

This is what a schema does for software.

The application decides what information it needs. The LLM supplies values that fit that definition.

### 4.2 JSON is not the same as a schema

These two JSON objects are both syntactically valid:

```
{
  "meetingTime": "4 PM",
  "person": "Rahul"
}
```

```
{
  "scheduled_at": "16:00",
  "attendees": ["Rahul"],
  "duration": 30
}
```

But they are not interchangeable.

Suppose your Java application expects:

```
meeting.getScheduledTime();
meeting.getAttendees();
```

The first object has a property named `meetingTime`, while the second uses `scheduled_at`.

Neither necessarily maps directly to the expected Java property `scheduledTime`.

A JSON parser can read both objects, but that does not mean either satisfies the application's contract.

This distinction is essential:

- Valid JSON: The text obeys JSON syntax.
- Schema-valid JSON: The JSON obeys the expected structural rules.
- Semantically correct JSON: The values correctly represent the user's actual request.
- Business-valid data: The information also satisfies your application's rules.

These are four different properties.

### 4.3 A concrete JSON Schema

The PDF introduces a `MeetingRequest` with a title, attendee, date, time, duration, and meeting type.

Let's express a simplified version using JSON Schema:

```
{
  "type": "object",
  "properties": {
    "title": {
      "type": "string"
    },
    "attendee": {
      "type": "string"
    },
    "date": {
      "type": "string",
      "format": "date"
    },
    "time": {
      "type": "string",
      "pattern": "^([01][0-9]|2[0-3]):[0-5][0-9]$"
    },
    "durationMinutes": {
      "type": "integer",
      "minimum": 1,
      "maximum": 480
    },
    "type": {
      "type": "string",
      "enum": [
        "ONE_ON_ONE",
        "TEAM",
        "INTERVIEW"
      ]
    }
  },
  "required": [
    "title",
    "attendee",
    "date",
    "time",
    "durationMinutes",
    "type"
  ],
  "additionalProperties": false
}
```

Let's understand what this specifies.

| Schema property               | Meaning                                |
| ----------------------------- | -------------------------------------- |
| `type: "object"`              | The root value must be a JSON object   |
| `properties`                  | Lists permitted fields                 |
| `type: "string"`              | Expects a string value                 |
| `type: "integer"`             | Expects an integer, not arbitrary text |
| `required`                    | Specifies which keys must exist        |
| `enum`                        | Restricts values to a defined set      |
| `minimum` / `maximum`         | Restricts numeric values               |
| `pattern`                     | Defines a textual pattern              |
| `additionalProperties: false` | Disallows unexpected keys              |

This is far more precise than saying:

> Please respond in JSON.

An important implementation nuance: `format: "date"` and other advanced JSON Schema keywords are not uniformly enforced by every validator or LLM provider. Your backend should still independently parse and validate the actual date.

### 4.4 Data types are part of the contract

Consider:

```
{
  "durationMinutes": "30"
}
```

versus:

```
{
  "durationMinutes": 30
}
```

The first contains a string. The second contains a number.

If your backend expects an integer, the second is the correct representation.

Similarly:

```
{
  "attendees": ["Rahul", "Priya"]
}
```

is an array of strings, whereas:

```
{
  "attendees": "Rahul, Priya"
}
```

is a single string.

These differences become important when deserializing the model's output into Java classes, Python models, or Go structs.

## 5. How does structured output map into a Java object?

Lecture: 00:20:44–00:23:47 and 00:28:49–00:35:38

The instructor introduces the concept of a response DTO.

DTO stands for Data Transfer Object.

Its job is to represent data exchanged between parts of a system, such as between the LLM integration and the application's service layer, or between the backend and its HTTP client.

### 5.1 Using a Java record

We can represent the meeting as:

```
public record MeetingRequest(
    String title,
    String attendee,
    String date,
    String time,
    Integer durationMinutes,
    MeetingType type
) {}
```

And define the enum:

```
public enum MeetingType {
    ONE_ON_ONE,
    TEAM,
    INTERVIEW
}
```

This closely follows the PDF notes. The video demonstration uses a `MeetingDetails` class with getters and setters instead. The fundamental idea is identical, although the PDF and lecture use slightly different field names and types.

Suppose the user says:

> Schedule a Docker discussion with Rahul on October 12 at 4 PM for 30 minutes.

The LLM produces structured data corresponding to:

```
{
  "title": "Docker Discussion",
  "attendee": "Rahul",
  "date": "2026-10-12",
  "time": "16:00",
  "durationMinutes": 30,
  "type": "ONE_ON_ONE"
}
```

A JSON deserializer such as Jackson can convert this to a Java record.

Conceptually, the process is:

Raw JSON response

Keys and values produced by the LLM

JSON deserialization

Matches JSON keys to Java fields and converts types

`MeetingRequest` object

Typed representation available to Java code

Now you can access the values:

```
MeetingRequest meeting = ...;

String attendee = meeting.attendee();

String date = meeting.date();

int duration = meeting.durationMinutes();
```

For example:

```
if (meeting.durationMinutes() > 60) {
    System.out.println("Manager approval required");
}
```

Or:

```
if (meeting.type() == MeetingType.INTERVIEW) {
    System.out.println("Interview preparation required");
}
```

Your application can use the result just like any other Java object.

### 5.2 What happens behind the scenes?

Let's assume Spring AI obtains the following response from the model:

```
{
  "title": "Docker Discussion",
  "attendee": "Rahul",
  "date": "2026-10-12",
  "time": "16:00",
  "durationMinutes": 30,
  "type": "ONE_ON_ONE"
}
```

A simplified mental model of deserialization is:

```
MeetingRequest meeting = new MeetingRequest(
    "Docker Discussion",
    "Rahul",
    "2026-10-12",
    "16:00",
    30,
    MeetingType.ONE_ON_ONE
);
```

This is conceptually what the mapping achieves; it is not the literal internal code generated by Spring AI.

One critical point: creating a Java object is not the same as creating an event or saving anything in a database.

At this stage, the application simply possesses a `MeetingRequest` instance.

### 5.3 What if the JSON property does not match the Java property?

Suppose the model returns:

```
{
  "duration": 30
}
```

But the Java class expects:

```
Integer durationMinutes
```

The expected property is missing.

Depending on how the deserializer is configured, this could cause a conversion error or result in a missing/null value.

This is precisely the kind of inconsistency that a predefined output schema is designed to reduce.

## 6. The most important Spring AI code: `.content()` versus `.entity()`

Lecture: 00:33:26–00:35:38

This is the central coding moment in the entire video.

### Ordinary text generation

```
String response = chatClient.prompt()
    .system("You are a helpful assistant.")
    .user("Explain Docker.")
    .call()
    .content();
```

Here:

- `prompt()` starts constructing an LLM request.
- `system()` provides behavioral instructions.
- `user()` provides the user's message.
- `call()` selects the synchronous call path.
- `content()` executes the request and returns the model's text content.

The resulting Java type is `String`.

### Structured output generation

```
MeetingDetails details = chatClient.prompt()
    .system(systemPrompt)
    .user(userMessage)
    .call()
    .entity(MeetingDetails.class);
```

Now the returned type is `MeetingDetails`.

The application is no longer asking Spring AI to hand back arbitrary textual content. It is asking Spring AI to obtain and convert a response into the expected Java type.

The difference is:

| Method                                         | Result                                         |
| ---------------------------------------------- | ---------------------------------------------- |
| `.call().content()`                            | Raw text as `String`                           |
| `.call().entity(MeetingDetails.class)`         | Deserialized Java `MeetingDetails`             |
| `.call().responseEntity(MeetingDetails.class)` | Parsed object and underlying response metadata |

### 6.1 Does `.entity()` merely convert JSON after the LLM responds?

This is an excellent point to understand deeply.

In current Spring AI, the default typed-output flow involves three conceptual operations:

1. Spring AI derives an output schema from the Java type.
2. It supplies schema-oriented formatting instructions to the model.
3. It parses the model's response into the Java type.

So `.entity(MeetingDetails.class)` is more than a normal Jackson `readValue()` call applied afterward.

However, there is a very important distinction:

Using `.entity()` by itself does not guarantee that the provider will enforce the schema.

The default approach relies on the model following schema instructions, and parsing can still fail if the output is malformed.

Spring AI's current reference documentation explicitly distinguishes ordinary conversion, schema validation, and provider-native constrained output.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://docs.spring.io\&sz=32)

Home

+1



### 6.2 Three levels of reliability

Level 1 — Prompt-only formatting

You instruct the model to respond in a particular JSON shape. The model attempts to comply, but nothing independently guarantees valid output.

Level 2 — Schema validation and retries

The output is checked against a schema. If invalid, the application can retry with feedback or reject the response.

Level 3 — Provider-native structured output

A compatible provider accepts schema constraints through its API and restricts the output format at generation time. This offers stronger structural guarantees, subject to provider capabilities and failure cases.

Current Spring AI documentation shows these options for compatible versions:

```
MeetingDetails details = chatClient.prompt()
    .system(systemPrompt)
    .user(userMessage)
    .call()
    .entity(MeetingDetails.class, spec -> spec
        .useProviderStructuredOutput()
        .validateSchema());
```

`useProviderStructuredOutput()` requests provider-level schema enforcement, while `validateSchema()` enables validation and a self-correcting retry mechanism. The native option requires model/provider support.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://docs.spring.io\&sz=32)

Home

+1



This is an extension beyond the instructor's simpler code. It is useful when moving from a demonstration toward a production system.

But even the strongest structural enforcement cannot prove that the extracted information is factually correct.

For example, this is perfectly well-structured JSON:

```
{
  "title": "Docker Discussion",
  "attendee": "Rahul",
  "date": "2026-10-12",
  "time": "16:00",
  "durationMinutes": 30,
  "type": "ONE_ON_ONE"
}
```

If the user actually requested 5 PM, the response is still wrong.

A schema validates the representation, not the truth of every extracted value.

## 7. The complete Spring Boot implementation explained

Lecture: 00:28:49–00:37:24

Let's reconstruct the implementation from the video.

The instructor uses three important components:

- `MeetingController` — receives the user's HTTP request.
- `MeetingService` — talks to the LLM and handles the extraction.
- `MeetingDetails` — represents the expected structured result.

Here is the overall architecture.

\#chatgpt-mermaid-\_r_o8\_{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;fill:rgb(237, 237, 237);}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#chatgpt-mermaid-\_r_o8\_ .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_o8\_ .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_o8\_ .error-icon{fill:rgb(48, 48, 48);}#chatgpt-mermaid-\_r_o8\_ .error-text{fill:rgb(237, 237, 237);stroke:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_o8\_ .edge-thickness-normal{stroke-width:1px;}#chatgpt-mermaid-\_r_o8\_ .edge-thickness-thick{stroke-width:3.5px;}#chatgpt-mermaid-\_r_o8\_ .edge-pattern-solid{stroke-dasharray:0;}#chatgpt-mermaid-\_r_o8\_ .edge-thickness-invisible{stroke-width:0;fill:none;}#chatgpt-mermaid-\_r_o8\_ .edge-pattern-dashed{stroke-dasharray:3;}#chatgpt-mermaid-\_r_o8\_ .edge-pattern-dotted{stroke-dasharray:2;}#chatgpt-mermaid-\_r_o8\_ .marker{fill:rgb(175, 175, 175);stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_o8\_ .marker.cross{stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_o8\_ svg{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;}#chatgpt-mermaid-\_r_o8\_ p{margin:0;}#chatgpt-mermaid-\_r_o8\_ .label{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_o8\_ .cluster-label text{fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_o8\_ .cluster-label span{color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_o8\_ .cluster-label span p{background-color:transparent;}#chatgpt-mermaid-\_r_o8\_ .label text,#chatgpt-mermaid-\_r_o8\_ span{fill:rgb(237, 237, 237);color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_o8\_ .node rect,#chatgpt-mermaid-\_r_o8\_ .node circle,#chatgpt-mermaid-\_r_o8\_ .node ellipse,#chatgpt-mermaid-\_r_o8\_ .node polygon,#chatgpt-mermaid-\_r_o8\_ .node path{fill:rgb(9, 23, 44);stroke:rgb(31, 78, 148);stroke-width:1px;}#chatgpt-mermaid-\_r_o8\_ .rough-node .label text,#chatgpt-mermaid-\_r_o8\_ .node .label text,#chatgpt-mermaid-\_r_o8\_ .image-shape .label,#chatgpt-mermaid-\_r_o8\_ .icon-shape .label{text-anchor:middle;}#chatgpt-mermaid-\_r_o8\_ .node .katex path{fill:#000;stroke:#000;stroke-width:1px;}#chatgpt-mermaid-\_r_o8\_ .rough-node .label,#chatgpt-mermaid-\_r_o8\_ .node .label,#chatgpt-mermaid-\_r_o8\_ .image-shape .label,#chatgpt-mermaid-\_r_o8\_ .icon-shape .label{text-align:center;}#chatgpt-mermaid-\_r_o8\_ .node.clickable{cursor:pointer;}#chatgpt-mermaid-\_r_o8\_ .root .anchor path{fill:rgb(175, 175, 175)!important;stroke-width:0;stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_o8\_ .arrowheadPath{fill:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_o8\_ .edgePath .path{stroke:rgb(175, 175, 175);stroke-width:1px;}#chatgpt-mermaid-\_r_o8\_ .flowchart-link{stroke:rgb(175, 175, 175);fill:none;}#chatgpt-mermaid-\_r_o8\_ .edgeLabel{background-color:rgb(0, 0, 0);text-align:center;}#chatgpt-mermaid-\_r_o8\_ .edgeLabel p{background-color:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_o8\_ .edgeLabel rect{opacity:0.5;background-color:rgb(0, 0, 0);fill:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_o8\_ .labelBkg{background-color:rgba(0, 0, 0, 0.5);}#chatgpt-mermaid-\_r_o8\_ .cluster rect{fill:rgb(48, 48, 48);stroke:rgba(255, 255, 255, 0.15);stroke-width:1px;}#chatgpt-mermaid-\_r_o8\_ .cluster text{fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_o8\_ .cluster span{color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_o8\_ div.mermaidTooltip{position:absolute;text-align:center;max-width:200px;padding:2px;font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:12px;background:rgb(48, 48, 48);border:1px solid rgba(255, 255, 255, 0.15);border-radius:2px;pointer-events:none;z-index:100;}#chatgpt-mermaid-\_r_o8\_ .flowchartTitleText{text-anchor:middle;font-size:18px;fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_o8\_ rect.text{fill:none;stroke-width:0;}#chatgpt-mermaid-\_r_o8\_ .icon-shape,#chatgpt-mermaid-\_r_o8\_ .image-shape{background-color:rgb(0, 0, 0);text-align:center;}#chatgpt-mermaid-\_r_o8\_ .icon-shape p,#chatgpt-mermaid-\_r_o8\_ .image-shape p{background-color:rgb(0, 0, 0);padding:2px;}#chatgpt-mermaid-\_r_o8\_ .icon-shape .label rect,#chatgpt-mermaid-\_r_o8\_ .image-shape .label rect{opacity:0.5;background-color:rgb(0, 0, 0);fill:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_o8\_ .label-icon{display:inline-block;height:1em;overflow:visible;vertical-align:-0.125em;}#chatgpt-mermaid-\_r_o8\_ .node .label-icon path{fill:currentColor;stroke:revert;stroke-width:revert;}#chatgpt-mermaid-\_r_o8\_ .node .neo-node{stroke:rgb(31, 78, 148);}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].node rect,#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].cluster rect,#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].node polygon{stroke:url(#chatgpt-mermaid-\_r_o8\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].swimlane.cluster rect{filter:none;}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].node path{stroke:url(#chatgpt-mermaid-\_r_o8\_-gradient);stroke-width:1px;}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].node .outer-path{filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].node .neo-line path{stroke:rgb(31, 78, 148);filter:none;}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].node circle{stroke:url(#chatgpt-mermaid-\_r_o8\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].node circle .state-start{fill:#000000;}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].icon-shape .icon{fill:url(#chatgpt-mermaid-\_r_o8\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_o8\_ [data-look="neo"].icon-shape .icon-neo path{stroke:url(#chatgpt-mermaid-\_r_o8\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_o8\_ .node text{font-size:14px;font-weight:600;letter-spacing:normal;fill:rgb(153, 206, 255);}#chatgpt-mermaid-\_r_o8\_ .edgeLabels text{font-size:13px;font-weight:600;letter-spacing:-0.08px;fill:rgb(153, 206, 255);}#chatgpt-mermaid-\_r_o8\_ .node tspan[font-weight="normal"],#chatgpt-mermaid-\_r_o8\_ .edgeLabels tspan[font-weight="normal"]{font-weight:600;}#chatgpt-mermaid-\_r_o8\_ .edgeLabel .label rect{opacity:1;rx:13px;ry:13px;fill:rgb(0, 14, 26);stroke:rgb(26, 62, 95);stroke-width:1px;}#chatgpt-mermaid-\_r_o8\_ .node rect,#chatgpt-mermaid-\_r_o8\_ .node circle,#chatgpt-mermaid-\_r_o8\_ .node ellipse,#chatgpt-mermaid-\_r_o8\_ .node polygon,#chatgpt-mermaid-\_r_o8\_ .node path{fill:rgb(0, 40, 77);stroke:rgba(255, 255, 255, 0.1);stroke-width:1px;}#chatgpt-mermaid-\_r_o8\_ .node rect{rx:16px;ry:16px;}#chatgpt-mermaid-\_r_o8\_ .node.mermaid-decision .label-container{fill:rgb(0, 14, 26);stroke:rgb(26, 62, 95);stroke-dasharray:2,2;}#chatgpt-mermaid-\_r_o8\_ .edgePaths .flowchart-link{stroke:rgb(175, 175, 175);stroke-width:1px;stroke-linecap:round;stroke-linejoin:round;}#chatgpt-mermaid-\_r_o8\_ .marker{fill:rgb(175, 175, 175);stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_o8\_ :root{--mermaid-font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";}#chatgpt-mermaid-\_r_o8\_ .emph rect{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_o8\_ .emph polygon{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_o8\_ .emph ellipse{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_o8\_ .emph circle{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_o8\_ .emph path{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_o8\_ .emph tspan{fill:rgb(23, 52, 93)!important;}User / PostmanMeetingControllerMeetingServiceLLMSpring AI entity converterMeetingDetails Java objectMeetingControllerPOST /api/scheduleSystem instructions + usermessage + output schemaStructured responseJSON HTTP response

Notice the absence of a database, Kafka producer, or calendar API.

This architecture only extracts and returns meeting information, exactly as the instructor demonstrates.

### Step 1: Define MeetingDetails.java

```
package com.example.meetings;

import java.util.List;

public record MeetingDetails(
    String title,
    List<String> attendees,
    String date,
    String time,
    Integer durationMinutes
) {}
```

Why have we created this record?

Because we want Spring AI to know that our expected result contains exactly these conceptual fields.

We use `List<String>` for attendees because a meeting can have multiple participants. The instructor's example is simple, and the PDF also shows a singular `attendee` field. Either shape is valid when it is chosen consistently.

We use `Integer` instead of primitive `int` because this makes it possible to represent a missing duration as `null`, should the application require that behavior.

### Step 2: Create MeetingService.java

```
package com.example.meetings;

import java.time.LocalDate;
import java.time.ZoneId;

import org.springframework.ai.chat.client.ChatClient;
import org.springframework.stereotype.Service;

@Service
public class MeetingService {

    private final ChatClient chatClient;

    public MeetingService(ChatClient.Builder builder) {
        this.chatClient = builder.build();
    }

    public MeetingDetails extractMeeting(String message) {

        ZoneId userZone = ZoneId.of("Asia/Kolkata");

        String today = LocalDate.now(userZone).toString();

        String systemPrompt = """
            You extract meeting information from
            natural-language user requests.

            Today's date is %s.
            User timezone is %s.

            Rules:

            1. Convert relative dates, such as today,
               tomorrow and day after tomorrow,
               into YYYY-MM-DD format.

            2. Convert time into 24-hour HH:mm format.

            3. If the title is missing, generate
               a simple descriptive title.

            4. If the duration is missing,
               default to 30 minutes.

            5. Do not invent attendees, dates or times.

            6. If attendees are missing, return
               an empty list.

            7. If date or time is missing,
               return an empty string for that field.

            8. Extract meeting details only.
               Do not claim to schedule the meeting.
            """.formatted(today, userZone);

        return chatClient.prompt()
            .system(systemPrompt)
            .user(message)
            .call()
            .entity(MeetingDetails.class);
    }
}
```

This follows the behavior described in the transcript. It assumes you have a compatible Spring AI starter and model configuration, with the API credentials provided through your application's configuration or environment.

Let's analyze the important details.

Why do we pass today's date?

The instructor emphasizes this at approximately 00:32:00.

The model is given the task of understanding expressions such as "today", "tomorrow", and "day after tomorrow".

But such expressions depend on the current date.

For example, if today's date is October 10, 2026:

| User expression    | Resolved date |
| ------------------ | ------------- |
| Today              | 2026-10-10    |
| Tomorrow           | 2026-10-11    |
| Day after tomorrow | 2026-10-12    |

Without a reliable reference date, the model could produce a plausible but incorrect answer.

The instructor uses `LocalDate.now()` to supply that reference.

Our version additionally specifies a timezone. In real applications, the user's timezone should come from their profile or request context rather than being hardcoded.

Why do we specify `YYYY-MM-DD`?

Because standardized formatting reduces ambiguity.

Consider:

```
01/10/2026
```

Is that January 10 or October 1?

Different countries interpret this differently.

Using:

```
2026-10-01
```

makes the intended calendar date much clearer.

Why default the duration to 30 minutes?

Because this is an explicit business rule chosen by the instructor.

It is not something structured output automatically does.

If your application's policy requires a user-specified duration, you should remove the default and ask the user instead.

Why allow a missing title but not a missing attendee or time?

A title such as `"Meeting with Rahul"` is relatively harmless as a default.

Inventing a meeting time or attendee can cause a real scheduling error.

The lecture therefore differentiates safe defaults from critical missing information.

### Step 3: Create MeetingController.java

```
package com.example.meetings;

import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api")
public class MeetingController {

    private final MeetingService meetingService;

    public MeetingController(MeetingService meetingService) {
        this.meetingService = meetingService;
    }

    @PostMapping(
        value = "/schedule",
        consumes = MediaType.TEXT_PLAIN_VALUE
    )
    public MeetingDetails schedule(
        @RequestBody String message
    ) {
        return meetingService.extractMeeting(message);
    }
}
```

This controller receives the user's raw text.

It delegates extraction to `MeetingService`.

Spring Boot then serializes the returned Java record into an HTTP JSON response, using its configured JSON serialization support.

That gives us two important conversions in one request:

```
Natural-language input
       |
       v
LLM structured output
       |
       v
MeetingDetails Java object
       |
       v
HTTP JSON response
```

### Step 4: Test using Postman or curl

Request:

```
POST http://localhost:8080/api/schedule
Content-Type: text/plain
```

Body:

```
Schedule a Docker discussion with Rahul
tomorrow at 4 PM for 30 minutes.
```

Using curl:

```
curl -X POST http://localhost:8080/api/schedule \
  -H "Content-Type: text/plain" \
  --data "Schedule a Docker discussion with Rahul tomorrow at 4 PM for 30 minutes."
```

Assuming today's date is October 10, 2026 and the application uses `Asia/Kolkata`, an expected response would be:

```
{
  "title": "Docker Discussion",
  "attendees": ["Rahul"],
  "date": "2026-10-11",
  "time": "16:00",
  "durationMinutes": 30
}
```

This is an illustrative expected response, not the result of an executed API call.

### Step 5: Understanding the entire lifecycle

| Stage | What happens                                                       |
| ----- | ------------------------------------------------------------------ |
| 1     | The user writes a request in natural language                      |
| 2     | Spring Boot receives the HTTP request                              |
| 3     | The service constructs a system prompt                             |
| 4     | The service supplies the current date and timezone                 |
| 5     | Spring AI derives the expected output schema from `MeetingDetails` |
| 6     | The LLM extracts the meeting information                           |
| 7     | Spring AI converts the response to `MeetingDetails`                |
| 8     | The controller returns that object                                 |
| 9     | Spring Boot serializes the object as HTTP JSON                     |
| 10    | Postman displays the extracted meeting details                     |

The meeting has still not been scheduled.

The purpose of this endpoint is currently extraction, despite its `/schedule` name. In a larger API, `/api/meetings/extract` would describe this behavior more precisely.

## 8. What happens when the user supplies incomplete information?

Lecture: 00:24:25–00:26:07 and 00:37:27–00:40:13

This is one of the most useful parts of the transcript because the instructor begins moving toward a conversational workflow.

Consider:

> Schedule a meeting with Rahul.

What information do we have?

We know the attendee is Rahul.

But we do not know when the meeting should take place.

The model could return:

```
{
  "title": "Meeting with Rahul",
  "attendees": ["Rahul"],
  "date": "",
  "time": "",
  "durationMinutes": 30
}
```

This is acceptable as an intermediate extraction result.

But it would be unacceptable to immediately create a calendar event because the required scheduling information is missing.

### 8.1 The instructor presents two approaches

The first is to leave unspecified information blank.

The second is to ask the user to provide the missing information.

The instructor recommends collecting missing information rather than inventing it.

### 8.2 How should you implement this properly?

Suppose you have the following conversation:

Schedule a meeting with Rahul.

What date should I schedule the meeting for?

Tomorrow.

What time should it start?

4 PM.

I have the details for your 30-minute meeting with Rahul tomorrow at 4 PM. Would you like me to create it?

The application must preserve previously supplied information across turns.

When the user says "Tomorrow", the model should understand that they are answering the previously asked date question, not starting a completely new request.

This is where conversation memory or persistent application state becomes useful.

### 8.3 Use an explicit state machine

The instructor proposes keeping chat history and potentially separating information collection from structured extraction.

Here is how I would extend that into a robust workflow:

\#chatgpt-mermaid-\_r_r6\_{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;fill:rgb(237, 237, 237);}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#chatgpt-mermaid-\_r_r6\_ .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_r6\_ .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_r6\_ .error-icon{fill:rgb(48, 48, 48);}#chatgpt-mermaid-\_r_r6\_ .error-text{fill:rgb(237, 237, 237);stroke:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_r6\_ .edge-thickness-normal{stroke-width:1px;}#chatgpt-mermaid-\_r_r6\_ .edge-thickness-thick{stroke-width:3.5px;}#chatgpt-mermaid-\_r_r6\_ .edge-pattern-solid{stroke-dasharray:0;}#chatgpt-mermaid-\_r_r6\_ .edge-thickness-invisible{stroke-width:0;fill:none;}#chatgpt-mermaid-\_r_r6\_ .edge-pattern-dashed{stroke-dasharray:3;}#chatgpt-mermaid-\_r_r6\_ .edge-pattern-dotted{stroke-dasharray:2;}#chatgpt-mermaid-\_r_r6\_ .marker{fill:rgb(175, 175, 175);stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_r6\_ .marker.cross{stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_r6\_ svg{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;}#chatgpt-mermaid-\_r_r6\_ p{margin:0;}#chatgpt-mermaid-\_r_r6\_ .label{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_r6\_ .cluster-label text{fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_r6\_ .cluster-label span{color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_r6\_ .cluster-label span p{background-color:transparent;}#chatgpt-mermaid-\_r_r6\_ .label text,#chatgpt-mermaid-\_r_r6\_ span{fill:rgb(237, 237, 237);color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_r6\_ .node rect,#chatgpt-mermaid-\_r_r6\_ .node circle,#chatgpt-mermaid-\_r_r6\_ .node ellipse,#chatgpt-mermaid-\_r_r6\_ .node polygon,#chatgpt-mermaid-\_r_r6\_ .node path{fill:rgb(9, 23, 44);stroke:rgb(31, 78, 148);stroke-width:1px;}#chatgpt-mermaid-\_r_r6\_ .rough-node .label text,#chatgpt-mermaid-\_r_r6\_ .node .label text,#chatgpt-mermaid-\_r_r6\_ .image-shape .label,#chatgpt-mermaid-\_r_r6\_ .icon-shape .label{text-anchor:middle;}#chatgpt-mermaid-\_r_r6\_ .node .katex path{fill:#000;stroke:#000;stroke-width:1px;}#chatgpt-mermaid-\_r_r6\_ .rough-node .label,#chatgpt-mermaid-\_r_r6\_ .node .label,#chatgpt-mermaid-\_r_r6\_ .image-shape .label,#chatgpt-mermaid-\_r_r6\_ .icon-shape .label{text-align:center;}#chatgpt-mermaid-\_r_r6\_ .node.clickable{cursor:pointer;}#chatgpt-mermaid-\_r_r6\_ .root .anchor path{fill:rgb(175, 175, 175)!important;stroke-width:0;stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_r6\_ .arrowheadPath{fill:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_r6\_ .edgePath .path{stroke:rgb(175, 175, 175);stroke-width:1px;}#chatgpt-mermaid-\_r_r6\_ .flowchart-link{stroke:rgb(175, 175, 175);fill:none;}#chatgpt-mermaid-\_r_r6\_ .edgeLabel{background-color:rgb(0, 0, 0);text-align:center;}#chatgpt-mermaid-\_r_r6\_ .edgeLabel p{background-color:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_r6\_ .edgeLabel rect{opacity:0.5;background-color:rgb(0, 0, 0);fill:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_r6\_ .labelBkg{background-color:rgba(0, 0, 0, 0.5);}#chatgpt-mermaid-\_r_r6\_ .cluster rect{fill:rgb(48, 48, 48);stroke:rgba(255, 255, 255, 0.15);stroke-width:1px;}#chatgpt-mermaid-\_r_r6\_ .cluster text{fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_r6\_ .cluster span{color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_r6\_ div.mermaidTooltip{position:absolute;text-align:center;max-width:200px;padding:2px;font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:12px;background:rgb(48, 48, 48);border:1px solid rgba(255, 255, 255, 0.15);border-radius:2px;pointer-events:none;z-index:100;}#chatgpt-mermaid-\_r_r6\_ .flowchartTitleText{text-anchor:middle;font-size:18px;fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_r6\_ rect.text{fill:none;stroke-width:0;}#chatgpt-mermaid-\_r_r6\_ .icon-shape,#chatgpt-mermaid-\_r_r6\_ .image-shape{background-color:rgb(0, 0, 0);text-align:center;}#chatgpt-mermaid-\_r_r6\_ .icon-shape p,#chatgpt-mermaid-\_r_r6\_ .image-shape p{background-color:rgb(0, 0, 0);padding:2px;}#chatgpt-mermaid-\_r_r6\_ .icon-shape .label rect,#chatgpt-mermaid-\_r_r6\_ .image-shape .label rect{opacity:0.5;background-color:rgb(0, 0, 0);fill:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_r6\_ .label-icon{display:inline-block;height:1em;overflow:visible;vertical-align:-0.125em;}#chatgpt-mermaid-\_r_r6\_ .node .label-icon path{fill:currentColor;stroke:revert;stroke-width:revert;}#chatgpt-mermaid-\_r_r6\_ .node .neo-node{stroke:rgb(31, 78, 148);}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].node rect,#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].cluster rect,#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].node polygon{stroke:url(#chatgpt-mermaid-\_r_r6\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].swimlane.cluster rect{filter:none;}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].node path{stroke:url(#chatgpt-mermaid-\_r_r6\_-gradient);stroke-width:1px;}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].node .outer-path{filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].node .neo-line path{stroke:rgb(31, 78, 148);filter:none;}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].node circle{stroke:url(#chatgpt-mermaid-\_r_r6\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].node circle .state-start{fill:#000000;}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].icon-shape .icon{fill:url(#chatgpt-mermaid-\_r_r6\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_r6\_ [data-look="neo"].icon-shape .icon-neo path{stroke:url(#chatgpt-mermaid-\_r_r6\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_r6\_ .node text{font-size:14px;font-weight:600;letter-spacing:normal;fill:rgb(153, 206, 255);}#chatgpt-mermaid-\_r_r6\_ .edgeLabels text{font-size:13px;font-weight:600;letter-spacing:-0.08px;fill:rgb(153, 206, 255);}#chatgpt-mermaid-\_r_r6\_ .node tspan[font-weight="normal"],#chatgpt-mermaid-\_r_r6\_ .edgeLabels tspan[font-weight="normal"]{font-weight:600;}#chatgpt-mermaid-\_r_r6\_ .edgeLabel .label rect{opacity:1;rx:13px;ry:13px;fill:rgb(0, 14, 26);stroke:rgb(26, 62, 95);stroke-width:1px;}#chatgpt-mermaid-\_r_r6\_ .node rect,#chatgpt-mermaid-\_r_r6\_ .node circle,#chatgpt-mermaid-\_r_r6\_ .node ellipse,#chatgpt-mermaid-\_r_r6\_ .node polygon,#chatgpt-mermaid-\_r_r6\_ .node path{fill:rgb(0, 40, 77);stroke:rgba(255, 255, 255, 0.1);stroke-width:1px;}#chatgpt-mermaid-\_r_r6\_ .node rect{rx:16px;ry:16px;}#chatgpt-mermaid-\_r_r6\_ .node.mermaid-decision .label-container{fill:rgb(0, 14, 26);stroke:rgb(26, 62, 95);stroke-dasharray:2,2;}#chatgpt-mermaid-\_r_r6\_ .edgePaths .flowchart-link{stroke:rgb(175, 175, 175);stroke-width:1px;stroke-linecap:round;stroke-linejoin:round;}#chatgpt-mermaid-\_r_r6\_ .marker{fill:rgb(175, 175, 175);stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_r6\_ :root{--mermaid-font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";}#chatgpt-mermaid-\_r_r6\_ .soft rect{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_r6\_ .soft polygon{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_r6\_ .soft ellipse{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_r6\_ .soft circle{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_r6\_ .soft path{fill:rgb(220, 234, 250)!important;stroke:rgb(59, 130, 246)!important;color:rgb(23, 52, 93)!important;}#chatgpt-mermaid-\_r_r6\_ .soft tspan{fill:rgb(23, 52, 93)!important;}User messageExtract meeting detailsRequired details present?Save partial meeting draftAsk for missing fieldsUser repliesValidate extracted valuesValid?Show meeting summaryUser confirms?Execute calendar integrationReturn actual resultNoYesNoYesNoYes

Notice that the application—not the LLM alone—controls whether a real meeting can be created.

For example, define explicit states:

```
public enum MeetingState {
    NEEDS_INFORMATION,
    READY_FOR_CONFIRMATION,
    CONFIRMED,
    SCHEDULED,
    FAILED
}
```

Why is this better?

Because the backend can enforce clear transition rules.

A meeting cannot jump from `NEEDS_INFORMATION` directly to `SCHEDULED`.

The application must establish that all mandatory details are present and that the necessary authorization and confirmation requirements have been met.

### 8.4 Structured output can also represent incomplete workflows

An even more sophisticated pattern is to represent the extraction result itself as structured data.

For example:

```
{
  "status": "NEEDS_INFORMATION",
  "meeting": {
    "title": "Meeting with Rahul",
    "attendees": ["Rahul"],
    "date": null,
    "time": null,
    "durationMinutes": 30
  },
  "missingFields": ["date", "time"],
  "clarificationQuestion": "What date and time should I use?"
}
```

This can be modeled as a more expressive output contract.

```
public enum ExtractionStatus {
    COMPLETE,
    NEEDS_INFORMATION
}
```

```
public record MeetingExtractionResult(
    ExtractionStatus status,
    MeetingDetails meeting,
    List<String> missingFields,
    String clarificationQuestion
) {}
```

In this design, the application receives not just meeting data, but the extraction state as well.

The nested `MeetingDetails` contract would need to allow null values for missing fields.

Also, the backend should not blindly trust the model's `status`. It should recompute completeness from the actual extracted fields.

For example:

```
List<String> missing = new ArrayList<>();

if (meeting.attendees() == null ||
    meeting.attendees().isEmpty()) {
    missing.add("attendees");
}

if (meeting.date() == null ||
    meeting.date().isBlank()) {
    missing.add("date");
}

if (meeting.time() == null ||
    meeting.time().isBlank()) {
    missing.add("time");
}

if (!missing.isEmpty()) {
    // Save draft and request clarification.
}
```

This is an example of an important principle:

Let the LLM interpret information. Let deterministic application code enforce required conditions.

## 9. Structured output versus tool calling

Lecture: 00:14:09–00:16:26 | PDF: pages 11–14

The instructor spends several minutes distinguishing structured output from AI tool calling.

This is especially important because both mechanisms can involve JSON objects and schemas.

### 9.1 Structured output

A user says:

> Schedule a Docker discussion with Rahul tomorrow at 4 PM.

The LLM returns data:

```
{
  "title": "Docker Discussion",
  "attendee": "Rahul",
  "date": "2026-10-11",
  "time": "16:00"
}
```

The model has produced an application-compatible representation of the user's request.

Nothing has necessarily been executed.

### 9.2 Tool calling

A user asks:

> What is the weather in Delhi right now?

The LLM decides it needs a weather tool.

It generates tool arguments such as:

```
{
  "city": "Delhi"
}
```

A tool-execution layer calls the relevant weather API.

The result comes back:

```
{
  "city": "Delhi",
  "temperatureCelsius": 29,
  "condition": "Cloudy"
}
```

The model can then prepare a user-facing answer.

The weather values above are hypothetical examples, not current weather observations.

### 9.3 Compare the architectures

### Structured output

User request

LLM

Typed data

The output object itself is the goal.

### Tool calling

User request

LLM selects tool

Tool execution

Result

The structured arguments support an action.

| Dimension                    | Structured output             | Tool calling                      |
| ---------------------------- | ----------------------------- | --------------------------------- |
| Main purpose                 | Produce usable data           | Request execution of an operation |
| LLM result                   | Defined output object         | Tool name and arguments           |
| External execution           | Not inherently required       | Usually follows the tool request  |
| Who performs actual actions? | Application, if it chooses to | Tool executor/application         |
| Example                      | Extract `MeetingDetails`      | Invoke `createCalendarEvent`      |
| Can both be combined?        | Yes                           | Yes                               |

A subtle but important correction to a common simplification: tool calling does not necessarily mean the model directly invokes software itself.

Usually, the model generates a tool-call request. The hosting application or framework validates the arguments, applies permissions, runs the function, and returns the result.

The model's role is proposing or selecting the operation.

The application's role is executing it.

### 9.4 Can we combine both techniques?

Absolutely.

For example:

```
User
  |
  v
LLM extracts MeetingDetails
  |
  v
Backend validates MeetingDetails
  |
  v
User confirms
  |
  v
Calendar tool executes
  |
  v
Calendar returns event ID
  |
  v
Backend returns actual scheduling result
```

This combination is often simpler to control than allowing the LLM to drive the entire workflow autonomously.

There are also agentic designs in which the model selects the tool directly and produces structured arguments for that tool.

Both are useful architectural patterns.

## 10. How does MCP fit into this?

Lecture: 00:16:20 onward and 00:40:20–00:40:39 | PDF: pages 14–16

MCP stands for Model Context Protocol.

It provides a standardized way for AI applications and compatible hosts to connect to external capabilities, including tools and data sources.

This is why the instructor introduces structured outputs before MCP.

Tools often need predictable argument structures.

Imagine an external calendar integration exposes a tool called:

```
create_calendar_event
```

The tool expects:

```
{
  "title": "Docker Discussion",
  "startTime": "2026-10-11T16:00:00+05:30",
  "endTime": "2026-10-11T16:30:00+05:30",
  "attendeeEmails": ["rahul@example.com"]
}
```

The tool cannot safely execute arbitrary text such as:

```
Please schedule my meeting with Rahul tomorrow.
```

It needs well-defined arguments.

But there are two separate concepts here.

Structured output is about producing data that follows an expected contract.

MCP is about the standardized interface through which a compatible host discovers and interacts with external capabilities.

MCP itself does not automatically provide authorization, correct date interpretation, calendar availability, or reliable execution of every requested action. Those responsibilities still need to be implemented by the surrounding system.

### What would the architecture look like with MCP?

\#chatgpt-mermaid-\_r_ug\_{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;fill:rgb(237, 237, 237);}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#chatgpt-mermaid-\_r_ug\_ .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_ug\_ .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#chatgpt-mermaid-\_r_ug\_ .error-icon{fill:rgb(48, 48, 48);}#chatgpt-mermaid-\_r_ug\_ .error-text{fill:rgb(237, 237, 237);stroke:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_ug\_ .edge-thickness-normal{stroke-width:1px;}#chatgpt-mermaid-\_r_ug\_ .edge-thickness-thick{stroke-width:3.5px;}#chatgpt-mermaid-\_r_ug\_ .edge-pattern-solid{stroke-dasharray:0;}#chatgpt-mermaid-\_r_ug\_ .edge-thickness-invisible{stroke-width:0;fill:none;}#chatgpt-mermaid-\_r_ug\_ .edge-pattern-dashed{stroke-dasharray:3;}#chatgpt-mermaid-\_r_ug\_ .edge-pattern-dotted{stroke-dasharray:2;}#chatgpt-mermaid-\_r_ug\_ .marker{fill:rgb(175, 175, 175);stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_ug\_ .marker.cross{stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_ug\_ svg{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:16px;}#chatgpt-mermaid-\_r_ug\_ p{margin:0;}#chatgpt-mermaid-\_r_ug\_ .label{font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_ug\_ .cluster-label text{fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_ug\_ .cluster-label span{color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_ug\_ .cluster-label span p{background-color:transparent;}#chatgpt-mermaid-\_r_ug\_ .label text,#chatgpt-mermaid-\_r_ug\_ span{fill:rgb(237, 237, 237);color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_ug\_ .node rect,#chatgpt-mermaid-\_r_ug\_ .node circle,#chatgpt-mermaid-\_r_ug\_ .node ellipse,#chatgpt-mermaid-\_r_ug\_ .node polygon,#chatgpt-mermaid-\_r_ug\_ .node path{fill:rgb(9, 23, 44);stroke:rgb(31, 78, 148);stroke-width:1px;}#chatgpt-mermaid-\_r_ug\_ .rough-node .label text,#chatgpt-mermaid-\_r_ug\_ .node .label text,#chatgpt-mermaid-\_r_ug\_ .image-shape .label,#chatgpt-mermaid-\_r_ug\_ .icon-shape .label{text-anchor:middle;}#chatgpt-mermaid-\_r_ug\_ .node .katex path{fill:#000;stroke:#000;stroke-width:1px;}#chatgpt-mermaid-\_r_ug\_ .rough-node .label,#chatgpt-mermaid-\_r_ug\_ .node .label,#chatgpt-mermaid-\_r_ug\_ .image-shape .label,#chatgpt-mermaid-\_r_ug\_ .icon-shape .label{text-align:center;}#chatgpt-mermaid-\_r_ug\_ .node.clickable{cursor:pointer;}#chatgpt-mermaid-\_r_ug\_ .root .anchor path{fill:rgb(175, 175, 175)!important;stroke-width:0;stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_ug\_ .arrowheadPath{fill:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_ug\_ .edgePath .path{stroke:rgb(175, 175, 175);stroke-width:1px;}#chatgpt-mermaid-\_r_ug\_ .flowchart-link{stroke:rgb(175, 175, 175);fill:none;}#chatgpt-mermaid-\_r_ug\_ .edgeLabel{background-color:rgb(0, 0, 0);text-align:center;}#chatgpt-mermaid-\_r_ug\_ .edgeLabel p{background-color:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_ug\_ .edgeLabel rect{opacity:0.5;background-color:rgb(0, 0, 0);fill:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_ug\_ .labelBkg{background-color:rgba(0, 0, 0, 0.5);}#chatgpt-mermaid-\_r_ug\_ .cluster rect{fill:rgb(48, 48, 48);stroke:rgba(255, 255, 255, 0.15);stroke-width:1px;}#chatgpt-mermaid-\_r_ug\_ .cluster text{fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_ug\_ .cluster span{color:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_ug\_ div.mermaidTooltip{position:absolute;text-align:center;max-width:200px;padding:2px;font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";font-size:12px;background:rgb(48, 48, 48);border:1px solid rgba(255, 255, 255, 0.15);border-radius:2px;pointer-events:none;z-index:100;}#chatgpt-mermaid-\_r_ug\_ .flowchartTitleText{text-anchor:middle;font-size:18px;fill:rgb(237, 237, 237);}#chatgpt-mermaid-\_r_ug\_ rect.text{fill:none;stroke-width:0;}#chatgpt-mermaid-\_r_ug\_ .icon-shape,#chatgpt-mermaid-\_r_ug\_ .image-shape{background-color:rgb(0, 0, 0);text-align:center;}#chatgpt-mermaid-\_r_ug\_ .icon-shape p,#chatgpt-mermaid-\_r_ug\_ .image-shape p{background-color:rgb(0, 0, 0);padding:2px;}#chatgpt-mermaid-\_r_ug\_ .icon-shape .label rect,#chatgpt-mermaid-\_r_ug\_ .image-shape .label rect{opacity:0.5;background-color:rgb(0, 0, 0);fill:rgb(0, 0, 0);}#chatgpt-mermaid-\_r_ug\_ .label-icon{display:inline-block;height:1em;overflow:visible;vertical-align:-0.125em;}#chatgpt-mermaid-\_r_ug\_ .node .label-icon path{fill:currentColor;stroke:revert;stroke-width:revert;}#chatgpt-mermaid-\_r_ug\_ .node .neo-node{stroke:rgb(31, 78, 148);}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].node rect,#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].cluster rect,#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].node polygon{stroke:url(#chatgpt-mermaid-\_r_ug\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].swimlane.cluster rect{filter:none;}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].node path{stroke:url(#chatgpt-mermaid-\_r_ug\_-gradient);stroke-width:1px;}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].node .outer-path{filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].node .neo-line path{stroke:rgb(31, 78, 148);filter:none;}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].node circle{stroke:url(#chatgpt-mermaid-\_r_ug\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].node circle .state-start{fill:#000000;}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].icon-shape .icon{fill:url(#chatgpt-mermaid-\_r_ug\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_ug\_ [data-look="neo"].icon-shape .icon-neo path{stroke:url(#chatgpt-mermaid-\_r_ug\_-gradient);filter:drop-shadow( 1px 2px 2px rgba(185,185,185,1));}#chatgpt-mermaid-\_r_ug\_ .node text{font-size:14px;font-weight:600;letter-spacing:normal;fill:rgb(153, 206, 255);}#chatgpt-mermaid-\_r_ug\_ .edgeLabels text{font-size:13px;font-weight:600;letter-spacing:-0.08px;fill:rgb(153, 206, 255);}#chatgpt-mermaid-\_r_ug\_ .node tspan[font-weight="normal"],#chatgpt-mermaid-\_r_ug\_ .edgeLabels tspan[font-weight="normal"]{font-weight:600;}#chatgpt-mermaid-\_r_ug\_ .edgeLabel .label rect{opacity:1;rx:13px;ry:13px;fill:rgb(0, 14, 26);stroke:rgb(26, 62, 95);stroke-width:1px;}#chatgpt-mermaid-\_r_ug\_ .node rect,#chatgpt-mermaid-\_r_ug\_ .node circle,#chatgpt-mermaid-\_r_ug\_ .node ellipse,#chatgpt-mermaid-\_r_ug\_ .node polygon,#chatgpt-mermaid-\_r_ug\_ .node path{fill:rgb(0, 40, 77);stroke:rgba(255, 255, 255, 0.1);stroke-width:1px;}#chatgpt-mermaid-\_r_ug\_ .node rect{rx:16px;ry:16px;}#chatgpt-mermaid-\_r_ug\_ .node.mermaid-decision .label-container{fill:rgb(0, 14, 26);stroke:rgb(26, 62, 95);stroke-dasharray:2,2;}#chatgpt-mermaid-\_r_ug\_ .edgePaths .flowchart-link{stroke:rgb(175, 175, 175);stroke-width:1px;stroke-linecap:round;stroke-linejoin:round;}#chatgpt-mermaid-\_r_ug\_ .marker{fill:rgb(175, 175, 175);stroke:rgb(175, 175, 175);}#chatgpt-mermaid-\_r_ug\_ :root{--mermaid-font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";}UserMeeting ApplicationLLM extracts or interpretsrequestValidated meeting detailsConfirmation andauthorizationMCP Host / ClientCalendar MCP ServerCalendar Provider APIEvent resultSuccess or error response touser

The diagram is an illustrative architecture, not the implementation built in the video.

An application can also integrate Google Calendar directly with its API without using MCP.

MCP is not mandatory for tool calling or external API integration. It is one approach for standardizing those interactions.

## 11. The advanced engineering concepts the lecture introduces indirectly

The following sections expand beyond the introductory lecture. They are important when you turn structured output into production software.

### 11.1 Schema validity does not guarantee correct business logic

Consider this structured response:

It can be syntactically valid JSON.

It might even deserialize into a Java object.

But a meeting cannot have a negative duration.

Similarly, a meeting date could already be in the past.

You therefore need more than a JSON parser.

A robust backend generally performs several distinct checks:

| Validation layer    | Example                                  |
| ------------------- | ---------------------------------------- |
| Syntax              | Is this valid JSON?                      |
| Schema              | Are the expected keys and types present? |
| Format              | Can the date and time be parsed?         |
| Domain rules        | Is the duration positive?                |
| Authorization       | Can this user create the meeting?        |
| External validation | Does the attendee exist?                 |
| Availability        | Is the selected time slot available?     |

These checks should not be delegated entirely to the model.

### 11.2 Timezones are a real backend concern

The transcript converts:

into:

This is useful but incomplete for a production calendar system.

The value `16:00` represents a local clock time. It does not say which timezone applies.

For example, 4 PM in Mumbai and 4 PM in London are different instants.

A scheduling application should distinguish:

The simplest robust approach is to combine the local date, local time, and timezone before converting to an instant.

This makes it possible to store or compare the time consistently.

For applications operating internationally, daylight saving time introduces further complexity, including ambiguous or nonexistent local times.

The timezone should come from an authoritative source, such as the user's saved settings or an explicitly stated timezone in the request.

### 11.3 Names are not reliable identity keys

The model extracts:

But who is Rahul?

What if the user has three contacts named Rahul?

The model should not guess an email address.

A production calendar system needs an