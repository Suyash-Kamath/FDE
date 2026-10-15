// A catalog entry together with its precomputed embedding vector.
export class Movie {
    /**
     * @param {{ title: string, description: string, embedding: number[] }} fields
     */
    constructor({ title, description, embedding }) {
        this.title = title;
        this.description = description;
        this.embedding = embedding;
    }
}
