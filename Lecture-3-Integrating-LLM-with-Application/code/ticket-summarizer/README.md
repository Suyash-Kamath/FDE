# Ticket Summarizer (Go)

This is the Go equivalent of the Spring Boot ticket summarizer. It exposes:

```text
POST /api/summarize
Content-Type: text/plain
```

## Run it

Create a `.env` file in this directory:

```dotenv
OPENAI_API_KEY=your-api-key
OPENAI_MODEL=gpt-4o-mini
```

Then start the server:

```bash
go run .
```

The model entry is optional because `gpt-4o-mini` is the default. Variables
already exported in your shell take precedence over values in `.env`.

You can alternatively configure the application through your shell:

```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_MODEL="gpt-4o-mini"
go run .
```

Send a ticket as plain text:

```bash
curl -X POST http://localhost:8080/api/summarize \
  -H "Content-Type: text/plain" \
  --data "The printer is offline and restarting it did not fix the issue."
```

Run the tests with:

```bash
go test ./...
```
