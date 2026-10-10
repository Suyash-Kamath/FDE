# Multimodal AI — Complete Deep-Dive Explanation

Based on Aditya Tandon's Forward Deployed Engineering lecture, your 28-page PDF notes, and your Excalidraw SVG diagram.

The central question of this entire lecture is:

How can a Large Language Model, which fundamentally performs numerical computations, look at an image, understand what is happening inside it, relate that information to a user's question, and generate a meaningful answer?

We are going to study this at three levels:

1. Conceptual: What multimodal AI is, how it differs from OCR, and why it is useful.
2. Internal architecture: How pixels become patches, how vision encoders generate visual representations, and how Transformers combine image and text information.
3. Application engineering: How the lecturer builds Vision Chat using Spring Boot, Spring AI, image uploads, and manually maintained conversation history.

I'll also distinguish the lecture's simplified explanations from the more precise technical mechanisms used in modern vision-language models.

## Part 1 — Understanding Multimodal AI from first principles

### 1. Why do we need multimodal AI?

Imagine you are building a customer-support chatbot.

A user reports:

> My payment is failing. Can you help me?

Your conventional text-based chatbot receives the user's message and generates a response.

The architecture is straightforward:

Client (Browser / Frontend)

"Why is my payment failing?"

POST /chat

Backend (Spring Boot / FastAPI)

Build request and call model API

LLM

Processes text and generates an answer

Text response to the user

This is the exact client → server → LLM architecture drawn at the beginning of your Excalidraw diagram.

But suppose the customer uploads this screenshot:

payment-error.png

## Payment Failed

# ₹7,999

Reason: Insufficient Balance

And asks:

> Look at this screenshot and explain why my payment failed.

The useful information is no longer entirely inside the text prompt.

Part of the information lives inside the image.

A text-only model cannot interpret that image merely because you attach a filename. The application needs a system capable of processing visual information.

With a multimodal model, the request becomes:

```
User input:
    Text  = "Why is my payment failing?"
    Image = payment-error.png

Model output:
    "The payment failed because the
     available balance was insufficient."
```

The model has combined two sources of information:

- The question, which tells it what the user wants.
- The image, which contains the evidence needed to answer.

This is multimodal AI.

### 2. What exactly is a modality?

A modality is a form of information.

Different forms of information have different physical and computational representations.

| Modality    | Example              | Underlying representation          |
| ----------- | -------------------- | ---------------------------------- |
| Text        | "Hello, Aditya"      | Character encoding, tokens         |
| Image       | `cat.jpg`            | Encoded image data, decoded pixels |
| Audio       | A voice recording    | Audio samples or encoded audio     |
| Video       | A meeting recording  | Frames, timestamps, often audio    |
| Sensor data | Temperature readings | Numerical time series              |

A multimodal AI model can process or generate more than one modality.

For example:

\\[ f(\text{Text},\text{Image})\rightarrow\text{Text} \\]

Here, \\(f\\) represents the model.

The inputs are text and an image, while the output is text.

The lecture focuses specifically on this configuration. It does not teach image generation or audio generation.

A more general multimodal system might support:

\\[ f(\text{Text},\text{Image},\text{Audio},\text{Video}) \rightarrow \text{Text or other modalities} \\]

The exact supported inputs and outputs depend on the model.

### 3. Multimodal vs Multi-Model

Your transcript explicitly distinguishes these two terms at approximately 05:00.

They sound similar, but describe completely different ideas.

Multimodal

Different information types

Text

Image

One multimodal model

Multi-Model

Different AI models

Model A

Model B

One application

Multimodal: Your application uses a vision-capable model to understand a screenshot together with the user's question.

Multi-model: Your application integrates different models, perhaps choosing one for inexpensive requests and another for more complex tasks.

An application can be both multimodal and multi-model. These properties are independent.

For example, your backend could route visual support tickets to a vision-language model and ordinary textual questions to a text-only model.

### 4. OCR vs Multimodal LLM

This is one of the most important concepts in the lecture.

OCR stands for Optical Character Recognition.

Its primary purpose is to recognize written characters in images or scanned documents.

Imagine this screenshot:

```
----------------------------
     PAYMENT FAILED

     Amount: ₹7,999

     Insufficient Balance
----------------------------
```

A traditional OCR pipeline processes the screenshot and extracts the written content:

```
PAYMENT FAILED
Amount: ₹7,999
Insufficient Balance
```

It has transformed image information into text.

A multimodal LLM, however, can answer a question about the screenshot.

For example:

Question: Why did the transaction fail?

Answer: The transaction failed because there was insufficient balance to complete the ₹7,999 payment.

It can use the recognized text, the visual layout, and the question together.

| Capability                 | Traditional OCR              | Multimodal LLM   |
| -------------------------- | ---------------------------- | ---------------- |
| Read printed text          | Yes                          | Yes              |
| Extract numbers            | Yes                          | Yes              |
| Recognize visual objects   | Not its core purpose         | Yes              |
| Interpret screen layout    | Limited, depending on system | Often            |
| Explain an error           | Requires additional logic    | Yes, potentially |
| Answer follow-up questions | Not by itself                | Yes              |
| Guarantee exact extraction | No                           | No               |

One subtle point: OCR and multimodal LLMs are not competing technologies in every situation.

A production system can use both:

```
Invoice image
      |
      v
OCR engine
      |
      v
Extracted text
      |
      v
LLM
      |
      v
Validated invoice fields
```

Or it can send the image directly to a multimodal model:

```
Invoice image + Question
           |
           v
    Multimodal LLM
           |
           v
    Structured output
```

Neither approach is automatically superior for every document.

OCR can be preferable when you need predictable document extraction, specialized layouts, or independent text verification. Multimodal LLMs are useful when interpretation of layout and meaning matters.

Key distinction: OCR principally extracts what is written. A multimodal LLM can attempt to explain what the written and visual information means.

## Part 2 — How computers actually represent images

Now we are entering the most important technical section of your PDF, especially pages 8–14, and the middle of your Excalidraw diagram.

The lecturer makes an important observation:

> To understand how an LLM can process an image, first understand how computers represent information numerically.

### 5. How does a computer understand text?

Imagine sending this message:

```
Hi, my name is Aditya.
```

A language model does not directly operate on the English characters the way a human reader does.

The simplified pipeline is:

```
"Hi, my name is Aditya."
             |
             v
        Tokenizer
             |
             v
     Sequence of tokens
             |
             v
         Token IDs
             |
             v
     Embedding lookup
             |
             v
   Numerical token vectors
             |
             v
      Transformer layers
             |
             v
      Output token logits
             |
             v
     Next-token selection
```

Let's understand the stages.

Step 1: Tokenization

The tokenizer divides text into units called tokens.

For illustration:

```
"Hi, my name is Aditya."

["Hi", ",", " my", " name", " is", " Aditya", "."]
```

This is illustrative. A real tokenizer could produce a different sequence.

Step 2: Assign token IDs

Each token is mapped to an integer based on a fixed vocabulary.

For example:

```
"Hi"       -> 23
","        -> 41
" my"      -> 56
" name"    -> 82
...
```

These IDs are fictional, just like those used by the lecturer.

An important detail: token IDs are identifiers, not meaningful numerical measurements.

Token ID 100 is not necessarily semantically closer to token ID 101 than to token ID 8000.

Step 3: Look up learned token embeddings

The model contains an embedding matrix.

Suppose:

- Vocabulary size = 50,000 tokens.
- Hidden dimension = 768.

The embedding matrix would have the shape:

\\[ E\in\mathbb{R}^{50000\times768} \\]

For any token ID \\(i\\), we can retrieve its embedding:

\\[ e_i=E[i] \\]

An illustrative result:

```
Token ID: 23

Embedding:
[0.12, -0.45, 0.87, 0.21, ...]
```

This vector becomes part of the input to the Transformer.

Step 4: Contextual processing

This is where attention becomes important.

Consider:

- "I deposited money in the bank."
- "We sat on the bank of the river."

The word bank appears in both sentences, but its meaning depends on surrounding words.

The model uses attention and other learned transformations to produce context-dependent internal representations.

A subtle distinction: the initial token embedding is not already fully aware of sentence context. The contextual representation develops through the Transformer layers.

Step 5: Predict the next token

The model computes scores for vocabulary tokens, turns them into a probability distribution, and selects or samples the next token according to the decoding strategy.

So the essential observation is:

Text → Tokens → Vectors → Neural-network computations → Text output

Now comes the interesting part.

Could we represent an image numerically too?

Absolutely.

### 6. What is a pixel?

A pixel is a sample of image information at a particular position.

A normal color image can be thought of as a rectangular grid of pixels.

Each pixel in a conventional RGB image has three color components:

- R: Red
- G: Green
- B: Blue

For a standard 8-bit-per-channel RGB image, each component ranges from 0 to 255.

Interactive RGB pixel explorer

Pixel value

RGB(255, 120, 30)

Change any channel to see the resulting color.

Red

255

Green

120

Blue

30

Reset to lecture example

Examples:

| RGB             | Color |
| --------------- | ----- |
| (255, 0, 0)     | Red   |
| (0, 255, 0)     | Green |
| (0, 0, 255)     | Blue  |
| (255, 255, 255) | White |
| (0, 0, 0)       | Black |

When you see a photograph of a cat on your screen, you perceive an animal. But the image-processing system starts with numerical information.

### 7. How is an entire image represented?

Imagine an image measuring 1920 × 1080 pixels, exactly the example in the transcript.

That means:

\\[ 1920\times1080=2,073,600\text{ pixels} \\]

With three RGB channels:

\\[ 2,073,600\times3=6,220,800 \\]

So there are over six million channel values in the decoded RGB image.

Mathematically, a conventional RGB image is represented as a tensor:

\\[ I\in\mathbb{R}^{H\times W\times C} \\]

Where:

- \\(H\\): image height
- \\(W\\): image width
- \\(C\\): number of channels, normally 3 for RGB

For our example:

\\[ I\in\mathbb{R}^{1080\times1920\times3} \\]

Here's an extremely small example with just four pixels:

```
image = [    [[255, 0, 0],   [0, 255, 0]],    [[0, 0, 255],   [255, 255, 255]]]
```

255,0,0

0,255,0

0,0,255

255,255,255

This example is a 2 × 2 RGB image.

An image-processing neural network receives similarly structured numerical data, just on a much larger scale.

One clarification to the lecture: JPEG and PNG files do not necessarily store their contents as a straightforward uncompressed RGB matrix. JPEG uses a compressed representation, for example. Image-decoding software produces a pixel representation that neural-network preprocessing can use.

### 8. Why not send all raw pixels directly to a language model?

Suppose you convert every RGB channel value into textual numbers and send them to an LLM.

Your message might begin:

```
255,120,30,255,119,32,252,117,29,...
```

There are multiple problems.

First, the sequence is huge.

Second, individual color values do not explain object structure. A model needs to understand spatial relationships, edges, shapes, and patterns.

Third, language-model tokenization is not an efficient way to represent millions of raw pixel values.

A specialized vision-processing pipeline is much more suitable.

That brings us to the next concept.

## Part 3 — Image patches: the foundation of vision Transformers

### 9. What exactly is an image patch?

In the transcript, at approximately 16:00, the lecturer introduces patches.

An image patch is a small rectangular region of an image.

For example, take this cat picture.

[The true drama of animal rescue – The Uniter](https://images.openai.com/static-rsc-4/5Tn46gOLix11pU5LKJvjtBI7bkLaw9RkFz7FOYTrtfkoM0wZNDxFr12qrW16_x0FCZMYlfNfR4CTokP6_LSCxr6kZDEi6lqN9v1T7z9TFmUg9CbM-2U5dweAMqjjPatXDfj05Zr9nQbAenDXfgT_xphcrFLs6SWkjBvhk4dJtd0?purpose=inline)

[uniter.ca](https://uniter.ca/view/the-true-drama-of-animal-rescue)

A vision system can divide the picture into a grid of smaller sections.

Conceptual 4 × 4 patch grid

P1

P2

P3

P4

P5

P6

P7

P8

P9

P10

P11

P12

P13

P14

P15

P16

Each cell represents a region of the original image, not a single pixel. A real vision encoder would process the actual pixels in those regions.

Imagine that different patches capture parts of the cat:

```
Patch 1  -> Background
Patch 2  -> Ear region
Patch 3  -> Eye region
Patch 4  -> Fur region
Patch 5  -> Whiskers
...
```

These labels are descriptions for us. The model is not handed the labels "ear" or "fur" when patches are created.

It receives the pixel values.

It must learn useful visual features from them.

### 10. What is the difference between a pixel and a patch?

A pixel is one image sample.

A patch is a collection of pixels from a small region.

The lecturer uses an example patch size of:

\\[ 16\times16 \\]

That means one patch contains:

\\[ 256\text{ pixels} \\]

For an RGB image:

\\[ 16\times16\times3=768 \\]

So each such patch contains 768 raw channel values.

The patch is still not a semantic embedding. It's a little block of image data.

The neural network must transform those 768 values into a learned representation.

### 11. How many patches will an image have?

This depends on image dimensions, patch size, padding, resizing, and the architecture.

For a simple, non-overlapping grid where the dimensions are exactly divisible by the patch size:

\\[ N=\frac{H}{P}\times\frac{W}{P} \\]

Where \\(P\\) is the patch width and height.

Take a 224 × 224 image with 16 × 16 patches:

\\[ N=\frac{224}{16}\times\frac{224}{16} \\]

\\[ N=14\times14=\boxed{196\text{ patches}} \\]

Try different values:

Patch count calculator

Square image size224 × 224

Patch size16 × 16

Patches per side

# 14

Total patches

# 196

For a straightforward non-overlapping patch grid. Actual multimodal models can resize, crop, tile, or use more complex tokenization.

Why does patch size matter?

Smaller patches preserve finer spatial detail but increase the number of visual units.

Larger patches reduce the number of units but can lose fine detail.

That tradeoff is especially important when reading small text in screenshots.

### 12. Are image patches the same as text tokens?

The lecturer uses this analogy:

```
Text:
Sentence -> Tokens -> Embeddings

Image:
Image -> Patches -> Visual Embeddings
```

This is a useful conceptual comparison.

However, the actual mechanisms differ.

Text tokenization is usually based on a learned or fixed vocabulary of textual pieces.

Image patching is often a spatial division of pixel data. The patches are then transformed into numerical features by learned neural-network operations.

In many vision Transformer architectures, the resulting patch representations are treated as visual tokens.

So we can say:

Text tokens and image tokens play analogous roles, but they are not created the same way.

## Part 4 — Vision encoders and visual embeddings

### 13. What is a vision encoder?

We now arrive at the central component of the lecture.

A vision encoder is a neural network that converts image information into learned numerical representations.

In your Excalidraw, the lecturer draws:

```
Image
  |
  v
Patches
  |
  v
Vision Encoder
  |
  v
Visual Embeddings
```

The purpose of the vision encoder is to extract features from visual data.

Instead of exposing millions of raw color values to the language model, it produces a more useful set of representations.

These representations can capture information about edges, textures, shapes, object parts, and relationships between image regions.

### 14. How does a vision encoder convert pixels into vectors?

Let's go deeper than the lecture.

Consider the earlier 224 × 224 image.

After dividing it into 16 × 16 patches, we have 196 patches.

Each patch contains 768 raw RGB values.

For a simple Vision Transformer, we can represent patch \\(i\\) as:

\\[ x_i\in\mathbb{R}^{768} \\]

This process of converting a 16 × 16 × 3 patch into a one-dimensional array is called flattening.

For example:

```
16 × 16 × 3 pixels
        |
        v
Flatten
        |
        v
[255, 120, 30, 253, 117, 29, ...]
        |
        v
768 values
```

But these are only pixel values.

We still need a learned transformation.

#### Linear patch projection

A basic Vision Transformer applies a learned linear projection:

\\[ z_i=x_iW+b \\]

Where:

- \\(x_i\\) is the flattened patch.
- \\(W\\) is a learned weight matrix.
- \\(b\\) is a learned bias.
- \\(z_i\\) is the projected patch representation.

Suppose the input patch has 768 values and the chosen hidden dimension is 768.

Then:

\\[ W\in\mathbb{R}^{768\times768} \\]

The output is a 768-dimensional vector.

Notice what is happening.

768 raw pixel values are being transformed into 768 learned feature values.

Even though the number of dimensions is the same in this example, the meaning of those dimensions has changed.

The input values represent color channels.

The output values are learned features.

This projection is part of patch embedding. The visual representation becomes increasingly contextual as it passes through subsequent neural-network layers.

#### Positional embeddings

But there is another problem.

Suppose the model receives two patches:

- One contains a portion of a cat's ear.
- Another contains a portion of a cat's eye.

It also needs information about where those patches came from.

In a standard ViT design, we add a positional embedding:

\\[ z_i^{(0)}=x_iW+b+p_i \\]

Where \\(p_i\\) is the positional representation for patch \\(i\\).

This gives the model both visual features and positional information.

The resulting sequence passes through Transformer encoder layers.

### 15. Why does spatial position matter so much?

This is the concept drawn in your SVG as:

```
Patch content
      +
Patch position
```

Imagine a payment screenshot with these elements:

```
Amount: ₹7,999

Status: Failed

Reason: Insufficient Balance
```

The relative positions of the text matter.

If a model recognized these words without maintaining enough layout information, it might struggle to determine which status belongs to which transaction.

Similarly, consider a UI:

- A red error message below a password field.
- A red error message below a payment field.

Both might look similar, but their position changes their meaning.

In the cat example, patch locations help the model learn that particular regions form ears, eyes, a face, and surrounding features.

Spatial information can be introduced through absolute positional embeddings, relative positional representations, attention biases, or other architecture-specific mechanisms.

The principle stays the same: visual meaning depends on both content and spatial relationships.

### 16. What exactly is a visual embedding?

A visual embedding is a learned numerical representation of visual information.

For example, the lecturer illustrates one as:

```
[0.12, -0.55, 1.04, 0.31, ...]
```

Do not interpret these values individually as predefined facts.

For example:

```
0.12  != "cat ear"
-0.55 != "gray fur"
1.04  != "whisker"
```

The representation is distributed across multiple dimensions.

Neural networks learn how combinations of dimensions are useful for recognizing and relating visual patterns.

Another crucial distinction:

A vision encoder can produce a sequence of representations, not necessarily just one vector for the entire picture.

For example:

```
Image
  |
  v
196 patches
  |
  v
Vision Transformer
  |
  v
196 contextual visual representations
```

Some architectures additionally produce pooled, global, or compressed representations.

This is useful because a whole-image vector can describe overall image content, while multiple image-region representations can preserve more detailed spatial information.

### 17. Vision encoder vs CNN vs Vision Transformer

The lecturer mentions CNNs around 21:30 but does not explore their mathematical details.

Here's the deeper comparison.

| Architecture                       | Main idea                                                         | Strength                                 |
| ---------------------------------- | ----------------------------------------------------------------- | ---------------------------------------- |
| CNN (Convolutional Neural Network) | Apply learned filters to local image regions                      | Strong local feature extraction          |
| ViT (Vision Transformer)           | Represent image patches as tokens and process them with attention | Captures relationships across patches    |
| Hybrid vision encoder              | Combine convolutional and Transformer techniques                  | Integrates local and contextual features |

A traditional CNN might first learn elementary features such as lines and edges, then combine them into textures, shapes, and higher-level object representations.

A Vision Transformer often begins with patch embeddings, adds positional information, and applies Transformer layers.

They are different ways of learning useful representations from visual information.

## Part 5 — How attention works between image patches

This is the heart of the technical explanation.

### 18. Why is understanding individual patches insufficient?

Suppose a particular patch includes a dark triangular shape.

On its own, that shape might be:

- Part of a cat's ear.
- Part of a building roof.
- A shadow.
- A triangular object.

The patch alone may not contain enough evidence.

But suppose nearby or related patches contain:

```
Patch A: Triangle-like shape
Patch B: Eye region
Patch C: Fur texture
Patch D: Whisker-like lines
```

Together, these provide stronger evidence of a cat.

Your Excalidraw captures this using the words:

```
Fur + Ear + Eyes + Background
                  |
                  v
                 Cat
```

The lecturer compares this to the ambiguity of the word bank.

The surrounding information changes how a piece of information should be interpreted.

This is where attention helps.

### 19. Self-attention across image patches

In a Vision Transformer, patch representations can interact through self-attention.

For a single attention head, the three important quantities are:

- Query (Q)
- Key (K)
- Value (V)

Given an input representation matrix \\(X\\), the model learns transformations:

\\[ Q=XW_Q \\]

\\[ K=XW_K \\]

\\[ V=XW_V \\]

The attention calculation is:

\\[ \boxed{ \operatorname{Attention}(Q,K,V) = \operatorname{softmax} \left(\frac{QK^T}{\sqrt{d_k}}\right)V } \\]

Let's interpret this visually.

Imagine patch 12 corresponds to an area around the cat's eye.

Its query vector can be compared with keys from other patches.

The computed attention weights determine how much information from the corresponding value vectors contributes to the updated representation.

Illustrative patch-attention example

Imagine the model updating an eye-region patch using information from other image regions.

Query: Eye-region patch

Hypothetical attention weights

Nearby fur

40%

Ear region

30%

Other eye

20%

Background

10%

Updated contextual patch representation

These are invented illustrative weights, not measured values from a particular model. Actual attention has multiple heads and layers.

Notice the conceptual change:

Before attention:

```
Patch representation = features from its image region
```

After attention:

```
Updated representation =
    information from this patch
    +
    weighted information from other patches
```

The model repeats attention and other learned computations across multiple layers.

Over those layers, representations can become increasingly useful for image understanding.

#### Why multiple attention heads?

Different heads can learn different interaction patterns.

For example, one head might become useful for local relationships while another attends to more distant visual regions.

These are learned behaviors, not fixed instructions assigning an object-detection task to each head.

#### Computational cost

For ordinary full self-attention over \\(N\\) patches, the attention matrix is approximately \\(N\times N\\).

So the attention computation scales quadratically with the sequence length:

\\[ O(N^2) \\]

For 196 patches, one head has:

\\[ 196^2=38,416 \\]

query-key pairwise scores.

This is one reason image resolution and visual-token count influence processing cost.

## Part 6 — Text embeddings vs visual embeddings and the projection layer

### 20. Why can't we simply feed visual embeddings into any text-only LLM?

Your transcript identifies a major problem:

The visual encoder produces vectors, and the language model also uses vectors, but they are not automatically compatible.

Let's examine why.

Imagine:

```
Text embedding:
[0.31, -0.18, 0.77, ...]

Visual embedding:
[0.28, -0.11, 0.81, ...]
```

They are both numerical vectors.

But that does not mean their individual dimensions have matching meanings.

They might also have different dimensionalities.

For example:

```
Vision encoder:
768-dimensional vectors

Language model:
4096-dimensional hidden states
```

You cannot simply concatenate mismatched representations into a sequence the language model expects.

You need an alignment or integration mechanism.

### 21. What is a projection or adapter layer?

A projection layer transforms one representation into another compatible representation.

For example:

\\[ v\in\mathbb{R}^{768} \\]

The target language-model hidden dimension is 4096.

A learned linear projection could perform:

\\[ h=vW+b \\]

With:

\\[ W\in\mathbb{R}^{768\times4096} \\]

Giving:

\\[ h\in\mathbb{R}^{4096} \\]

The visual representation now has the required dimensionality.

But dimensionality alone is not enough.

The projection must be trained to produce representations that the language model can use meaningfully.

This is why simply padding an image vector with zeros would not solve the problem.

Visual projection pipeline

Image / Patches

Vision Encoder

Learn visual features

Visual feature — 768 dimensions

Projection / Adapter

Learn a compatible representation

LLM-compatible feature — 4096 dimensions

The dimensions are illustrative.

### 22. Does the adapter literally convert an image into text embeddings?

This is where I want to make an important distinction from the lecture.

The lecturer repeatedly describes the projection as converting visual embeddings into text embeddings.

That is useful as an introductory explanation, but technically it is not always precise.

In many architectures, the projection converts visual features into visual tokens that are compatible with the language model's hidden representation space.

The projected feature does not have to correspond to an ordinary word or a text-token ID.

For example:

```
Visual representation of cat's ear
                |
                v
             Adapter
                |
                v
Compatible continuous vector
```

The adapter is not necessarily turning that vector into the English token `"ear"`.

Rather, it is producing a representation that the language model can attend to and use when generating an answer.

This difference matters when you start studying multimodal architecture papers.

### 23. Are all multimodal models built with the same adapter architecture?

No.

The lecture describes a very common conceptual design, but modern vision-language systems can vary considerably.

Some common architectural approaches include:

| Approach                        | How visual information is integrated                                                  |
| ------------------------------- | ------------------------------------------------------------------------------------- |
| Projection-based                | Visual representations are mapped into the LLM's hidden space                         |
| Cross-attention-based           | Language representations attend to visual features through dedicated attention layers |
| Resampler / query-based         | A module condenses visual features into a smaller set of representations              |
| More unified multimodal designs | Different modalities are processed through a more integrated architecture             |

The important concept from your notes remains valid:

An appropriate mechanism is required to connect visual information to language generation.

## Part 7 — How the model combines text and image information

We've now understood both input pipelines.

The next question is:

How does a model connect the question "What animal is this?" with the visual information showing a cat?

### 24. The two processing pipelines

Suppose the request contains:

```
Text: "What animal is shown here?"
Image: cat.jpg
```

The text and image can travel through different processing components.

Text input

"What animal is this?"

Tokenizer

Token embeddings

Image input

cat.jpg

Patches

Vision encoder

Adapter

Multimodal language model

Combine visual and linguistic information

Output: "This is a cat."

This is the fundamental end-to-end architecture covered in pages 20–22 of your PDF.

However, the diagram is conceptual. Real models can use different fusion mechanisms, visual-token formats, and preprocessing strategies.

### 25. What actually happens when the two modalities interact?

Let's look at a slightly more complicated example.

You upload a screenshot showing:

```
Order ID: 82931

Total: ₹4,999

Payment Status: FAILED
Reason: Insufficient Balance
```

And ask:

> Has this order been paid successfully?

The vision-processing system extracts representations of the screenshot.

Some representations preserve visual patterns associated with text, while others help retain layout and structural information.

The language model also processes your question.

The multimodal model then uses both information sources to produce an answer.

Conceptually, it needs to relate:

```
Question concept:
    Payment success

Relevant visual information:
    "Payment Status: FAILED"

Supporting information:
    "Reason: Insufficient Balance"
```

The answer becomes:

> No, the payment was unsuccessful. The screenshot indicates that it failed because of insufficient balance.

The key is that the text prompt directs the model toward relevant visual evidence.

A user could ask a completely different question about the same screenshot:

- What is the order ID?
- What is the total amount?
- Is the payment successful?
- What does the error mean?
- What should the customer do next?

The image remains the same.

The required output changes because the user's instruction changes.

### 26. Self-attention vs cross-attention in multimodal systems

The lecturer primarily discusses self-attention among image patches.

That is important, but there are two related concepts.

Self-attention: Representations within a sequence attend to one another.

For example, image patches inside a Vision Transformer can exchange information.

Cross-attention: Representations from one sequence attend to representations from another.

For example, language representations may attend to visual features.

Not every multimodal model uses a dedicated cross-attention module.

In a projection-based design, image representations can instead be inserted as visual tokens into the language model's input sequence. The language Transformer can then use its normal attention mechanism across the relevant tokens, subject to its attention mask.

The distinction matters when you read research papers about architectures such as LLaVA and other vision-language models.

## Part 8 — How does the model learn that an image contains a cat?

### 27. Is a vision encoder programmed with rules about cats?

No.

A conventional neural-network vision encoder does not normally contain explicit handwritten rules such as:

```
if has_pointed_ears and has_whiskers:    return "cat"
```

Instead, the network learns patterns from training.

The lecturer explains this at approximately 25:50–28:40 using paired images and descriptions.

For example:

```
Image: Cat photograph
Caption: "A cat sitting on a sofa."

Image: Dog photograph
Caption: "A dog running in a park."
```

Across training examples, the system can learn associations between visual patterns and linguistic concepts.

### 28. What is vision-language alignment?

Vision-language alignment means learning useful relationships between visual representations and linguistic representations.

For example:

```
Visual information:
    Fur
    Face shape
    Ears
    Whiskers

Language concepts:
    Cat
    Kitten
    Feline
    Pet
```

The training objective encourages the model to relate the correct visual and linguistic information.

One well-known approach is contrastive training, used in models such as CLIP.

Conceptually, if we have an image of a cat, its learned representation should be more strongly associated with the matching caption than with an unrelated caption.

For example:

```
Image: [Cat]

Text A: "A cat sitting on a table."
Text B: "An aircraft flying through the sky."
```

Contrastive learning encourages matching image-text pairs to have higher similarity than mismatched pairs.

This is one approach, not the complete training method for every modern multimodal LLM.

Other training stages may include visual feature pretraining, image-text instruction tuning, language modeling with visual context, and additional optimization.

### 29. What changes during training?

The neural network adjusts its learned parameters.

This includes, depending on the architecture and training procedure:

- Vision encoder weights.
- Projection or adapter weights.
- Language model weights, or selected portions of them.

Suppose the system incorrectly answers "dog" when shown a cat.

During training, a loss function quantifies how far the predictions are from the intended training objective.

Gradient-based optimization changes the trained parameters to reduce the loss.

After many updates across diverse examples, the model can improve at mapping visual information to appropriate language outputs.

This is considerably more sophisticated than remembering exact RGB values from previously seen cat photographs.

A model needs to generalize across varying lighting, camera positions, backgrounds, colors, and poses.

## Part 9 — How multimodal LLMs read screenshots and recognize text

### 30. How does the model recognize text in an image?

Consider an error screenshot:

```
Traceback (most recent call last):

File "app.py", line 14

NameError: name 'user_id' is not defined
```

There are two important tasks.

First, the model must obtain the relevant visual information from the screenshot.

Second, it must interpret the information in the context of the question.

Modern vision-language models can learn visual features associated with characters, words, document structures, and UI elements.

Depending on the implementation, the model may directly process visual representations or use auxiliary OCR-like mechanisms.

The lecturer correctly emphasizes that you do not always need to run a separate OCR engine before calling a vision-capable LLM.

### 31. Why does a screenshot require more than character recognition?

Suppose the screenshot contains:

```
Username: admin

Password: ********

Error: Invalid credentials
```

Recognizing the words is one part of the problem.

The model also needs to understand that the error relates to the authentication attempt.

That requires using relationships between text, position, and interface structure.

This is particularly useful for debugging screenshots, invoices, dashboard screenshots, technical diagrams, and web interfaces.

### 32. Can the model read every detail perfectly?

No.

Your PDF correctly identifies challenges such as blurry text, poor lighting, cropped information, low contrast, and densely populated screenshots.

A model may misread:

```
0 vs O
1 vs l
5 vs S
```

It can also misunderstand line numbers or relationships between UI elements.

For code debugging, this matters enormously.

If you upload a screenshot of an error, the model's explanation might be useful, but it should not be treated as guaranteed evidence of exactly what happened.

Where possible, providing the actual source code and stack trace as text is usually better than relying only on an image.

### 33. Why are large images resized, cropped, or tiled?

This relates to the final section of your SVG:

```
Raw Image
    |
    v
Compress / Preprocess
    |
    v
Manageable Image Representation
    |
    v
Vision Processing
```

A large image can contain enormous amounts of information.

For example, a screenshot with 8000 × 6000 pixels has:

\\[ 48,000,000\text{ pixels} \\]

Processing every region at full resolution can be expensive.

Image preprocessing may include:

| Operation | What it does                           | Tradeoff                                         |
| --------- | -------------------------------------- | ------------------------------------------------ |
| Resize    | Changes image resolution               | May lose fine details                            |
| Crop      | Selects a region                       | Removes surrounding context                      |
| Tile      | Divides a large image into regions     | Requires managing multiple regions               |
| Normalize | Adjusts numerical input representation | Usually not intended to discard semantic content |
| Compress  | Reduces encoded data size              | Lossy compression may reduce fidelity            |

A model provider might process a low-resolution overview alongside higher-resolution image crops.

This can help preserve both global scene information and local details.

One distinction: compressing a JPEG file is not the same thing as reducing the number of visual tokens. These are separate operations, although both may be part of an image-processing workflow.

# Part 10 — Building Vision Chat, exactly as the lecturer explains

Now let's move from neural-network architecture to backend engineering.

The lecture switches to implementation at approximately 30:10.

The important architectural insight is:

You do not need to implement a vision encoder, patch tokenizer, attention calculation, or projection layer inside your Spring Boot application.

You integrate a model provider that already supports image understanding.

Spring AI provides abstractions to package messages containing text and media. The vision processing takes place in the model provider's implementation.

The current Spring AI documentation supports this `UserMessage` and `Media` approach.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://docs.spring.io\&sz=32)

Home

+1



### 34. The application's high-level architecture

```
                        USER
                          |
           Text + optional image upload
                          |
                          v
                   Browser / UI
                          |
                          | HTTP POST
                          | multipart/form-data
                          v
                   ChatController
                          |
                          v
                     ChatService
                          |
                   Build UserMessage
                          |
                   Attach Media (if any)
                          |
                   Build conversation
                          |
                          v
                     ChatClient
                          |
                          v
                   Model Provider
                          |
                          v
                   Multimodal Model
                          |
                     Text response
                          |
                          v
                   ChatService
                          |
                   Store response
                          |
                          v
                       Browser
```

Let's build the important parts.

### 35. Spring AI project setup

The lecturer uses Spring Boot, Spring AI, and GPT-4o mini.

Create a compatible Spring Boot project with Spring Web and the Spring AI OpenAI starter.

The Spring AI dependency is:

```
<dependency>
    <groupId>org.springframework.ai</groupId>
    <artifactId>spring-ai-starter-model-openai</artifactId>
</dependency>
```

Use the appropriate Spring AI BOM for version management.

For the current Spring AI 2.0.1 release, the relevant configuration properties include `spring.ai.openai.chat.model` rather than the older `spring.ai.openai.chat.options.model`.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://docs.spring.io\&sz=32)

Home

+1



In `application.properties`:

```
spring.application.name=vision-chat

server.port=8080

spring.ai.openai.api-key=${OPENAI_API_KEY}
spring.ai.openai.chat.model=gpt-4o-mini

spring.servlet.multipart.max-file-size=10MB
spring.servlet.multipart.max-request-size=10MB
```

The 10 MB upload limit is taken from the lecture.

Set the API key using an environment variable rather than hardcoding it into the repository.

```
export OPENAI_API_KEY="your-api-key"
```

### 36. ChatController — handling text and image requests

The lecturer creates two endpoints:

| HTTP method | Endpoint    | Purpose                         |
| ----------- | ----------- | ------------------------------- |
| POST        | `/api/chat` | Send text and an optional image |
| DELETE      | `/api/chat` | Clear conversation history      |

Here's a controller following that design:

```
package com.example.visionchat;

import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;

import java.io.IOException;

@RestController
@RequestMapping("/api/chat")
public class ChatController {

    private final ChatService chatService;

    public ChatController(ChatService chatService) {
        this.chatService = chatService;
    }

    @PostMapping(
        consumes = MediaType.MULTIPART_FORM_DATA_VALUE
    )
    public String chat(
        @RequestParam("message") String message,
        @RequestParam(
            value = "image",
            required = false
        ) MultipartFile image
    ) throws IOException {

        return chatService.chat(message, image);
    }

    @DeleteMapping
    public ResponseEntity<Void> clearHistory() {
        chatService.clearHistory();
        return ResponseEntity.noContent().build();
    }
}
```

Let's understand the important pieces.

`@RestController`

Registers a Spring MVC controller whose return values become HTTP response bodies.

`@PostMapping`

Handles the POST request.

`MultipartFile`

Represents the uploaded file received in a multipart HTTP request.

`required = false`

Allows the client to send only text, without attaching an image.

The request might look like:

```
POST /api/chat
Content-Type: multipart/form-data
```

With form fields:

```
message = "What can you see here?"
image   = cat.jpg
```

This is an important detail.

The frontend does not need to convert the image into a vector embedding.

It simply uploads the file.

### 37. ChatService — constructing the multimodal message

This is where most of the lecture's code discussion happens.

```
package com.example.visionchat;

import org.springframework.ai.chat.client.ChatClient;
import org.springframework.ai.chat.messages.*;
import org.springframework.ai.content.Media;

import org.springframework.core.io.ByteArrayResource;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Service;
import org.springframework.util.MimeTypeUtils;
import org.springframework.web.multipart.MultipartFile;
import org.springframework.web.server.ResponseStatusException;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

@Service
public class ChatService {

    private final ChatClient chatClient;

    // Single-conversation demonstration only.
    private final List<Message> history =
            new ArrayList<>();

    public ChatService(ChatClient.Builder builder) {
        this.chatClient = builder.build();
    }

    public synchronized String chat(
            String message,
            MultipartFile image
    ) throws IOException {

        UserMessage userMessage;

        if (image != null && !image.isEmpty()) {

            String contentType = image.getContentType();

            if (!List.of("image/png", "image/jpeg")
                    .contains(contentType)) {
                throw new ResponseStatusException(
                    HttpStatus.UNSUPPORTED_MEDIA_TYPE,
                    "Only PNG and JPEG images are supported"
                );
            }

            byte[] imageBytes = image.getBytes();

            ByteArrayResource imageResource =
                    new ByteArrayResource(imageBytes);

            Media media = new Media(
                MimeTypeUtils.parseMimeType(contentType),
                imageResource
            );

            userMessage = UserMessage.builder()
                    .text(message)
                    .media(media)
                    .build();

        } else {

            userMessage = new UserMessage(message);
        }

        // Form the request without changing history yet.
        List<Message> requestMessages =
                new ArrayList<>(history);

        requestMessages.add(userMessage);

        String response = chatClient.prompt()
                .system("""
                    You are a helpful AI assistant.
                    Analyze provided images carefully.
                    Answer clearly and accurately.
                    If information is not visible,
                    say that it is uncertain.
                    """)
                .messages(requestMessages)
                .call()
                .content();

        if (response == null) {
            throw new IllegalStateException(
                "Model returned no text response"
            );
        }

        // Commit a completed conversation turn.
        history.add(userMessage);
        history.add(new AssistantMessage(response));

        return response;
    }

    public synchronized void clearHistory() {
        history.clear();
    }
}
```

This follows the lecturer's basic architecture, with a few small improvements such as content-type checks and saving a turn only after the model call succeeds.

The synchronization makes access to this one shared list sequential, but it does not make the design appropriate for multiple users. We'll address that shortly.

### 38. Understanding the image-processing code line by line

This is probably the most important implementation detail in the entire video.

#### Step A: Read the uploaded image

```
byte[] imageBytes = image.getBytes();
```

A `MultipartFile` represents the uploaded content received by Spring.

Calling `getBytes()` retrieves its bytes into memory.

This does not convert the image into embeddings.

You are only reading the uploaded file content.

#### Step B: Create a resource

```
ByteArrayResource imageResource =
        new ByteArrayResource(imageBytes);
```

`ByteArrayResource` wraps the byte array in Spring's resource abstraction.

Why is this useful?

Because Spring AI's media representation can work with Spring `Resource` objects.

It provides a convenient way to retain the uploaded image data independently of the original multipart request.

#### Step C: Create a Media object

```
Media media = new Media(
    MimeTypeUtils.parseMimeType(contentType),
    imageResource
);
```

The `Media` object contains information needed to supply the attachment to the model.

The MIME type describes the media format.

For example:

```
image/png
image/jpeg
```

The resource supplies the actual content.

This distinction is important:

```
MIME type  -> What kind of content is this?
Resource   -> Where is the content?
```

In production, validating the actual file contents is also important; an HTTP content-type header alone is not proof that the uploaded bytes contain a safe, valid image.

#### Step D: Build a UserMessage

```
userMessage = UserMessage.builder()
        .text(message)
        .media(media)
        .build();
```

This constructs one logical user message with text and an attachment.

Conceptually:

```
UserMessage
  |
  |-- text: "What is in this image?"
  |
  |-- media:
        |
        |-- MIME: image/jpeg
        |
        |-- Resource: cat.jpg bytes
```

Notice something important.

The image is not converted into English text inside your Spring Boot application.

You are packaging it for a vision-capable model.

Also, the `builder()` usage here is the Builder design pattern, not some multimodal-specific machine-learning technique.

Spring AI's documented `UserMessage` builder supports both text and media.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://docs.spring.io\&sz=32)

Home

+1



#### Step E: Handle text-only input

```
userMessage = new UserMessage(message);
```

If no image is uploaded, you only need a text message.

Thus the same controller supports both ordinary chat and image-based chat.

#### Step F: Call the model

```
String response = chatClient.prompt()
        .system("You are a helpful AI assistant.")
        .messages(requestMessages)
        .call()
        .content();
```

Break it down:

| Method          | Purpose                                     |
| --------------- | ------------------------------------------- |
| `prompt()`      | Start building the request                  |
| `system(...)`   | Supply higher-level behavioral instructions |
| `messages(...)` | Supply the conversation messages            |
| `call()`        | Execute a synchronous model call            |
| `content()`     | Retrieve the generated textual content      |

The model provider receives the text and media in an appropriate request representation.

Internally, the model's vision-processing system performs the visual encoding and integration we studied earlier.

Spring AI also documents the fluent `ChatClient` message API.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://docs.spring.io\&sz=32)

Home

+1



## Part 11 — Maintaining conversation history manually

This is the final major part of the transcript, beginning at approximately 32:30.

### 39. Why does a chatbot need conversation history?

Imagine the following interaction.

First message:

```
User:
Hi, my name is Aditya.

Assistant:
Hello Aditya!
```

Second message:

```
User:
What is my name?
```

For a stateless model request, the second request does not automatically contain the first request's information.

If the backend sends only:

```
"What is my name?"
```

The model has no reliable way to know what name was previously supplied.

But suppose the backend sends:

```
[
  UserMessage("Hi, my name is Aditya."),
  AssistantMessage("Hello Aditya!"),
  UserMessage("What is my name?")
]
```

Now the model can use the supplied history.

This is the basis of manually maintained chat context.

### 40. Why is image history even more interesting?

Consider this conversation.

User · Message 1

Why is my payment failing?

payment-error.png

Payment Failed — Insufficient Balance

Assistant · Message 2

The screenshot indicates your payment failed because of insufficient balance.

User · Message 3

How can I fix it?

Assistant · Message 4

Check the available balance or choose another payment method, then retry the payment.

The last user message does not mention the screenshot, payment status, or amount.

Yet the assistant can continue the conversation because the relevant previous context is supplied again.

### 41. How is image history stored?

In the lecturer's implementation, the backend stores messages:

```
private final List<Message> history =
        new ArrayList<>();
```

The list may contain a `UserMessage` that includes both text and a `Media` object.

Conceptually:

```
history = [

  UserMessage {
      text: "Why did my payment fail?",
      media: [payment-error.png]
  },

  AssistantMessage {
      text: "The payment failed..."
  },

  UserMessage {
      text: "How can I fix it?"
  }

]
```

On the next model call, the backend sends the relevant list of messages again.

Notice that the image can remain part of the historical user message.

That is why the lecturer pays attention to converting the uploaded image into a reusable resource.

The model itself is not necessarily retaining the uploaded image between independent stateless requests. The application is maintaining the context.

### 42. Does the backend save embeddings for every image?

Not in the lecture's implementation.

This is a very important distinction.

The backend stores the user message and media information.

It does not manually create or save visual embeddings in a vector database.

When the corresponding image-containing message is sent to the model again, the provider handles image processing according to its API and architecture.

You should not confuse these three things:

| Mechanism               | What it stores or processes                                |
| ----------------------- | ---------------------------------------------------------- |
| Chat history            | Previous messages and potentially their attachments        |
| Vector database         | Indexed embeddings for similarity search                   |
| Model vision processing | Learned visual representations used during model inference |

They are not interchangeable.

### 43. Why should we not just keep every image forever?

Imagine a conversation with 50 images.

If the backend repeatedly sends all 50 images to the model, several problems can emerge.

There may be higher request cost, longer latency, context-window pressure, increased memory use, and unnecessary processing of old information.

The simplest implementation is useful for learning, but production applications should decide which historical visual information remains relevant.

Possible strategies include retaining recent turns, keeping references to uploaded files, extracting verified structured facts, or summarizing older context.

A text summary is cheaper than repeatedly supplying a large image, but it can discard useful visual information. This is a tradeoff.

### 44. The danger of one global `ArrayList`

The lecturer creates a simple list for history.

This is fine for a single-user demonstration.

But Spring services are normally singleton beans.

Suppose two users use your application:

If all messages go into the same list, User B may receive information from User A's conversation.

This is a privacy and correctness problem.

A production system should keep separate conversation histories.

Conceptually: