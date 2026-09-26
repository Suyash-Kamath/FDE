import os
import json

import dotenv
dotenv.load_dotenv()

from langchain_google_genai import GoogleGenerativeAIEmbeddings
from pinecone import Pinecone
from google import genai
from google.genai import types


ai = genai.Client()
History = []


def transform_query(question):

    History.append({
        "role": "user",
        "parts": [{"text": question}]
    })

    response = ai.models.generate_content(
        model="gemini-3.5-flash",
        contents=History,
        config=types.GenerateContentConfig(
            system_instruction="""You are a query rewriting expert. Based on the provided chat history, rephrase the "Follow Up user Question" into a complete, standalone question that can be understood without the chat history.
    Only output the rewritten question and nothing else.
      """,
        ),
    )

    History.pop()

    return response.text


# ==========================================================
# Embeddings wrapper : force 768-dim output
# ==========================================================

"""
  Same fix as index.py: gemini-embedding-001 returns 3072 dims by
  default, but the Pinecone index is 768-dim. The query vector has to
  be truncated the exact same way the stored vectors were, or the
  Pinecone query call itself rejects it with the same dimension error
  you just hit.
"""


class Gemini768Embeddings(GoogleGenerativeAIEmbeddings):

    # Note the `**kwargs` - the parent's embed_documents/embed_query take
    # keyword-only arguments (task_type, titles, output_dimensionality).
    # Passing them through keeps the override signature-compatible.

    def embed_documents(self, texts, **kwargs):
        vectors = super().embed_documents(texts, **kwargs)
        return [v[:768] for v in vectors]

    def embed_query(self, text, **kwargs):
        vector = super().embed_query(text, **kwargs)
        return vector[:768]


# Created once, outside the loop - no need to spin up a new embeddings
# client and a new Pinecone connection on every single question.
embeddings = Gemini768Embeddings(
    google_api_key=os.getenv("GEMINI_API_KEY"),
    model="gemini-embedding-001",  # must match whatever index.py used to embed the stored chunks
    output_dimensionality=768,
)

pinecone = Pinecone(api_key=os.getenv("PINECONE_API_KEY"))
pinecone_index = pinecone.Index(os.getenv("PINECONE_INDEX_NAME"))


def chatting(question):

    queries = transform_query(question)

    # convert this question into vector
    query_vector = embeddings.embed_query(queries)

    # search pinecone
    # Note the snake_case: Python uses top_k / include_metadata where the
    # JS SDK used topK / includeMetadata.
    search_results = pinecone_index.query(
        top_k=10,
        vector=query_vector,
        include_metadata=True,
    )

    print("\n--- Raw Pinecone matches (JSON) ---\n")
    print(json.dumps(search_results.to_dict(), indent=2))

    # .get("text", "") instead of .metadata.text - a match with no `text`
    # key would raise AttributeError/KeyError and kill the whole turn.
    context = "\n\n---\n\n".join(
        (match.metadata or {}).get("text", "")
        for match in search_results.matches
    )

    print("\n--- Retrieved context (readable) ---\n")
    print(context or "(no matches found)")
    print("\n------------------------------------\n")

    # NOTE: this currently only retrieves context — it doesn't call an
    # LLM yet to turn `context` + `question` into an actual answer.
    # Say the word if you want that generation step wired in next.
    History.append({
        "role": "user",
        "parts": [{"text": queries}]
    })

    response = ai.models.generate_content(
        model="gemini-3.5-flash",
        contents=History,
        config=types.GenerateContentConfig(
            system_instruction=f"""You have to behave like a Data Structure and Algorithm Expert.
    You will be given a context of relevant information and a user question.
    Your task is to answer the user's question based ONLY on the provided context.
    If the answer is not in the context, you must say "I could not find the answer in the provided document."
    Keep your answers clear, concise, and educational.

      Context: {context}
      """,
        ),
    )

    History.append({
        "role": "model",
        "parts": [{"text": response.text}]
    })

    print("\n")
    print(response.text)

    return context


def main():
    # The JS version called main() recursively at the end of itself.
    # In Python that hits RecursionError after ~1000 questions, so this is
    # a while loop instead. Same behaviour, no stack growth.
    while True:
        user_problem = input("Ask me anything--> ")

        try:
            chatting(user_problem)
        except Exception as error:
            print("⚠️  Something went wrong:", error)


if __name__ == "__main__":
    main()