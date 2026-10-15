// Mirrors one entry of resources/movies.json.
export class MovieData {
    /**
     * @param {{ title: string, description: string }} fields
     */
    constructor({ title, description }) {
        // Minimal stand-in for pydantic validation.
        if (typeof title !== "string" || typeof description !== "string") {
            throw new TypeError("MovieData requires string 'title' and 'description'");
        }
        this.title = title;
        this.description = description;
    }
}
