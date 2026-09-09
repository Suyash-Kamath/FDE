# Forward Deployed Engineer — Lecture 1
## Introduction to FDE + The Origin Story of AI (Rule-based → ML → Neural Nets → Transformers → LLMs)

**Series:** Forward Deployed Engineer (Coder Army)
**Lecture goal:** Understand (a) what a Forward Deployed Engineer actually *is* and *does*, and (b) how the world arrived at today's LLM era — from first principles.
**Prerequisites:** Any one programming language (C++ / Java / Python / JavaScript). Basic backend/frontend awareness helps but is optional. **No ML prerequisite** — ML is covered only intuitively, because the target is the GenAI layer, not model training.

---

# PART 0 — How to read this lecture

Two halves, and they are deliberately joined:

| Half | Question it answers |
|---|---|
| Half 1: The FDE role | *What is my job when a company says "build us an AI thing"?* |
| Half 2: History of AI | *Why is an LLM the answer today, and why wasn't it the answer in 1990?* |

The reason the history matters for an FDE: your job is to **choose the right technique for the actual problem**. If you don't know that rule-based systems, classifiers, and generative models exist as separate tools with separate failure modes, you'll reach for an LLM every time — including where it's the wrong, expensive, or dangerous choice.

---

# PART 1 — What is a Forward Deployed Engineer? (The e-commerce case study)

## 1.1 The scenario

An e-commerce company has a large active user base. Every day, users raise **support queries** — the lecture uses the figure of ~50,000 queries/day.

> *Note: the transcript says "500 users" once early on and "50,000" thereafter. Treat 50,000/day as the working number — the point is "volume far beyond what humans can absorb."*

Typical queries:
- "Where is my order?"
- "I received the wrong product."
- "Can I return my product after 15 days?"
- "I placed an order 3 days ago, why no shipment mail yet?"

**Current setup:** a large team of **support executives** (humans) answering these.

## 1.2 The vague requirement handed to you

Management says:

> *"We get ~50,000 support queries a day. Humans handle them today. Your job: integrate an AI chatbot that resolves all user queries."*

An FDE's very first move is **not** to start coding. It is to ask:

> **Does this company literally want an AI chatbot?**

**Answer: No.** The company cares about a **business outcome**, not a technology. "AI chatbot" is a *vague statement*, not a requirement. It doesn't tell you which features are needed, what "resolve" means, what accuracy is acceptable, or what happens on failure.

This is the single most important idea in the lecture: **a raw requirement is a symptom; you have to find the disease.**

## 1.3 The real pain points (dig these out by talking to the business)

1. **Waiting queues.** Even with 100 executives, if all 100 are on calls, everyone else waits. Users hate this.
2. **Cost.** Salaries + infrastructure + training + attrition for a large support org.
3. **Repetitive/trivial work.** Most queries are low-value ("where is my order?") — the answer often already exists on the website. Humans burn their day on it.
4. **Important cases get buried.** When staff are drowning in trivial tickets, a genuinely critical case (fraud, damaged high-value order, payment failure) can get lost.
5. **Limited working hours.** Humans work shifts. Customer problems happen at 2 AM. A bot can be **24×7**.
6. **Trust.** Slow or wrong resolution damages customer trust, which is a revenue problem, not a support problem.

## 1.4 Reframing into a proper, ordered requirement

The FDE converts the vague ask into something engineerable:

> **"Reduce the time required to resolve customer problems, without reducing accuracy or customer trust."**

Notice the shape of a good requirement:
- **An objective** (reduce resolution time)
- **Hard constraints** (accuracy must not drop; trust must not drop)
- **Implied constraints** (24×7 availability; cost reduction; critical cases must not be buried)
- **Technology-neutral** — it does not say "chatbot", so it doesn't pre-commit you to a solution

> **Rule of thumb:** if the requirement names a technology, it's probably not a requirement yet.

## 1.5 Candidate Solution 1 — Rule-based AI chatbot

**How it works:** exact string / keyword matching on the user's query.

| If query contains | Then do |
|---|---|
| `order status` | Take user to the Order Tracking screen |
| `refund` | Display the Refund Policy |
| `cancellation` | Display the Cancellation Policy |

**Where you've already seen this:** chatbots with **no text box** — only a predefined set of buttons ("Order status", "Refund", "Cancellation"), each hard-wired to a canned answer.

**Why it's attractive:** trivially cheap, deterministic, fully auditable, zero hallucination risk.

**Why it fails:**

1. **User can't express intent.** With button-only bots the customer literally cannot say "my order arrived crushed." Highly dissatisfying → **breaks trust** (which was one of our constraints).
2. **Even with a free-text box, keyword matching breaks.** "What is my order status?" matches. But all of these mean the same thing and *don't* contain the exact key:
   - "What is the status of my order?"
   - "मेरे ऑर्डर का क्या स्टेटस है?" (another language)
   - "मेरा ऑर्डर कहां तक पहुंचा?"
   - "Why have I not received my order?"
   - "Track my package"
3. **The core insight about human language:** *one idea has infinitely many surface forms, and context changes meaning.* You can keep adding rules and handling edge cases forever — **you will always lose the race**. There will always be one more phrasing your ladder doesn't cover.

**Verdict:** fails on natural language. Rejected.

## 1.6 Candidate Solution 2 — A Machine Learning classifier

**Idea:** stop writing rules. Train a **classification model** on pre-labelled examples.

**Step 1 — define categories (intents):**
- `Order Status`
- `Refund`
- `Cancellation`
- `Damaged Product`

**Step 2 — give it training data** for each category (many real phrasings per intent):

```
Category: Order Status
  - "Where is my order"
  - "मेरा ऑर्डर कहां है"
  - "Why have I not received my order"
  - "Track my package"
  ... hundreds more
```

**Step 3 — at inference time**, a *new, unseen* query gets classified into one of the four buckets, and you take the corresponding action.

**Real-world analogue given in the lecture: spam filtering (Gmail).**
- A rule-based spam filter (`if contains "free money" or "$$" or "lottery" → spam`) is trivially defeated: spammers are smart, they change words, spellings, spacings.
- A classifier trained on **labelled data** ("these millions of mails are spam; these are not") learns the *shape* of spam and generalises to new mails it has never seen. It was one of the classic pre-GenAI student projects.

**What we gained:** we can now *understand* varied phrasings of user intent. Big step up from Solution 1.

**Why it still fails:** a classifier **classifies, it does not generate.**
- It can route you to the Order Tracking page or display the Refund Policy.
- It **cannot produce a human-sounding reply** such as: *"Your order has already been shipped and will be delivered within 3 days."*
- Classification gives you a *label*, on which you can base a decision. It does not give you *language*.

**Verdict:** understands intent, cannot converse. Rejected — because we wanted a good response, not just a redirect.

## 1.7 Candidate Solution 3 — Plug in a modern general-purpose LLM

**Idea:** integrate an off-the-shelf LLM (ChatGPT / Claude / Gemini / Grok / DeepSeek) directly.

> *Side note flagged in the lecture: ChatGPT, Claude, DeepSeek etc. are **not** "just an LLM" — they are products built around models (system prompts, tools, retrieval, safety layers, UI). For Lecture 1, treat them as "an LLM" and refine later in the series.*

**What we gain immediately:** fluent, natural, human-like generated responses to arbitrary phrasings. Exactly what Solution 2 lacked.

**And then the new, dangerous problem appears:**

```
User: "Can I return my order after 15 days?"
LLM : "Yes, sure! You can return your order within 30 days."
```

**But the company's real return window is 7 days.**

**Why did this happen?** A general-purpose LLM has **no knowledge of your company's internal details**:
- your return policy
- your refund policy
- your cancellation terms
- what your order-tracking screen shows
- this particular user's order state

**And here is the double-edged nature of LLMs:** they *always* produce an output. Ask a random question and you get a confident answer, whether or not it is correct. The model optimises for a **convincing** answer, not necessarily a **correct** one. (Hence the fine print under every such product: *"This model can make mistakes."*)

**Business consequence:** this is not a cosmetic bug. Tomorrow a customer says *"your own chatbot told me 30 days"* — and you now have a legal/commercial liability, refund losses, and a trust hit. Precisely the constraints we promised not to violate.

**Verdict:** great language, unacceptable grounding. Cannot be used raw.

## 1.8 Candidate Solution 4 (the answer) — LLM + Context

Stop sending the model only the user's query. Send the model everything it needs to answer *for your company*:

```
                 ┌───────────────────────────┐
 User Query ────►│                           │
 Company Policy ►│         LLM MODEL         │────► Grounded response
 Order Status ──►│                           │
 Prev. Convo ───►│                           │
                 └───────────────────────────┘
```

| Context piece | Why it's needed |
|---|---|
| **User query** | What is being asked (intent) |
| **Company policies** | Return = 7 days, refund rules, cancellation terms — prevents the 30-day hallucination |
| **User's order status** | So "where is my order" gets a *real* answer, not a generic one |
| **Previous conversation** | Continuity; don't make the user repeat themselves; understand pronouns/references |

Now the same query produces: *"Our return window is 7 days from delivery. Your order was delivered 15 days ago, so it's outside the return window — but here's what I can do…"*

**And that is the FDE's finished job:** a *vague ask* became a *framed requirement*, was matched against *multiple candidate techniques*, and ended as a **production-ready, grounded solution** that satisfies the business constraints.

> This "assemble the right context around the model" idea is the seed of what the rest of the series builds: prompting, context windows, RAG, tools/function calling, evaluation.

## 1.9 The FDE loop (memorise this)

```
1. Sit close to the customer / operations / management
2. Observe the actual workflow  (not the described workflow)
3. Extract the RAW requirement  ("give us an AI chatbot")
4. Convert it to an ORDERED requirement (objective + constraints, tech-neutral)
5. Enumerate candidate solutions, cheapest/simplest first
       rule-based → classical ML → LLM → LLM + context → agents/tools
6. Kill each one on its real failure mode, with reasons
7. Pitch the surviving solution
8. Build it, deploy it to production, and get it actually USED
```

Point 5 deserves emphasis: **start simple.** If a rule-based flow genuinely solves 80% of tickets at 1/100th the cost, that may be the right engineering answer for those 80%. The LLM is not a badge of honour.

---

# PART 2 — Official definition and the two strange words

## 2.1 The definition

> **A Forward Deployed Engineer works closely with the customer, client, department and operational teams. The engineer observes the actual workflow, understands the constraints, builds the solution — and helps put it into real use.**

## 2.2 What "Deployed" means here

**Not** the CS meaning (pushing an app to a server).

Here, **you** are the thing being deployed: an engineer is *placed into the environment where the real problem lives*. You sit with the customer, with ops, with management. You see the constraints with your own eyes instead of reading them in a ticket.

## 2.3 What "Forward" means here

In a normal product org:
- A **product person** decides *what* should be built and writes a detailed **ticket**.
- The **engineer** implements the ticket in code and ships it.

An FDE works **further forward in the pipeline** — before the ticket exists.

| | Normal SDE | Forward Deployed Engineer |
|---|---|---|
| Input | A fully specified ticket | A vague business complaint |
| Owns the problem definition? | No (product owns it) | **Yes** |
| Talks to end customers/ops? | Rarely | **Constantly** |
| Chooses the technique? | Usually pre-decided | **Yes, and must justify it** |
| Success measured by | Ticket closed | **Business outcome achieved & adopted** |
| Skills | Implementation | Implementation + problem framing + solution pitching + domain empathy |

## 2.4 Why FDE work is AI-heavy right now

Every company wants to automate something, onboard some new tool, or wire an LLM/SLM into an existing workflow. The bottleneck is rarely the model — it's someone who can translate messy business reality into a working, grounded AI system. That's the FDE.

---

# PART 3 — The origin of AI

To understand GenAI we have to go back and see *how we got here*.

## 3.1 The starting point: computers were deterministic calculators

Why were computers invented? **To compute.** It's in the name.

`2 + 2 = 4`. Always. We fed instructions; instructions had predefined outputs.

Then we built ladders of abstraction to give instructions more comfortably:
- binary → low-level languages → high-level languages (C++, Java, Python, JavaScript)
- concepts: variables, loops, conditionals (`if/else`), recursion
- entire disciplines: data structures & algorithms

**Everything in that world shared three properties:**

| Property | Meaning |
|---|---|
| **Input was clear** | Well-formed, typed, structured |
| **Instructions were precisely defined** | Every line has exact semantics |
| **Output was deterministic** | A predefined output existed; same input → same output |

Any language-level instruction works only because the machine can convert it to binary. **So: if we could express "human intelligence" as a precise set of instructions, we could give a computer intelligence too.**

**The blocker:** we can't. We have never managed to define human intelligence precisely — not even for ourselves.

## 3.2 Turing's question (1950)

> **"Can a machine perform a task that requires actual human intelligence?"** — Alan Turing, 1950

Remarkable that this was asked in **1950**, at the dawn of computing.

**Why it's hard: human intelligence is enormous and fuzzy.** We can:
- understand multiple languages
- detect **sarcasm**
- read **emotions** (angry? happy?)
- solve **unfamiliar** problems never seen before
- **make decisions from incomplete data**
- explain ideas, tell stories, write songs and poems, dance…

All of that lumped together, we call "intelligence."

## 3.3 "Can machines think?" → the definitional trap → the Turing Test

Turing's compressed version was: **Can machines think?**

But to answer it you must first define *think*, and *intelligence*. We can't. Worse:

> Even for humans, we never observe another person's **thoughts** directly. We observe their **behaviour**, and from behaviour we *perceive* how intelligent they are.

So Turing did something clever: he **abandoned the philosophical question and substituted an operational/mathematical one**:

> *Can a machine communicate so well that a human cannot tell it apart from another human?*

### The Turing Test (the two-rooms experiment)

```
      Room A                Room B
   ┌──────────┐         ┌──────────┐
   │ MACHINE  │         │  HUMAN   │
   └────┬─────┘         └────┬─────┘
        │  (text only)       │
        └────────┬───────────┘
                 │
           ┌─────▼──────┐
           │ EVALUATOR  │  (a human; does NOT know which room is which)
           └────────────┘
```

- Evaluator interrogates both, **via text only** (so voice/appearance can't give it away).
- If the evaluator **cannot reliably tell** which is the machine → the machine **passes the Turing Test** and we call it intelligent.

### The two ideas the Turing Test smuggles in

**1. Intelligence is defined by indistinguishable behaviour — even if it is only *mimicry*.**

However you personally define intelligence, this machine counts as intelligent, because you can't tell the difference. Today's LLMs sit exactly here: they **mimic** intelligence and mimic emotions. They don't *feel* your pain, but they can convince you they understand it — and plenty of people talk to them like friends. *They are not aware. They don't feel things. They just mimic it.*

The uncomfortable question: **if something mimics intelligence so well that you cannot create a difference, is it intelligent?** This is as philosophical as it is mathematical, and your answer depends entirely on your definition.

**2. Convincing ≠ correct.**

To fool the evaluator, both rooms must give *convincing* answers. Convincing does not mean *correct* — and this is a very human trait too (every viva/interview you've bluffed through). Today's LLMs inherit exactly this: fluent, confident, sometimes wrong.

*Watch:* the movie **The Imitation Game** is about Alan Turing.

## 3.4 1956 — "Artificial Intelligence" gets its name

**McCarthy and team** coined the term **Artificial Intelligence** at a workshop — the first time the phrase was used. That naming mattered: it gave AI the status of a **distinct field inside computer science**, and research boomed.

**Two readings of "artificial":**
1. **Not natural** — our intelligence came from nature; this one is manufactured.
2. **Fake** — as in "artificial Nike shoes" vs real ones.

Either way, **it's still intelligence** by the Turing-Test standard: if the mimicry is indistinguishable, the "fake" label stops doing useful work. Everything depends on how you define intelligence, thinking, and emotion.

---

# PART 4 — Era 1: Rule-based Artificial Intelligence

The very first and most basic form of AI. Everyone (including us, in Part 1) reaches for it first.

## 4.1 It really is just `if/else`

**Example A — voting eligibility (pseudocode):**

```
if (age < 18)
    print("Not eligible to vote")
else
    print("You are eligible to vote")
```

Is that intelligence? It's a **predefined rule**. But now watch how easily rules *mimic* intelligence:

**Example B — a "smart" assistant:**

```
if (device_does_not_turn_on AND battery_percent == 0)
    recommend("Charge your device")
```

To the user, something just **diagnosed their problem**. It looks smart. Behind the scenes: one `if`.

This is why early AI was mocked — *"look behind any AI and you'll find a giant if-else ladder."*

## 4.2 The two ingredients of a rule-based system

**(a) Factual statements (the knowledge base)**
```
order_is_returnable  = true
order_is_damaged     = true
order_was_delivered_3_days_ago = true
```

**(b) Rules (the inference layer)**
```
if (product_delivered_within(7 days)
    AND order_is_damaged
    AND product_is_returnable)
        → user is ELIGIBLE for return / reimbursement
```

Recall how `AND` works: **every** condition must be true to enter the branch. Evaluate our facts — delivered 3 days ago ✓ within 7, damaged ✓, returnable ✓ → eligible.

To the customer, this **looks like a reasoning model** — it appears to be actually thinking through their case. Internally it is a fixed set of rules with zero reasoning.

## 4.3 The fatal limitation: Natural Language Processing

Rule-based AI cannot process **natural language** — and this is *the* central problem of the whole field.

Users do **not** hand you clean structured input. Nobody types:
```json
{ "returnable": true, "damaged": true }
```
They type: *"I want to return my order"*, or politely, or angrily — *"RETURN MY ORDER"* — or in Hindi, or in a mix. Your job is to understand that line. Rules can't, because rules only match what you anticipated, and you can never anticipate a whole language.

## 4.4 Case study: ELIZA (1960s) — the first great illusion

ELIZA was a famous early chatbot that felt eerily human:

```
User : I am feeling unhappy
ELIZA: Why are you feeling unhappy?
User : My friends do not understand me
ELIZA: Why do you think your friends do not understand you?
User : They never listen to me
ELIZA: Can you tell me more about them?
```

**How did a 1960s program do this?** It was **100% rule-based** pattern templates:

| Pattern | Response template |
|---|---|
| `I am feeling <X>` | `Why are you feeling <X>?` |
| `My <X> do not <Y> me` | `Why do you think your <X> do not <Y> you?` |
| `They never <X> me` | `Can you tell me more about them?` |

Reflect the user's own words back as an interrogative. That's it. You could build ELIZA in ten minutes today.

**Why ELIZA matters philosophically:** users *felt* they were talking to something that understood them and had emotions. Your first reaction is "that's stupid, that's not AI at all" — but that reaction assumes a definition of intelligence you haven't actually justified. We judge intelligence by behaviour; ELIZA copied the behaviour.

**ELIZA is the ancestor of today's models in one precise sense:** ChatGPT/Claude are thousands of times more capable, but under our behavioural definition they are still doing the same category of thing — **mimicking**. They are not aware; they don't feel; they mimic.

**ELIZA's practical limits:** narrow, repetitive, no real understanding. Talk to it a few minutes and the illusion collapses: *"this isn't reasoning, it's giving me the same shape of answer every time."*

---

# PART 5 — Era 2: Machine Learning

## 5.1 The motivating failure: teaching a machine what a dog is

Rule-based approach:
```
A dog has 4 legs
A dog has fur
A dog has 2 ears
A dog has a tail
```

**Problem:** every one of those is also true of a **wolf**, a **cat**, and most mammals. So add more rules — *dogs bark*, *dog faces look like this* — and now: a **Chihuahua** and a **Great Dane** look wildly different, and there are hundreds of breeds. The rule set explodes and never closes.

## 5.2 The ML idea: teach it like you teach a child

> Rules can be infinite. Drop the rules. Learn from examples instead.

How does a child learn what a dog is? Not from a definition — from seeing hundreds of dogs in real life, in movies, being told "that's a dog", across many breeds. The brain trains itself, and later classifies a *never-before-seen* dog correctly.

**So:**

```
TRAINING DATA (labelled)          →   MODEL learns/adjusts parameters
  [image] label: dog
  [image] label: dog
  [image] label: cat                  (millions of examples)
  [image] label: cat

TEST DATA (unseen)                →   "I'm 93% sure this is a dog"
  [new image]
```

**Key vocabulary:**
- **Training data** — the examples you learn from.
- **Labelled data** — each example carries its answer ("this is a dog").
- **Test data** — unseen examples used to check performance.
- **Accuracy/confidence** — the output isn't certainty, it's a percentage. 95% ≫ 60%.

## 5.3 Supervised vs Unsupervised (intuition only)

| | Supervised learning | Unsupervised learning |
|---|---|---|
| Training data | **Labelled** ("dog", "cat") | **Unlabelled** (just images) |
| What it learns | Map input → known category | Similarity structure |
| Output | "This is a dog, 93%" | Groups/clusters: Group 1, Group 2, Group 3 |
| Analogy | Teaching with a supervisor | Sorting a pile by resemblance, unaided |

Classification (Part 1's Solution 2, and spam filtering) is the classic **supervised** case.

## 5.4 What is a "model"? (the confusion-killer)

Many people get stuck on this word. Demystify it:

> **A model is just a function: it takes an input and returns an output.** That's it.

- Classical ML model: image in → label out.
- Modern LLM: prompt in → response out.

Internally the model holds **parameters** (numbers). Learning = **adjusting those parameters** against training data. Adjusting/improving them is called **tuning**. A well-tuned model generalises to new inputs.

## 5.5 Limitations of classical ML

**1. Heavy human intervention — "feature engineering."**
A computer doesn't *see* a picture. To a model an image is just a **grid of pixels** and their intensities. It has no eyes. So a human must tell it *what to look at*:
> "Look for regions where pixels are unusually dark; if two such blobs appear near each other, they might be eyes."

That hand-designed guidance is **feature engineering**, and a human effectively becomes the model's full-time guide.

**2. Humans inject their own errors.** Bad features → bad model. Your mistakes get baked in and reflected back.

**3. Poor scaling / no transfer.** A great dog-vs-cat model **cannot** recognise human faces. You must gather a new labelled dataset and retrain from scratch for every new task.

**4. Someone must know the pattern.** Feature engineering assumes a human can *articulate* the pattern. What about problems where **no human can state the pattern at all?** → next era.

---

# PART 6 — Era 3: Neural Networks and Deep Learning

## 6.1 The problem class that needs them

**Task:** decide whether an incoming support ticket is **URGENT** or not.

Candidate signals:

| Variable | Meaning |
|---|---|
| `x1` | Text contains the word "urgent" |
| `x2` | Text mentions a **failed payment** |
| `x3` | Customer has attempted to reach support **multiple times** |
| `x4` | Customer **cannot log in** / account is locked |

Every one of these matters *somewhat*. But **how much** does each matter, and in what combination? No human can honestly state that formula. And in reality there aren't 4 variables — there may be **50**, each carrying *some* non-zero weight. It's never "one variable is responsible and the rest are irrelevant."

## 6.2 A pattern is just a mathematical formula

Write urgency `z` as a weighted sum:

```
z = w1·x1 + w2·x2 + w3·x3 + w4·x4
```

- `x1…x4` = the inputs (we know these)
- `w1…w4` = the **weights** — how much each input matters (**we do NOT know these**)

The whole game: **find the weights.**

## 6.3 What a neural network does

We can't derive the weights, so the network derives them for us. Feed the network huge historical data — millions of past tickets and their real outcomes. It observes: how often did "urgent" appear? when was payment failing? when did the user retry? when was the account locked? Which of those actually turned out to be urgent?

Then it **repeatedly adjusts the weights** until the formula reliably predicts urgency.

**Crucial contrast with classical ML:** we do **not** supervise the pattern here. We *cannot* — we don't know it. **The network discovers the pattern itself.**

## 6.4 Structure and the brain analogy

```
 Input layer      Hidden (intermediate) layers        Output
    ( )  ──────────►  ( )   ──────────►  ( )  ─────────► ( )
    ( )  ──────────►  ( )   ──────────►  ( )
    ( )  ──────────►  ( )   ──────────►  ( )
    ( )  ──────────►  ( )   ──────────►  ( )
        (every node connected onward; many layers possible)
```

Called *neural* because it mimics the brain's **neurons and their connections**. Learning across many layers is called **Deep Learning**.

## 6.5 The hierarchy (clear this up once)

```
┌───────────────────────────────────────────────┐
│  ARTIFICIAL INTELLIGENCE  (broadest term)     │
│  ┌─────────────────────────────────────────┐  │
│  │  MACHINE LEARNING                       │  │
│  │  ┌───────────────────────────────────┐  │  │
│  │  │  NEURAL NETWORKS / DEEP LEARNING  │  │  │
│  │  │   ← all modern LLMs live here     │  │  │
│  │  └───────────────────────────────────┘  │  │
│  └─────────────────────────────────────────┘  │
└───────────────────────────────────────────────┘
```

Neural networks are the *representation*; the learning that happens in them is **deep learning**. **Every modern LLM is built on neural network technology.**

---

# PART 7 — Neural nets conquered images long before text

Neural networks were applied to two big domains:

## 7.1 Computer Vision — solved, and fast

**Model family: CNN (Convolutional Neural Network).**

Applications:
- **Face recognition** — e.g. unlocking a phone with your face
- **Facebook auto-tagging** — upload a photo with a friend, the platform recognises the face and attaches the name
- **Self-driving cars** — continuously interpreting camera frames of the surroundings

Vision became a genuine **superpower** for neural networks.

## 7.2 Text — the humbling surprise

Everyone assumed **text would be the easy part** and images/video the hard part. The opposite happened.

Attempts were made — **RNN**, **LSTM** — and they partially worked: they could generate a few coherent lines. Then they would **lose context**, forget what they were talking about, and sometimes **loop and repeat themselves**.

Text turned out to be genuinely, structurally hard. Which raises the question:

---

# PART 8 — Why is natural language so hard? (deep dive)

Language feels easy to us only because we've absorbed it since childhood. For a model, an entire language must be learned from scratch — and a language carries:

| Dimension | Why it breaks naive models |
|---|---|
| **Context** | Meaning depends on what came before |
| **Grammar** | Structural rules, endless exceptions |
| **Word order** | Changes meaning entirely |
| **Speaker intention** | The same words can mean the opposite |
| **Sarcasm** | Positive words, negative intent |
| **Culture** | Where the speaker is from shapes phrasing |
| **Tone** | Politeness, anger, urgency |
| **Earlier conversation** | Pronouns and references point backwards |

## 8.1 Concrete failure modes

**(a) Sarcasm.** A customer writes: *"बहुत बढ़िया काम कर रहे हो, मेरा ऑर्डर पिछले 15 दिन से नहीं आया।"* ("Doing a great job — my order hasn't arrived in 15 days.") A naive model sees "great job", concludes it's praise, and replies *"Thank you for the kind feedback!"* — catastrophic. The customer was being sarcastic; the intent was a complaint.

**(b) One word, many meanings.** `Same word can have multiple meanings.`

**(c) One meaning, many wordings.** Even staying inside English:
- "Where is my order"
- "Track my package"
- "Why haven't I received my order"

Same intent, no shared keyword.

**(d) Word order matters.**
- *The dog chased the man.*
- *The man chased the dog.*

Same words, opposite events. Subject vs object must be understood.

**(e) Negation flips everything.**
- *I am hungry.*
- *I am **not** hungry.*

**(f) Interrogation changes the act.**
- *Am I hungry?*

**(g) Coreference / references (the hard one).**
> *"Aditya placed the laptop on the table because **it** was heavy."*

As a human you instantly know **"it" = the laptop**. For a model, resolving that pronoun to the right earlier noun is very hard. How do you supply "it"'s context via "laptop"? (Hold this example — Transformers answer it in Part 11.)

**(h) Language is productive.** New words enter dictionaries constantly; new slang and emojis every year; new ways of saying things. The target never stops moving. Hard-coding a language is therefore permanently impossible.

---

# PART 9 — Attempt 1 at language: Statistical / n-gram models

These predate neural networks. The technique: **count words.**

## 9.1 Worked example

Pretend our **entire** training universe is three sentences:

```
1.  I like machine learning
2.  I like java programming
3.  Students like machine learning
```

Now count what follows the word **"like"**:

| Word following "like" | Count |
|---|---|
| machine | 2 |
| java | 1 |

Convert counts to probabilities:

```
P(machine | "like") = 2/3
P(java    | "like") = 1/3
```

**At test time**, input: `Rohan like ___`
The model asks "what most likely follows *like*?" → **machine** (highest probability) → outputs `Rohan like machine`.

That's the whole mechanism: **next-word prediction by counting**.

## 9.2 n-grams: unigram, bigram, trigram

`n` = how many previous words you condition on.

| Name | n | Conditions on | Example |
|---|---|---|---|
| **Unigram** | 1 | 1 previous word | `P(machine \| like)` |
| **Bigram** | 2 | 2 previous words | `P(machine \| I like)` vs `P(machine \| Students like)` |
| **Trigram** | 3 | 3 previous words | … and so on |

**Why bigger n helps:** more context. With unigram, "I like" and "Students like" are indistinguishable — both just "like". With bigram, `I like` vs `Students like` are different contexts, so predictions differentiate.

## 9.3 Why it still fails

Probabilities are computed over **exact words**. Change the word and the meaning may be identical while the probability distribution is completely different:

- Trained: *"I want a refund"* → follow with refund policy. Fine.
- User types: *"I want reimbursement."* Same meaning. Never trained. Fails.
- Train that too, and the user says it a third way — or in Hindi.

**Same infinite-edge-case wall as rule-based AI, now dressed in statistics.** And large `n` doesn't save you: data sparsity grows and you still don't have *meaning*, only surface co-occurrence.

---

# PART 10 — Attempt 2: RNNs (sequence processing)

**RNN** — presented in the lecture as "Recursive Neural Network"; the standard name is **Recurrent Neural Network**. Related: **LSTM**.

These genuinely advanced NLP — but never became LLMs.

## 10.1 How sequence processing works

Input: `The payment has not been refunded`

```
read "The"       → update internal state
read "payment"   → update internal state
read "has"       → update internal state
read "not"       → update internal state
read "been"      → update internal state
read "refunded"  → update internal state → predict / output
```

The model holds an **internal state**: a *summarised form of everything read so far*.

## 10.2 The sticky-note analogy (memorise this)

You're reading a novel. After pages 1–3, you write everything you understood onto a **small sticky note**. Then you read pages 4, 5, 6 — updating that one sticky note each time.

By page 6, how much of page 1 survives on the note? **Almost nothing.** A sticky note can only hold a summary; enormous information was lost.

**That is exactly RNN context loss.** After word 1 it holds word 1. After word 2 it holds a summary of words 1–2. It never holds the *full* sentence — only a super-summarised version. As sentences grow longer, **early context decays**.

## 10.3 Consequences

- Fine for a few lines; **fails on long generation**
- **Loses context** of earlier words
- Can fall into **loops**, repeating itself
- **Slow to train**: strictly word-by-word, sequential. To train on the entire internet (which is what today's models do), a word-by-word reader would take, effectively, forever. **No parallelism.**

## 10.4 Example

Input: `The dog barked at the ___`

The RNN reads it word by word, holds context in its internal state, then picks the highest-probability next word from training — say `delivery boy` → *"The dog barked at the delivery boy."* Reasonable, but it is fundamentally still probability-over-a-decaying-summary.

**Verdict:** best pre-2017 attempt at NLP. Never became an LLM.

---

# PART 11 — 2017: "Attention Is All You Need" → Transformers

The research paper that **changed the world**. It solved what nothing before it could: processing natural language properly.

## 11.1 The two breakthroughs

**(1) No more word-by-word sequential state.**
An RNN ingests one word at a time and updates a summary. A Transformer takes the **entire context — sentence, paragraph, page — into the neural layers at once**, and understands it *as it is*. This makes it:
- **Far better at context** (nothing decays into a sticky note)
- **Far faster / parallelisable**, which is exactly what makes training on internet-scale data feasible

**(2) Attention: relevance between words.**

To process a word, the Transformer asks: *which other words help me understand **this** word?*

**Worked example:**
> *"Aditya gave Rohit the laptop because **he** needed **it** for work."*

Humans resolve this instantly. A naive model cannot: who is "he" — Aditya or Rohit? What is "it"? For the model to understand "he", it must somehow *match* "he" against Aditya/Rohit. To understand "it", it must match against "laptop".

The Transformer computes a **relevance score between "he" and every other word**:

| Word pair | Relevance |
|---|---|
| he ↔ Aditya | **High** (it's a name) |
| he ↔ Rohit | **High** (also a name) |
| he ↔ gave | Low (a verb) |
| he ↔ the | Low |
| he ↔ laptop | Medium |

It then knows which words are *necessary* to interpret this particular word in this particular context.

## 11.2 The most important part: nobody wrote those rules

**These are not human-written rules.** Transformers are built on **neural networks** — and we established that in a neural network we *don't* hand over the pattern; the network finds it.

Nobody told the model "'he' usually refers to a name/noun." It **observed that pattern itself** across billions and trillions of sentences during training, and therefore assigns "he ↔ Aditya" high relevance and "he ↔ gave" low relevance.

## 11.3 How deep should an FDE go here?

Enough for **intuition** — an overview of how it works at a higher level, so you can exploit these systems in your own work. Full Transformer internals are a **deep learning** specialisation, an entire field of its own.

**Recommended:** read the paper **"Attention Is All You Need"** at least once. It genuinely defines an era — this one paper is the reason people say **"pre-ChatGPT era" / "post-ChatGPT era."**

---

# PART 12 — The post-Transformer era: LLMs

Once Transformers existed (2017), we could process natural language well *and* fast. The race to build **LLMs** began.

**LLM = Large Language Model.** Called *large* because they're trained on enormous data. ChatGPT, Claude, DeepSeek, Gemini are LLM-based products.

**ChatGPT's launch** (Sam Altman's announcement, **around the end of 2022**) is the moment this reached the public. Suddenly a model could handle Hindi, English, Hinglish, regional languages; understand what "it", "they", "them" refer to; hold context; and **generate** a good answer.

## 12.1 Decoding "GPT" — Generative Pre-trained Transformer

| Letter | Meaning | Why |
|---|---|---|
| **G — Generative** | It **generates** new content | Unlike older ML classifiers, which could only *classify* (spam / not-spam). A GPT actually produces natural language for you. |
| **P — Pre-trained** | Already trained on a **very large** dataset before you ever use it | You get a general-purpose language ability out of the box |
| **T — Transformer** | Built on Transformer architecture | The 2017 breakthrough underneath everything |

## 12.2 So why did text take so long? (Two reasons, not one)

Many people assume all AI progress happened in the last decade. **False** — research, papers and tools were flowing steadily in ML, neural networks and NLP the whole time. Images were cracked early via computer vision. Text lagged for **two** reasons:

1. **Algorithmic** — we didn't have the Transformer/attention idea yet.
2. **Hardware** — GPUs and compute of the necessary scale simply **did not exist**. Even the RNN and statistical models we *did* have could not be trained on giant datasets, because the hardware resources weren't there.

Don't remember only reason 1. Both mattered.

---

# PART 13 — Consolidated timeline

| Year | Event | Significance |
|---|---|---|
| Pre-1950 | Computers as calculators | Clear input, precise instructions, deterministic output |
| **1950** | Turing asks: can a machine do tasks needing human intelligence? / *Can machines think?* | Founding question of AI |
| 1950 | **Turing Test** (Imitation Game) proposed | Replaces a philosophical question with an operational one |
| **1956** | McCarthy & team coin **"Artificial Intelligence"** | AI becomes a recognised field; research boom |
| 1950s–60s | **Rule-based AI** | Facts + rules; giant if-else ladders |
| **1960s** | **ELIZA** chatbot | Convincing human-like dialogue from pure pattern templates |
| 1960s onward | **Statistical / n-gram models** | Next-word prediction by counting; unigram/bigram/trigram |
| 1980s–2010s | **Machine Learning** (supervised/unsupervised, classification) | Learn from labelled data instead of rules; spam filters |
| 2010s | **Neural networks / Deep learning** | Model discovers patterns humans can't articulate |
| 2010s | **CNNs** dominate vision | Face recognition, auto-tagging, self-driving |
| 2010s | **RNN / LSTM** for text | Partial NLP success; context loss, loops, slow |
| **2017** | **"Attention Is All You Need"** → **Transformers** | Full-context, parallel, attention-based. Era-defining |
| **~end 2022** | **ChatGPT** goes public | LLMs reach mass adoption; "post-ChatGPT era" |
| Now | FDE work | Wiring grounded LLM systems into real business workflows |

---

# PART 14 — Glossary (quick revision)

| Term | One-line meaning |
|---|---|
| **FDE** | Engineer placed with the customer/ops who converts raw business problems into deployed solutions |
| **Raw requirement** | What the business says ("build an AI chatbot") |
| **Ordered requirement** | What they actually need (objective + constraints, technology-neutral) |
| **Rule-based AI** | Facts + hand-written rules; if-else ladder |
| **NLP** | Natural Language Processing — making machines handle human language |
| **Classification** | Supervised ML that assigns an input to one of N categories |
| **Labelled data** | Training examples that carry their correct answer |
| **Training / test data** | Data used to learn / to evaluate on unseen inputs |
| **Model** | A function: input → output, governed by parameters |
| **Parameters / tuning** | Internal numbers, and the process of adjusting them |
| **Feature engineering** | Humans telling the model what to look at |
| **Supervised / unsupervised** | Learning from labelled / unlabelled data |
| **Neural network** | Layered, brain-inspired architecture that discovers patterns itself |
| **Weights** | How much each input contributes (`w1…wn`) |
| **Deep learning** | Learning in multi-layer neural networks |
| **CNN** | Convolutional NN — the vision workhorse |
| **RNN / LSTM** | Sequence models with an internal state; suffer context loss |
| **n-gram** | Statistical next-word prediction on the previous n words |
| **Transformer** | 2017 architecture: full context at once + attention |
| **Attention** | Scoring how relevant every other word is to the word being processed |
| **LLM** | Large Language Model |
| **GPT** | Generative Pre-trained Transformer |
| **Hallucination** | A confident, fluent, wrong answer |
| **Context** | Extra grounding data (policies, order status, history) sent with the query |
| **Turing Test** | If an evaluator can't distinguish machine from human via text, the machine passes |
| **Mimicry** | Reproducing intelligent behaviour without awareness or feeling |

---

# PART 15 — Fact-checks and small corrections

Worth knowing so you don't repeat transcript slips in an interview:

1. **Query volume:** the lecture says "500 users/day" once and "50,000" thereafter. Use 50,000 as the intended figure.
2. **"AI" coined:** the lecture says 1955 with McCarthy & team. The **proposal** for the Dartmouth workshop was written in **1955**; the **workshop where the term took hold was 1956**. Both dates appear in sources for that reason.
3. **RNN:** stated as "Recursive Neural Network." The standard name is **Recurrent Neural Network**. (Recursive Neural Networks are a different, tree-structured family.)
4. **ChatGPT launch date:** the lecture guesses "December 1, 2022." The public launch was **November 30, 2022**.
5. **"ChatGPT was the first LLM":** not quite. GPT-1 (2018), GPT-2 (2019), GPT-3 (2020), BERT (2018) and others preceded it. ChatGPT was the first LLM to reach **mass consumer adoption** — that's the accurate claim.
6. **ChatGPT/Claude ≠ a bare LLM:** the lecture flags this itself. They are products: model + system prompt + tools + retrieval + safety layers + UI.
7. **Transformers vs "no context limit":** Transformers process the whole given context at once, but that context is still **bounded** (the context window). Later lectures will cover this.

---

# PART 16 — Self-test (answer these from memory before Lecture 2)

**On the FDE role**
1. Why is "build us an AI chatbot" not a requirement?
2. State the reframed requirement for the e-commerce case, verbatim in spirit.
3. List the five real pain points behind the ask.
4. What do "Forward" and "Deployed" each mean in the title?
5. Give three differences between an FDE and a normal SDE.
6. Order the four candidate solutions and name the exact failure mode that killed each of the first three.
7. What four pieces of context did we feed the LLM, and which failure does each prevent?

**On AI history**
8. What three properties did all pre-AI computing share?
9. Why did Turing replace "Can machines think?" with the Turing Test?
10. What does the Turing Test imply about mimicry? Why is "convincing ≠ correct" relevant to today's LLMs?
11. What are the two ingredients of a rule-based system? Write the return-eligibility rule.
12. How did ELIZA work, and why is it still philosophically interesting?
13. Why does the rule-based approach fail at "what is a dog"?
14. Define model, parameters, labelled data, training vs test data.
15. Supervised vs unsupervised — one crisp difference.
16. Name three limitations of classical ML. What is feature engineering?
17. Why can't a human write the ticket-urgency formula, and what does a neural network do instead?
18. Draw the AI ⊃ ML ⊃ NN hierarchy.
19. Why were images cracked before text? List six reasons language is hard.
20. Compute `P(machine | "like")` from the three-sentence corpus. Why do n-grams still fail?
21. Explain RNN context loss using the sticky-note analogy. Name two other RNN weaknesses.
22. What two things did Transformers change? Explain attention on the "Aditya gave Rohit the laptop because he needed it" example.
23. Expand GPT and justify each letter.
24. Give both reasons text lagged behind images.

---

# PART 17 — What's coming in Lecture 2

- Inside **LLMs**: what "Large Language Model" actually means
- How they **predict**
- **Tokens** and the **tokenization** process
- Then onward, in depth, through the rest of the GenAI stack

**FDE mindset to carry forward:** you don't need to be able to *build* a Transformer. You need enough intuition about how each layer works to **pick the right tool, ground it in real company context, and get it into production use.**
