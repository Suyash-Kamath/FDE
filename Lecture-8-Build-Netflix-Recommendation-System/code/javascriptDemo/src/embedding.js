// EmbeddingClient "interface" (the Python version uses a typing.Protocol):
// any object with an async `embed(text) -> number[]` method will do.
// This keeps MovieService independent of OpenAI.

export class OpenAIEmbeddingClient {
    /**
     * @param {import("openai").default} client
     * @param {string} model
     */
    constructor(client, model) {
        this._client = client;
        this._model = model;
    }

    /**
     * @param {string} text
     * @returns {Promise<number[]>}
     */
    async embed(text) {
        const response = await this._client.embeddings.create({ model: this._model, input: text });
        return response.data[0].embedding;
    }
}
