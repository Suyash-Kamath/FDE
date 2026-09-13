Yes — this was one of the most important parts of the transcript, and the lecturer was clearing up a very common confusion:

> **“If I keep giving information to the LLM in prompts, is the model learning that information permanently?”**

His answer was:

> **No. Context window and model parameters are completely different things.**

Here’s the exact intuition.

---

# 1. Model parameters = what the model learned during training

When an LLM is trained, it sees a huge amount of text and repeatedly tries to predict the next token.

Very simplified:

```text
Input:
"The capital of India is ..."

Model predicts:
"Mumbai"

Correct answer:
"Delhi"

Error happens
↓
training algorithm adjusts model weights
↓
model gets slightly better
```

This repeats billions/trillions of times.

Those learned internal numbers are called:

> **parameters / weights**

So you can think:

```text
Training data
    ↓
learning process
    ↓
weights / parameters change
    ↓
trained model
```

Those parameters are the model’s long-term learned structure.

---

# 2. Your prompt does NOT normally change those parameters

This is exactly what the lecturer was emphasizing.

Suppose you tell the model:

```text
My name is Aditya.
```

That does **not** mean:

```text
Model weights updated:
"User's name = Aditya"
```

No.

During a normal API call, you are doing **inference**, not training.

Conceptually:

```text
already-trained fixed model
        +
your temporary prompt/context
        ↓
generate answer
```

The parameters stay fixed during that request.

---

# 3. Then what is the context window?

Context window is the temporary information the model can see **right now** while generating a response.

Suppose you send:

```text
SYSTEM:
You are a food delivery support agent.

USER:
My name is Aditya.

ASSISTANT:
Hi Aditya.

USER:
What is my name?
```

All of that becomes part of the current context.

The model can answer:

```text
Your name is Aditya.
```

But it answered that because:

> `"Aditya"` existed in the current context.

Not because its parameters were retrained.

That is the difference.

---

# 4. Think of it as long-term learned brain vs temporary desk

A very useful analogy:

## Model parameters

Imagine a student's brain after years of education.

They already learned:

```text
math
English
history
programming concepts
patterns
```

That is somewhat analogous to trained model parameters.

---

## Context window

Now during an exam you give the student a piece of paper:

```text
Customer name = Aditya
Order ID = 9281
Order status = delayed
```

The student can use that information while solving the current problem.

But giving them the paper does not permanently rewire their brain.

That's context.

So:

```text
Model parameters
= what the model learned during training

Context window
= information temporarily available for the current task
```

---

# 5. This was the confusion the lecturer was addressing

People often think:

> “I told ChatGPT something, therefore the LLM learned it.”

But the lecturer explains that these are two different mechanisms.

### Training

Changes the model:

```text
weights before
    ↓
training
    ↓
weights after
```

### Prompting

Doesn't normally change weights:

```text
fixed model
+
temporary context
↓
output
```

That distinction is massive.

---

# 6. Why can the model answer things from the prompt then?

Because the model does not need to permanently learn something to use it.

Suppose the trained model knows how to understand sentences like:

```text
"My favorite language is Go."
```

You provide:

```text
My favorite language is Go.
What is my favorite language?
```

The model uses the information present in the context.

So:

```text
model knowledge:
understands language and relationships

current context:
favorite language = Go
```

Together:

```text
Answer:
Go
```

No retraining required.

---

# 7. Parameters are persistent; context is temporary

This is probably the cleanest comparison.

| Model ParametersContext Window |                                         |
| ------------------------------ | --------------------------------------- |
| Learned during training        | Supplied during inference               |
| Long-lived                     | Temporary                               |
| Internal weights               | Input tokens/messages                   |
| Expensive to change            | Easy to change                          |
| Requires training/fine-tuning  | Requires just sending another prompt    |
| Shared model capability        | Request-specific information            |
| Gives broad learned patterns   | Gives current task-specific information |

---

# 8. Example from the transcript

The lecturer gave the example of conversation history.

Suppose:

```text
User:
Hi my name is Aditya.

Assistant:
Hi Aditya.

User:
What is my name?
```

The model can answer only because the application sends previous history again.

Conceptually:

```text
Current request context:

Hi my name is Aditya.
Hi Aditya.
What is my name?
```

Therefore:

```text
LLM → "Your name is Aditya."
```

The lecturer's point was:

> The model didn't retrain itself after the first message.

The application simply put the previous information back inside the context window.

---

# 9. What happens when context disappears?

This makes the difference obvious.

Imagine:

### API call 1

```text
My name is Aditya.
```

Model:

```text
Nice to meet you, Aditya.
```

Finished.

Then completely independent API call:

```text
What is my name?
```

with no history.

Now the model might say:

```text
I don't know your name.
```

Why?

Because:

```text
parameters never stored "Aditya is this user's name"
```

and:

```text
current context does not contain "Aditya"
```

Therefore it doesn't know.

---

# 10. This is why context is called temporary

The transcript essentially says:

> Context window is a temporary part of generation.

You can imagine:

```text
REQUEST STARTS

System prompt
Conversation
Documents
Current user query
Tool results

      ↓

LLM processes them

      ↓

Response generated

REQUEST ENDS
```

The model doesn't automatically take that entire request and write it into its weights.

---

# 11. Then where does ChatGPT memory come from?

This was another confusion the lecturer addresses.

If ChatGPT remembers something across conversations, don't immediately think:

```text
LLM weights changed.
```

Instead think:

```text
Application stored information somewhere
        ↓
later retrieved it
        ↓
put relevant information into context
        ↓
LLM used it
```

So:

```text
Application memory
≠
Model training
```

This is one of the biggest lessons from the transcript.

---

# 12. Three things you should never mix up

You should mentally maintain these three separate boxes.

## Box 1 — Model Parameters

```text
What did the model learn during training?
```

Examples:

```text
language patterns
coding patterns
general world knowledge
reasoning patterns
```

---

## Box 2 — Application Memory

```text
What information has your app saved?
```

Examples:

```text
user name
preferences
past conversation
order ID
company-specific information
```

Stored in things like:

```text
database
Redis
memory service
vector DB
files
```

---

## Box 3 — Context Window

```text
What information are we giving the model right now?
```

Could contain:

```text
system prompt
recent history
retrieved memory
RAG documents
tool output
current message
```

Then:

```text
Context Window
        ↓
LLM with fixed parameters
        ↓
answer
```

---

# 13. How all three connect

This is the architecture you should remember:

```text
                  TRAINING
                     │
                     ▼
              MODEL PARAMETERS
                     │
                     │ fixed during normal inference
                     ▼
               ┌───────────┐
               │    LLM    │
               └─────▲─────┘
                     │
                     │ context
                     │
        ┌────────────┼────────────┐
        │            │            │
 System prompt   Chat history   RAG/memory
        │            │            │
        └────────────┼────────────┘
                     │
                User message
```

Parameters determine how the model processes language.

Context determines what it should think about **now**.

---

# 14. Another excellent analogy: CPU program + RAM

Not exact technically, but very useful.

Think:

```text
Model parameters ≈ installed program / learned machinery
```

and:

```text
Context window ≈ RAM / active working data
```

Suppose your laptop has a program installed.

Opening a document doesn't rewrite the application's executable binary.

Likewise:

```text
Prompt/context
```

doesn't usually rewrite:

```text
model weights.
```

---

# 15. Why context window has a limit but parameters can be huge

A model may have billions of parameters.

But that doesn't mean you can give it infinite prompt text.

These are separate capacities.

Imagine:

```text
Model:
very large neural network
```

but:

```text
context window:
limited number of tokens per interaction
```

So a model can have enormous learned capacity while still having a finite active context size.

---

# 16. Why this matters for RAG

This distinction becomes extremely useful when you study RAG.

Suppose your company has:

```text
10 million documents
```

You don't train the LLM on them every time.

And you definitely don't place all 10 million documents into the context window.

Instead:

```text
10 million documents
        ↓
retrieve relevant 5 chunks
        ↓
put only those chunks in context
        ↓
LLM answers
```

Again:

```text
external knowledge storage
≠
model parameters
≠
current context
```

---

# 17. Why this matters for fine-tuning

Suppose you actually **fine-tune** a model.

Now you are changing parameters.

That is fundamentally different from prompting.

Prompt:

```text
"You are Tomato customer support."
```

means:

```text
temporarily behave this way.
```

Fine-tuning would be closer to:

```text
adjust model weights so this behavior/pattern becomes part of the learned model.
```

Very different operations.

---

# 18. Another transcript-specific point: context guides next-token prediction

The lecturer connects context directly with next-token probability.

Suppose:

```text
Explain Docker.
I am a complete beginner.
```

The model may generate:

```text
Docker is like a box...
```

Now change context:

```text
Explain Docker.
I am a senior kernel engineer.
```

The model may generate:

```text
Docker uses namespaces, cgroups...
```

Same model parameters.

Different context.

Different token probabilities.

So:

```text
Fixed weights
+
different context
=
different output distribution
```

That is a deep point.

---

# 19. Context does not give the model new permanent knowledge

Suppose you tell it:

```text
In my fictional company,
the CEO is Rahul.
```

The model can answer:

```text
Who is the CEO?

Rahul.
```

during the current context.

But you have not permanently taught the foundation model:

```text
CEO = Rahul
```

That is just temporary task information.

---

# 20. The shortest definition

If you want to remember this permanently:

> **Parameters = what the model learned.**

> **Context window = what the model is currently looking at.**

And:

> **Application memory = what your software has stored and can later put back into the context window.**

That's the exact confusion the lecturer was clearing up.