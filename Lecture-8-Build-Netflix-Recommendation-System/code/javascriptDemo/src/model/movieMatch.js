// A search/similarity result returned to the client.
export class MovieMatch {
    /**
     * @param {{ title: string, description: string, match: number }} fields
     */
    constructor({ title, description, match }) {
        this.title = title;
        this.description = description;
        this.match = match;
    }
}
