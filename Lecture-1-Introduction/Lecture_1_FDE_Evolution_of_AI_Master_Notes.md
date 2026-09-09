# Lecture 1 — Forward Deployed Engineer + Evolution of AI

## Extremely Detailed Master Notes

These notes follow the lecture transcript closely, including the instructor’s examples, progression of ideas, and the connection between **Forward Deployed Engineering, AI, GenAI, NLP, Transformers, and LLMs**.

---

# 0. What is this lecture actually trying to teach?

This lecture has **two parallel goals**.

The first goal is to answer:

> **What exactly does a Forward Deployed Engineer do?**

The second goal is to build the conceptual foundation for:

> **Why modern AI/LLM systems are useful for solving real business problems.**

So the lecture is not just an AI-history lecture.

The bigger story is:

```text
Real Business Problem
        ↓
Understand actual requirement
        ↓
Try simplest possible solution
        ↓
Understand why it fails
        ↓
Move to a more capable approach
        ↓
Rule-Based AI
        ↓
Machine Learning
        ↓
Neural Networks / Deep Learning
        ↓
Language Models
        ↓
Transformers
        ↓
LLMs
        ↓
Production AI System
```

And this is essentially the mindset the instructor wants an **FDE** to develop.

---

# PART I — FORWARD DEPLOYED ENGINEERING

# 1. The starting example: E-commerce customer support

Imagine an e-commerce company.

Every day, thousands of customers ask questions such as:

- Where is my order?
- Why hasn't my order arrived?
- I received the wrong product.
- Can I return my order?
- What is the refund policy?
- Can I cancel my order?
- My payment failed.
- My product arrived damaged.

Traditionally, the company might employ a large team of:

> **Customer Support Executives**

Their job is to manually answer these queries.

---

# 2. Why is this a business problem?

Suppose the company receives:

```text
50,000 support queries/day
```

A purely human support system creates several problems.

## Problem 1 — Cost

You need many support employees.

That means:

- salaries
- office infrastructure
- hiring
- training
- management
- shifts
- supervision

Therefore customer support becomes expensive.

---

## Problem 2 — Waiting queues

Imagine:

```text
100 support executives
```

and all 100 are already talking to customers.

Customer #101 cannot immediately receive support.

They enter a:

```text
waiting queue
```

You've probably experienced:

> "All our executives are currently busy. Please stay on the line."

This creates customer frustration.

---

# 3. Many queries are repetitive

An important observation made in the lecture is that a large number of support requests may be extremely simple.

For example:

> "Where is my order?"

The company probably already has the answer in its database.

Another:

> "What is your return policy?"

The return policy probably already exists on the website.

Therefore a human employee is often doing nothing more than:

```text
Customer asks question
        ↓
Employee finds existing information
        ↓
Employee tells customer
```

That is repetitive work.

And repetitive work is often a good candidate for automation.

---

# 4. Important customer problems may get buried

Suppose there are thousands of trivial questions:

```text
Where is my order?
Where is my refund?
What is your return policy?
How can I cancel?
```

Inside those thousands of queries there might be something serious:

```text
My account has been compromised.
₹50,000 was charged incorrectly.
I have contacted support five times.
My account is locked.
```

If the support team is overwhelmed, serious cases may receive insufficient attention.

Therefore automation isn't necessarily about:

> "Removing all humans."

A better architecture could be:

```text
AI handles routine cases
        +
Humans handle exceptional/important cases
```

This idea will become extremely important in real AI systems.

---

# 5. Human support has working-hour limitations

Humans don't work 24×7.

Customers, however, may have problems:

```text
2 PM
2 AM
Sunday
Holiday
Any timezone
```

A software system can potentially operate:

```text
24 × 7 × 365
```

Therefore automation can improve availability.

---

# 6. Management gives you a requirement

Management approaches you and says:

> **"We need an AI chatbot."**

At first glance this sounds like the requirement.

But according to the lecture:

**This is not really the requirement.**

This distinction is probably the most important FDE lesson in Lecture 1.

---

# 7. Requirement vs proposed solution

Management says:

> Build an AI chatbot.

But why?

Because they want something deeper.

The actual business goal is closer to:

> **Reduce the time required to resolve customer problems without reducing accuracy or customer trust.**

That is much stronger than:

> "Build chatbot."

---

# 8. Why "build an AI chatbot" is a bad requirement

Because it already assumes the solution.

Imagine someone says:

> "I need Kafka."

An FDE shouldn't immediately say:

> "Okay, we'll implement Kafka."

Instead ask:

> Why?

Maybe they actually need:

- asynchronous processing
- event delivery
- buffering
- retries
- decoupling

Kafka might be one solution.

But perhaps RabbitMQ, SQS, Redis streams, or even a database table would solve it.

Same here.

```text
"AI chatbot"
```

is one possible solution.

The actual problem is:

```text
Customer support resolution
```

---

# 9. A fundamental FDE thinking pattern

Think:

```text
Customer asks for X
        ↓
Why does customer want X?
        ↓
What pain exists today?
        ↓
What measurable business outcome do they want?
        ↓
What constraints exist?
        ↓
What is the simplest solution?
        ↓
Does it actually solve the pain?
```

This is dramatically different from:

```text
Requirement → Code
```

---

# 10. Normal engineer vs Forward Deployed Engineer

The lecture draws an important distinction.

In many conventional engineering organizations:

```text
Customer / Business
        ↓
Product Manager
        ↓
Ticket / Requirement
        ↓
Engineer
        ↓
Code
```

The engineer gets something relatively structured.

For example:

> Add a refund button to `/orders/:id` when order age < 7 days.

The developer implements it.

---

An FDE may instead receive:

> "Our customers complain that refunds are confusing."

Now the FDE needs to discover:

- What exactly is confusing?
- Which users?
- Which products?
- How often?
- What is the current workflow?
- Which systems contain refund information?
- What policy constraints exist?
- Should we change UI?
- Should we automate it?
- Should an LLM be involved?
- Should humans remain in the loop?

That's a very different job.

---

# 11. Meaning of "Forward Deployed"

Here, **deployed** does NOT mean:

```text
deploy application → server
```

Instead the engineer is metaphorically "deployed" close to the problem.

You work near:

- customer
- client
- operational teams
- management
- actual workflow

Instead of being isolated from the business problem.

---

# 12. Lecture's FDE definition

The central idea is approximately:

> A Forward Deployed Engineer works closely with customers, clients, departments and operational teams, observes the actual workflow, understands constraints, builds a solution, and helps put it into real use.

Notice all the verbs:

```text
observe
understand
frame
design
build
deploy
iterate
```

An FDE is therefore not simply:

```text
coder
```

It is closer to:

```text
Engineer
+
Product thinker
+
Problem solver
+
Customer-facing technologist
+
System integrator
```

---

# PART II — SOLVING THE CUSTOMER-SUPPORT PROBLEM

# 13. Solution 1: Rule-Based AI

Before jumping to an LLM, the lecture tries progressively stronger approaches.

The first idea:

> Why not create a rule-based chatbot?

Example:

```text
IF message contains "order status"
    show order tracking

IF message contains "refund"
    show refund policy

IF message contains "cancellation"
    show cancellation policy
```

This is extremely easy to build.

---

# 14. Rule-based chatbot architecture

Conceptually:

```text
User Query
   ↓
Keyword / Rule Matching
   ↓
Matching action
   ↓
Predefined response
```

Example:

```text
"order status"
      ↓
ORDER_STATUS rule
      ↓
Open tracking page
```

---

# 15. Why rule-based systems initially look intelligent

Suppose you type:

> What is my **order status**?

The system detects:

```text
"order status"
```

and redirects you to tracking.

From the user's perspective:

> Wow, the chatbot understood me!

But internally:

```python
if "order status" in query:
    open_tracking_page()
```

There is no deep language understanding.

---

# 16. The major weakness: exact wording

Consider:

### Query A

> What is my order status?

Keyword exists:

```text
order status
```

Works.

But:

### Query B

> What is the status of my order?

Same meaning.

Different exact word arrangement.

Or:

> Where has my package reached?

Or:

> Why hasn't my package arrived?

Or:

> मेरा ऑर्डर कहाँ है?

All are conceptually related to:

```text
ORDER STATUS
```

But literal keyword matching may fail.

---

# 17. Human language has combinatorial variation

This is one of the lecture's central themes.

A human can express one intention in hundreds of forms.

For the intent:

```text
Order tracking
```

possible sentences include:

- Where is my order?
- Where's my package?
- Has it shipped?
- When will my package arrive?
- Why hasn't it arrived?
- Track my parcel.
- What's happening with my delivery?
- मेरे ऑर्डर का क्या हुआ?
- मेरा पैकेज कहाँ पहुँचा?
- Bro where is my order 😭

The underlying **intent** can remain the same while surface wording changes dramatically.

---

# 18. Rule explosion

You might respond:

> Fine. I'll create more rules.

Then you add:

```text
"where is my order"
"track order"
"track package"
"package status"
"shipment status"
"delivery status"
...
```

But new variations keep appearing.

This creates:

```text
Rule
Rule
Rule
Rule
Rule
Rule
Rule
...
```

Eventually you get an enormous:

> **if-else ladder**

And still cannot cover human language.

---

# PART III — RULE-BASED ARTIFICIAL INTELLIGENCE

# 19. Early AI and predefined rules

The lecture now connects this customer-support example with AI history.

Early AI systems often tried to represent intelligence using explicit logical rules.

Example:

```text
IF age < 18
    NOT ELIGIBLE TO VOTE
ELSE
    ELIGIBLE TO VOTE
```

Technically, this system performs a decision.

But is it "intelligent"?

That depends on your definition of intelligence.

---

# 20. Facts + rules

Rule-based AI can be understood as two pieces.

### Facts

Example:

```text
Order delivered 3 days ago.
Order is damaged.
Order is returnable.
```

### Rules

```text
IF
    delivered within 7 days
AND damaged
AND returnable

THEN
    eligible for refund
```

Together:

```text
Facts
  +
Rules
  ↓
Decision
```

---

# 21. Apparent reasoning

To the customer:

> "Your damaged item was delivered three days ago and qualifies for a refund."

This can look like reasoning.

But internally it may simply be:

```text
A = true
B = true
C = true

IF A AND B AND C
    refund = allowed
```

The system behaves intelligently without necessarily possessing what humans would call understanding.

This idea connects directly to Alan Turing.

---

# PART IV — ALAN TURING AND MACHINE INTELLIGENCE

# 22. The foundational question

The lecture traces AI back to Alan Turing around 1950.

The central philosophical question:

> **Can machines think?**

The problem is that the word:

```text
think
```

is incredibly hard to define.

Likewise:

```text
intelligence
consciousness
understanding
emotion
thought
```

are difficult to precisely define.

---

# 23. Traditional computers vs human intelligence

Traditional computing is usually built around:

```text
Clear Input
+
Precisely Defined Instructions
+
Deterministic Processing
=
Expected Output
```

Example:

```text
2 + 2 = 4
```

Every time.

---

Programming works similarly.

We explicitly define:

- variables
- loops
- conditionals
- functions
- data structures
- algorithms

The computer follows instructions.

---

# 24. Human intelligence is different

Humans can:

- understand ambiguous language
- understand emotions
- recognize sarcasm
- reason under uncertainty
- work with incomplete information
- solve unfamiliar problems
- tell stories
- write poems
- make decisions
- adapt
- infer intentions
- understand context

We cannot easily write:

```text
if sarcasm:
    ...
else:
    ...
```

for every possible human interaction.

---

# 25. Turing's shift in perspective

Instead of trying to answer:

> "What exactly is thinking?"

Turing proposed something behavior-oriented.

Roughly:

> Can a machine communicate so convincingly that a human cannot reliably distinguish it from another human?

This becomes the conceptual basis of the:

# Turing Test

---

# 26. Turing Test mental model

Imagine:

```text
Room A → Human

Room B → Machine

        ↑
    Evaluator
```

The evaluator communicates through text.

The evaluator doesn't know which participant is the human.

If the evaluator cannot reliably distinguish them:

```text
Machine passes the test
```

---

# 27. Intelligence vs imitation of intelligence

This produces a deep philosophical question.

Suppose a machine:

- doesn't feel sadness
- doesn't experience happiness
- isn't conscious

but responds perfectly as though it does.

Is it intelligent?

The lecture repeatedly emphasizes the concept of:

> **mimicking intelligence**

Modern language models can produce behavior that sounds human-like even though that does not mean they experience human emotions.

---

# 28. Convincing ≠ correct

This point is EXTREMELY important for modern LLM engineering.

A human can give a:

```text
convincing answer
```

without giving a:

```text
correct answer
```

Likewise an AI system can generate:

> Extremely fluent, confident, convincing text

that is wrong.

This becomes directly relevant later when the lecture discusses LLM hallucination-like behavior.

---

# PART V — ELIZA

# 29. Early chatbot example: ELIZA

The lecture gives ELIZA as an example.

User:

> I am feeling unhappy.

ELIZA:

> Why are you feeling unhappy?

User:

> My friends do not understand me.

ELIZA:

> Why do you think your friends do not understand you?

User:

> They never listen to me.

ELIZA:

> Can you tell me more about them?

This sounds surprisingly conversational.

---

# 30. But how did ELIZA work?

Rule patterns.

For instance:

```text
Input pattern:
"I am feeling X"

Response:
"Why are you feeling X?"
```

Another possible pattern:

```text
Input:
"My X does Y"

Response:
"Why do you think your X does Y?"
```

So ELIZA could create the **appearance** of understanding through clever text manipulation.

---

# 31. Lesson from ELIZA

Do not judge the internal intelligence of a system purely from the user's emotional experience of interacting with it.

A relatively simple program can sometimes create:

```text
complex-looking behavior
```

This is a recurring theme across AI.

---

# PART VI — MACHINE LEARNING

# 32. Moving beyond manually written rules

The major insight behind Machine Learning is:

> Instead of explicitly programming every rule, allow the system to learn patterns from examples.

Rule-based approach:

```text
Human discovers pattern
        ↓
Human codes pattern
        ↓
Machine executes it
```

Machine-learning approach:

```text
Human provides examples/data
        ↓
Algorithm learns pattern
        ↓
Model predicts on new data
```

This is a massive conceptual shift.

---

# 33. Dog vs cat example

Suppose you want a computer to identify dogs.

Rule-based approach might say:

```text
Dog:
4 legs
fur
2 ears
tail
...
```

Problem:

Cats have those.

Wolves have those.

Many animals do.

Then you add more rules.

And more.

And more.

Eventually it becomes extremely difficult.

---

# 34. Machine-learning approach

Instead, provide:

```text
1000 dog images → DOG
1000 cat images → CAT
```

The model trains on examples.

Then show:

```text
new image
```

and ask:

```text
Dog or Cat?
```

The learned model predicts.

Example:

```text
P(Dog) = 0.93
P(Cat) = 0.07
```

---

# 35. Training data vs test data

### Training Data

Data used to help the model learn.

```text
Images + labels
```

### Test Data

Previously unseen data used to evaluate whether the learned patterns generalize.

```text
New dog photo
        ↓
model
        ↓
93% dog
```

---

# 36. Supervised Learning

In the lecture's simplified framing:

```text
Image → "DOG"
Image → "CAT"
```

The label is provided.

Therefore:

> **Supervised learning**

Humans supervise the learning process through labelled examples.

---

# 37. Unsupervised Learning

Now imagine giving the model thousands of images without labels.

No:

```text
dog
cat
horse
```

Instead the model discovers similarities.

Conceptually:

```text
Images
 ↓
Model
 ↓
Cluster 1
Cluster 2
Cluster 3
```

Later we may interpret cluster 1 as dogs, cluster 2 as cats, etc.

---

# 38. What is a model?

The instructor gives an extremely useful developer-friendly mental model:

> Think of a model as a function.

```text
input
  ↓
MODEL
  ↓
output
```

For classification:

```text
image
  ↓
model
  ↓
dog probability
```

For an LLM:

```text
prompt
  ↓
model
  ↓
response
```

This mental model is worth remembering.

---

# PART VII — WHY MACHINE LEARNING WAS NOT ENOUGH

# 39. Feature engineering

Traditional ML often requires humans to identify useful features.

For an image:

Computer initially sees numerical pixel information.

Humans may need to identify useful characteristics.

Conceptually:

```text
Raw Image
   ↓
Feature extraction
   ↓
ML algorithm
   ↓
Prediction
```

This is called:

> **Feature Engineering**

---

# 40. Why feature engineering is difficult

Suppose you want to recognize faces.

Possible features might involve:

- edges
- dark/light regions
- geometric patterns
- distances
- shapes

But deciding the correct features is hard.

And human assumptions can be wrong.

Therefore your model quality can become limited by:

```text
quality of human-designed features
```

---

# 41. Scaling problem

A classifier trained to distinguish:

```text
Dog vs Cat
```

does not automatically become:

```text
Human face recognizer
```

You need additional:

- data
- training
- features
- models

This creates scalability problems when trying to build general intelligence.

---

# PART VIII — NEURAL NETWORKS

# 42. The next conceptual leap

Suppose even humans cannot easily describe the exact pattern.

Instead of saying:

> "Here are the features."

Can the machine discover complex features and relationships itself?

This leads to:

> **Neural Networks**

---

# 43. Support-ticket urgency example

Suppose we want to predict:

```text
Is support ticket urgent?
```

Potential signals:

```text
x₁ = message contains "urgent"
x₂ = payment failed
x₃ = customer contacted multiple times
x₄ = customer account locked
```

But how much should each variable matter?

Maybe:

```text
payment failure = very important
word "urgent" = moderately important
repeat contact = strong signal
locked account = very strong signal
```

We don't initially know.

---

# 44. Weighted combination mental model

The lecture introduces something roughly like:

```math
z = w_1x_1 + w_2x_2 + w_3x_3 + w_4x_4
```

Where:

```text
x₁, x₂, x₃, x₄ = input signals
w₁, w₂, w₃, w₄ = learned importance/weights
```

Training tries to discover good values for:

```text
w₁
w₂
w₃
w₄
```

---

# 45. Why weights matter

Imagine:

```text
x₁ = uses word "urgent"
x₂ = payment failed
x₃ = contacted support 5 times
x₄ = account locked
```

Maybe the learned system discovers:

```text
w₁ = 0.1
w₂ = 0.7
w₃ = 0.8
w₄ = 0.9
```

Meaning:

Simply writing "urgent" is not necessarily highly important.

But:

```text
account locked
```

may be extremely predictive of urgency.

The machine learns this relationship from examples.

---

# 46. Neural network structure

Conceptually:

```text
Input Layer
    ↓
Hidden Layer
    ↓
Hidden Layer
    ↓
Hidden Layer
    ↓
Output Layer
```

Each neuron receives values, transforms them, and passes information forward.

A neural network may contain huge numbers of:

- neurons
- connections
- weights
- layers

---

# 47. Deep Learning

When neural networks have multiple layers that learn increasingly rich representations, we commonly discuss:

> **Deep Learning**

The lecture gives the broad hierarchy:

```text
Artificial Intelligence
        ↓
Machine Learning
        ↓
Neural Networks / Deep Learning
```

Modern LLMs are built using deep neural networks.

---

# PART IX — COMPUTER VISION GOT STRONG EARLY

# 48. Neural networks and images

The lecture points out that neural networks became extremely successful for images.

A famous architecture:

> CNN — Convolutional Neural Network

Applications mentioned include:

- face recognition
- automatic photo tagging
- self-driving perception
- image classification

So image understanding became very powerful.

But language remained unusually difficult.

---

# PART X — WHY HUMAN LANGUAGE IS SO HARD

This is one of the most important sections.

# 49. Language isn't simply "words"

To understand language, a model may need to understand:

```text
words
+
word order
+
grammar
+
context
+
intention
+
tone
+
culture
+
previous conversation
+
references
+
negation
+
sarcasm
```

That's enormously complicated.

---

# 50. Same meaning, different sentences

Example:

> Where is my order?

and:

> Track my package.

Different words.

Essentially same customer intention.

The model has to understand meaning, not just literal strings.

---

# 51. Word order changes meaning

Example:

> Dog chased the man.

vs

> Man chased the dog.

Same words.

Completely different meaning.

Therefore language is not a:

```text
bag of independent words
```

Order matters.

---

# 52. Negation changes meaning

Compare:

> I am hungry.

and:

> I am not hungry.

One word:

```text
not
```

reverses the meaning.

---

# 53. Questions change meaning

Compare:

```text
I am hungry.
```

and:

```text
Am I hungry?
```

Same concepts.

Different communicative intent.

---

# 54. Pronouns and references

Consider the lecture's example idea:

> Aditya placed the laptop on the table because **it** was heavy.

What does:

```text
it
```

refer to?

A human resolves it using context.

The AI system needs a mechanism for connecting:

```text
it ↔ laptop
```

This is called a reference/coreference-type problem.

---

# 55. Sarcasm

Customer says:

> Very nice service! My order hasn't arrived for 15 days.

Literal interpretation:

```text
customer likes service
```

Actual interpretation:

```text
customer is angry
```

A keyword-based sentiment system might badly misunderstand this.

---

# 56. Language changes over time

New:

- slang
- abbreviations
- emojis
- phrases
- internet expressions

continuously emerge.

Therefore language is productive and dynamic.

---

# PART XI — STATISTICAL LANGUAGE MODELS

# 57. Predicting language statistically

An early approach to language modelling was based on:

> What word is likely to come next?

Suppose training data contains:

```text
I like machine learning.
I like Java programming.
Students like machine learning.
```

After:

```text
like
```

we have observed:

```text
machine → 2 times
Java → 1 time
```

Therefore:

```math
P(machine \mid like) = \frac{2}{3}
```

```math
P(Java \mid like) = \frac{1}{3}
```

So given:

```text
Rohan likes ...
```

the model may choose the more probable continuation.

---

# 58. Core insight

Language generation can partially be framed as:

> **predict the next linguistic unit based on previous context**

This idea continues into modern language models, although modern implementations are vastly more sophisticated.

---

# PART XII — N-GRAMS

# 59. Why one previous word isn't enough

Suppose you only examine:

```text
previous word
```

Context is tiny.

Instead examine:

```text
previous 2 words
previous 3 words
previous 4 words
...
```

These are n-gram-style models.

---

# 60. Unigram

Very little context.

Conceptually considering individual words.

---

# 61. Bigram

Two-word sequence/context.

Example:

```text
machine learning
Java programming
```

---

# 62. Trigram

Three-word sequence.

Example:

```text
I like Java
```

Increasing `n` increases local context.

---

# 63. But n-grams still fail at scale

Imagine you've learned:

```text
"I want a refund"
```

But customer says:

> I want reimbursement.

Same intention.

Different word.

Or:

> Can you return my money?

Or Hindi.

Or slang.

Statistical surface-level word patterns struggle to capture the richness of meaning.

Therefore we needed more powerful sequence models.

---

# PART XIII — RNNs

# 64. Sequence processing idea

The lecture next discusses RNN-type models.

Take:

> The payment has not been refunded.

The model processes approximately:

```text
"The"
 ↓
update internal state

"payment"
 ↓
update internal state

"has"
 ↓
update internal state

"not"
 ↓
update internal state

...
```

The internal state carries some information about previously processed words.

---

# 65. Sticky-note analogy

This is an excellent mental model from the lecture.

Imagine reading a novel.

After several pages, you write a tiny sticky note summarizing everything you've read.

Then read more pages.

Update the sticky note.

Then continue.

Eventually:

```text
original details
        ↓
compressed repeatedly
        ↓
information lost
```

The internal state of a traditional recurrent sequence model can be thought of similarly.

---

# 66. Long-range context problem

Suppose something important appeared very early in a long paragraph.

As the model processes:

```text
word 1
word 2
word 3
...
word 500
...
```

earlier information can become difficult to preserve.

Therefore long sequences historically created problems.

---

# 67. RNN text generation

Given:

> The dog barked at the ...

the model uses its learned patterns and internal state to predict something plausible.

Maybe:

```text
delivery boy
```

Then it continues.

But generating long coherent text is harder because context degrades.

---

# PART XIV — TRANSFORMERS

# 68. 2017: "Attention Is All You Need"

The lecture presents 2017 as the major turning point.

The paper:

> **Attention Is All You Need**

introduced the Transformer architecture.

This changed how language could be processed.

---

# 69. Why RNN-style processing is limiting

RNN mental model:

```text
token 1
 ↓
token 2
 ↓
token 3
 ↓
token 4
 ↓
...
```

Sequential processing creates:

- difficulty parallelizing
- long-context problems
- training inefficiencies

Transformers introduced a fundamentally different mechanism.

---

# 70. Attention intuition

Suppose:

> Aditya gave Rohit the laptop because he needed it for work.

To understand:

```text
he
```

the system should consider other words.

Possibilities include:

```text
Aditya
Rohit
laptop
```

Attention asks roughly:

> Which other pieces of the sentence matter most for interpreting this token?

---

# 71. Relevance relationships

Conceptually:

```text
he ↔ Aditya      HIGH?
he ↔ Rohit       HIGH?
he ↔ gave        LOW
he ↔ laptop      LOW/MEDIUM
```

Similarly:

```text
it ↔ laptop      HIGH
```

Attention learns these relationships from data rather than requiring humans to explicitly encode:

```python
if token == "it":
    find_previous_noun()
```

---

# 72. The fundamental attention question

For every token:

> **Where should I look to understand this token?**

That's a very useful intuition.

Instead of only using:

```text
previous compressed state
```

the Transformer allows tokens to relate more directly to other tokens in the context.

---

# 73. Why Transformers were transformational

According to the lecture's framing, Transformers improved:

### Context handling

They could model relationships between words much more effectively.

### Parallelism

Training can operate much more efficiently than strict recurrent word-by-word processing.

### Scale

We could train models on dramatically larger corpora.

These factors helped make modern large language models practical.

---

# PART XV — LARGE LANGUAGE MODELS

# 74. What is an LLM?

LLM:

> **Large Language Model**

The lecture's intuitive explanation:

These are language models trained on extremely large amounts of text using modern neural-network architectures such as Transformers.

Think:

```text
Massive text corpus
       ↓
Training
       ↓
Huge neural network
       ↓
Language model
```

---

# 75. Why "Large"?

"Large" can intuitively refer to scale such as:

- huge training datasets
- many parameters
- large compute requirements
- broad language capability

The course promises to explore these concepts later.

---

# PART XVI — GPT

# 76. Meaning of GPT

GPT =

> **Generative Pre-trained Transformer**

Each word matters.

---

## G — Generative

It generates content.

Traditional classifier:

```text
Email
 ↓
Spam / Not Spam
```

Generative model:

```text
Prompt
 ↓
Model
 ↓
New text
```

It produces a response rather than merely assigning a category.

---

## P — Pre-trained

The model has already been trained on large amounts of data before you start interacting with it.

You don't personally train the entire model every time you write:

> Explain Docker.

The foundational learning happened beforehand.

---

## T — Transformer

The model architecture is based on Transformer technology.

Hence:

```text
Generative
+
Pre-trained
+
Transformer
=
GPT
```

---

# PART XVII — BACK TO THE E-COMMERCE CHATBOT

Now the whole AI-history discussion loops back to the FDE problem.

---

# 77. Solution 2: Machine-learning classification

Instead of exact keyword rules, train a classifier.

Possible intents:

```text
ORDER_STATUS
REFUND
CANCELLATION
DAMAGED_PRODUCT
```

Training examples:

```text
"Where is my order?"          → ORDER_STATUS
"Why hasn't it arrived?"      → ORDER_STATUS
"मेरा ऑर्डर कहाँ है?"        → ORDER_STATUS

"I want my money back."       → REFUND

"Cancel my order."            → CANCELLATION

"My product arrived broken."  → DAMAGED_PRODUCT
```

The model learns patterns.

---

# 78. This is much better than keyword rules

Now:

> Why hasn't my parcel reached me?

can potentially classify as:

```text
ORDER_STATUS
```

even without exact phrase:

```text
order status
```

Excellent.

But another problem remains.

---

# 79. Classification doesn't naturally generate a rich response

Classifier outputs:

```text
ORDER_STATUS
```

It doesn't necessarily generate:

> Your order was dispatched yesterday and should arrive within three days.

You could map:

```text
ORDER_STATUS
        ↓
tracking page
```

But you still lack a naturally generated conversational answer.

Therefore we move toward LLMs.

---

# PART XVIII — SOLUTION 3: DIRECTLY USE AN LLM

# 80. Why an LLM seems perfect

LLMs can:

- understand varied language
- understand intent
- generate natural responses
- respond conversationally
- handle multiple phrasings

So simply integrate ChatGPT/Gemini/Claude/etc.?

Not so fast.

---

# 81. Critical production problem: the LLM doesn't know your company

Customer asks:

> Can I return my order after 15 days?

Suppose generic LLM responds:

> Yes, returns are allowed within 30 days.

But your actual company's policy is:

```text
7 days
```

Disaster.

---

# 82. Why did the LLM answer incorrectly?

Because the LLM may have general knowledge about common return policies.

But it doesn't automatically know your company's:

- current return policy
- refund rules
- cancellation policy
- internal databases
- order status
- customer account
- past conversation

The missing ingredient is:

# Context

---

# 83. General model knowledge vs company-specific knowledge

Think:

```text
LLM
knows general patterns
```

But company-specific answer requires:

```text
LLM
+
company information
```

---

# 84. Better architecture

Instead of:

```text
User Query
   ↓
LLM
   ↓
Answer
```

we want something closer to:

```text
User Query
       +
Company Policies
       +
User Order Status
       +
Previous Conversation
       ↓
      LLM
       ↓
Context-aware response
```

This is foundational to modern enterprise GenAI.

---

# 85. Example

Customer asks:

> Can I return my order after 15 days?

Context says:

```text
Company return policy = 7 days
Order delivered = 10 days ago
Product type = electronics
```

Now the model can respond based on company information instead of inventing a generic answer.

---

# 86. THIS is where the FDE becomes important

The job wasn't:

> Call OpenAI API.

Any developer can technically write:

```python
client.chat.completions.create(...)
```

The hard engineering problem is:

```text
Which data should be sent?
Where does it come from?
Is it trustworthy?
Is it current?
Who has access?
What happens if information is missing?
When should humans take over?
What actions can AI perform?
How do we prevent incorrect responses?
How do we measure accuracy?
How do we maintain customer trust?
```

That is much closer to real Forward Deployed Engineering.

---

# PART XIX — THE FDE MINDSET HIDDEN INSIDE THIS ENTIRE LECTURE

The lecture is secretly teaching a powerful hierarchy:

```text
DON'T START WITH TECHNOLOGY.
START WITH THE PROBLEM.
```

Management:

> We need an AI chatbot.

FDE:

> Why?

Management:

> Support is expensive and slow.

FDE:

> Which queries are repetitive?

Management:

> Order tracking/refunds/etc.

FDE:

> What level of accuracy is required?

Management:

> Very high.

FDE:

> What company data is required?

Management:

> Orders, policies, user history.

Only now should engineering design start.

---

# 87. Solution ladder

The lecture effectively demonstrates:

```text
Problem
   ↓
Can simple rules solve it?
   ↓ no
Can classification solve it?
   ↓ partially
Do we need generation?
   ↓ yes
Use LLM
   ↓
Does generic LLM know business data?
   ↓ no
Provide company context
   ↓
Production-oriented solution
```

This is an outstanding engineering mindset.

You don't begin with:

```text
Agents!
RAG!
Vector DB!
MCP!
Kafka!
Kubernetes!
```

You progressively introduce complexity only when the problem requires it.

---

# PART XX — THE EVOLUTION OF AI AS ONE STORY

You can memorize Lecture 1 using this single progression:

```text
1950
Alan Turing
"Can machines think?"
        ↓
Turing Test
        ↓
Artificial Intelligence becomes a field
        ↓
Rule-Based AI
        ↓
Too many rules
        ↓
Machine Learning
"Learn patterns from data"
        ↓
Feature engineering / limited specialization
        ↓
Neural Networks
"Learn complex patterns"
        ↓
Deep Learning
        ↓
Computer Vision success
        ↓
Language still difficult
        ↓
Statistical Language Models
        ↓
N-Grams
        ↓
Sequence models / RNNs
        ↓
Long-context limitations
        ↓
2017 Transformers
"Attention Is All You Need"
        ↓
Scale language modelling
        ↓
Large Language Models
        ↓
GPT / modern GenAI
        ↓
Enterprise AI systems
        ↓
Forward Deployed Engineer
```

---

# PART XXI — MOST IMPORTANT CONCEPTUAL DISTINCTIONS

## 1. Business problem vs technology requirement

Wrong:

```text
Need chatbot
```

Better:

```text
Need faster, reliable customer-resolution system
```

---

## 2. Rule vs learning

Rule-based:

```text
Human writes patterns
```

ML:

```text
Model learns patterns from examples
```

---

## 3. Classification vs generation

Classification:

```text
"This message belongs to Refund category."
```

Generation:

```text
"Your refund request has been received and should be processed..."
```

---

## 4. Generic knowledge vs private context

Generic LLM:

```text
Knows broad patterns
```

Enterprise AI:

```text
LLM
+
your company's actual data
```

---

## 5. Fluent vs correct

Never equate:

```text
Confident language
```

with:

```text
Correct answer
```

This is one of the most important lessons for an FDE.

---

# PART XXII — TECHNICAL CLARIFICATIONS

The following points are **my technical clarifications, not claims made exactly as stated in the transcript**. I'm separating them so the lecture notes remain faithful.

### 1. RNN usually means Recurrent Neural Network

In NLP discussions:

> RNN = **Recurrent Neural Network**

rather than "Recursive Neural Network."

Recursive neural networks do exist, but are a different concept.

---

### 2. ChatGPT was not the first LLM

The lecture uses ChatGPT as the major popular breakthrough.

Historically, large language models and earlier GPT models existed before ChatGPT.

ChatGPT's public release dramatically popularized conversational LLMs.

---

### 3. Deep Learning is not separate from Machine Learning

A better hierarchy is:

```text
Artificial Intelligence
└── Machine Learning
    └── Deep Learning
        └── Neural-network architectures
            └── Transformers
                └── Many modern LLMs
```

---

### 4. Transformers don't literally have unlimited context

They can model relationships across tokens much more directly than classic RNNs, but actual Transformer models still have a finite:

> **context window**

You'll likely study this later in the course.

---

### 5. Attention does more than pronoun resolution

The lecture's:

```text
he → Aditya/Rohit
it → laptop
```

example is a good intuition.

But mathematically, attention is a much broader mechanism for dynamically combining information across token representations.

You'll eventually encounter concepts such as:

```text
Query
Key
Value
Attention Score
Softmax
Multi-Head Attention
```

Those don't need to be mastered yet.

---

# PART XXIII — WHAT YOU SHOULD PERSONALLY LEARN FROM LECTURE 1 AS AN ASPIRING FDE

This part matters more than memorizing the history.

### Habit 1 — Question the requirement

Someone says:

> Build RAG.

Ask:

> What problem requires RAG?

Someone says:

> Build an AI agent.

Ask:

> What task needs autonomous tool use?

Someone says:

> Use Kafka.

Ask:

> What communication/scale/reliability problem requires Kafka?

---

### Habit 2 — Start with the simplest architecture

Always mentally try:

```text
Simple function?
        ↓
Rules?
        ↓
Database query?
        ↓
Traditional ML?
        ↓
LLM?
        ↓
RAG?
        ↓
Tools?
        ↓
Agent?
        ↓
Distributed architecture?
```

Don't jump directly to the bottom.

---

### Habit 3 — Think about the user's workflow

Don't merely ask:

> What API should I build?

Ask:

```text
Who currently performs this task?
What do they click?
What information do they inspect?
Where does that information live?
Which decisions do they make?
Which decisions are repetitive?
Which decisions require humans?
What can go wrong?
```

That is FDE thinking.

---

### Habit 4 — Treat AI as one component of a system

A real production chatbot isn't:

```text
LLM API = Product
```

It is something closer to:

```text
Frontend
    ↓
Backend
    ↓
Authentication
    ↓
User Query
    ↓
Relevant company data
    ↓
Policy retrieval
    ↓
Order system
    ↓
Conversation history
    ↓
LLM
    ↓
Validation / Guardrails
    ↓
Actions / Tools
    ↓
Logging
    ↓
Monitoring
    ↓
Human escalation
```

This is why your interests in **backend engineering + cloud + AI** fit extremely naturally with Forward Deployed Engineering: the role sits right at the intersection of systems, business workflows, APIs/data, and applied AI.

---

# PART XXIV — 20 QUESTIONS YOU SHOULD BE ABLE TO ANSWER AFTER LECTURE 1

Don't memorize answers blindly. Try answering these yourself.

1. What is a Forward Deployed Engineer?
2. Why is "build an AI chatbot" not a proper business requirement?
3. What is the actual business objective in the e-commerce example?
4. What is the difference between an FDE and an engineer receiving a predefined ticket?
5. Why can rule-based AI appear intelligent?
6. Why does rule-based AI struggle with human language?
7. What was ELIZA?
8. What philosophical problem does the Turing Test address?
9. Why doesn't convincing output guarantee correctness?
10. What is the fundamental difference between rule-based AI and Machine Learning?
11. What is training data?
12. What is supervised learning?
13. What is unsupervised learning?
14. What is feature engineering?
15. Why are neural networks useful?
16. What are weights conceptually?
17. Why is natural language particularly difficult?
18. What is an n-gram model?
19. Why do traditional recurrent models struggle with long sequences?
20. What conceptual innovation did attention/Transformers provide?

And the most important FDE question:

> **Why shouldn't you directly integrate a generic LLM into a company's customer-support system and trust whatever it generates?**

If you deeply understand that answer, you've understood the central thread of Lecture 1.

---

# Final mental model

Keep this picture in your head:

```text
                  FORWARD DEPLOYED ENGINEER

Customer says:
"We need an AI chatbot."
            │
            ▼
      Don't start coding
            │
            ▼
   Understand the workflow
            │
            ▼
   Identify actual problem
            │
            ▼
"Reduce resolution time
while preserving accuracy
and customer trust."
            │
            ▼
      Explore solutions
            │
    ┌───────┴────────┐
    ▼                ▼
 Rule-based        ML
    │                │
    └──── fails/limited
            │
            ▼
          LLM
            │
            ▼
 Generic LLM lacks
 company information
            │
            ▼
 Add:
 policies
 orders
 history
 user context
            │
            ▼
   Reliable AI system
            │
            ▼
      Production use
```

## The deepest takeaway of Lecture 1

**Forward Deployed Engineering is not about knowing the coolest AI tool.**

It is about being capable of moving from:

> **messy real-world problem**

to:

> **clearly defined problem**

to:

> **appropriate technical architecture**

to:

> **working production system**

while understanding enough **backend, data, AI, users, business constraints and deployment** to make the right engineering choices.

And that is why the lecturer begins the FDE course with the evolution from **rules → ML → deep learning → NLP → Transformers → LLMs** rather than immediately teaching an LLM API. The goal is to make you understand **why each abstraction exists and what problem it solved**, rather than merely learning how to call it.