# Loremaster

Loremaster turns a story writer's collection of markdown files into an LLM-queryable knowledge base. It indexes `.md` files into PostgreSQL (pgvector + full-text search), generates vector embeddings via Ollama, and exposes semantic and keyword search in two ways:

- **MCP server** — Claude (Desktop or Code) can call search tools directly while writing or editing your story
- **CLI** — run searches yourself from the terminal

## How it works

Each markdown file is parsed, split into ~400-token chunks, and embedded into a vector. PostgreSQL stores both the embedding (for semantic search via cosine similarity) and a full-text index (for keyword search). Searches can be run against either index independently, or fused together with Reciprocal Rank Fusion (RRF) via `hybrid_search`.

Projects are isolated namespaces — you can have multiple stories in one database and queries never cross project boundaries.

## Prerequisites

- Go 1.26+
- [Podman](https://podman.io/) or Docker (for PostgreSQL + Ollama containers)
- `podman-compose` or `docker compose`

## Getting started

### 1. Start the infrastructure

```sh
podman compose up -d
podman exec loremaster-ollama-1 ollama pull nomic-embed-text
```

This starts a PostgreSQL 17 instance with the pgvector extension and an Ollama instance with the `nomic-embed-text` embedding model.

### 2. Set environment variables

Copy `.env.example` and export the variables, or set them in your shell:

```sh
export LOREMASTER_DB_URL="postgres://loremaster:loremaster@localhost:5432/loremaster?sslmode=disable"
export LOREMASTER_OLLAMA_URL="http://localhost:11434"
```

### 3. Install the binary

```sh
go install github.com/gdeshazer/loremaster@latest
```

Or build from source:

```sh
go build -o loremaster .
```

### 4. Initialize a project

Run `init` from your story's root directory. It creates the database schema on first run, registers the project, and writes three files:

```sh
cd ~/stories/my-novel
loremaster init
```

This produces:

- **`loremaster.json`** — project-local config (slug, model, optional DB/Ollama URL overrides)
- **`mcp.json`** — ready-to-paste MCP server config block for Claude
- **`CLAUDE.md`** — tool guidance appended for Claude to use when working in this directory

To embed connection URLs directly in `loremaster.json` (so no env vars are needed later):

```sh
loremaster init --db-url "postgres://user:pass@localhost:5432/loremaster?sslmode=disable" \
                --ollama-url "http://localhost:11434"
```

### 5. Index your files

```sh
loremaster index .
```

Re-indexing is idempotent — files are upserted by `(project, file_path, chunk_index)`.

### 6. Search from the CLI

```sh
loremaster search "how does the magic system work"
loremaster search "Roland's backstory" --limit 10
```

Results are printed as JSON with `file_path`, `chunk_index`, `title`, `content`, `score`, and frontmatter `metadata`.

### 7. Connect Claude via MCP

Add the contents of the generated `mcp.json` to your Claude Code or Claude Desktop MCP settings. Then start the MCP server manually if needed:

```sh
loremaster serve
```

Claude Code picks up `CLAUDE.md` automatically and knows which search tool to use for different query types.

## CLI reference

| Command | Description |
|---|---|
| `loremaster init` | Initialize a project in the current directory |
| `loremaster index [path]` | Index markdown files into the database |
| `loremaster search [query]` | Run a hybrid search and print JSON results |
| `loremaster serve` | Start the MCP server (stdio JSON-RPC) |
| `loremaster status` | Show project and database status |
| `loremaster projects list` | List all projects with document counts |
| `loremaster projects describe [slug]` | Show metadata for a project |
| `loremaster projects delete [slug]` | Delete a project and all its indexed documents |

## MCP tools

When connected via MCP, Claude can call these tools:

| Tool | When to use |
|---|---|
| `hybrid_search` | Default — combines semantic + keyword via RRF |
| `semantic_search` | Conceptual or thematic questions |
| `keyword_search` | Exact names, invented terms, specific phrases |
| `get_document` | Retrieve a full file by path |
| `list_documents` | Browse all indexed files in a project |
| `list_projects` | List all projects in the database |

All tools accept a `project` parameter (the slug from `loremaster.json`) and search tools accept an optional `filter` object to constrain results by frontmatter fields (e.g. `{"characters": "Aria", "tags": "lore"}`).

## Configuration

Configuration is resolved in this order (highest priority wins):

```
--flag  >  loremaster.json  >  LOREMASTER_* env var  >  built-in default
```

| Environment variable | Default | Description |
|---|---|---|
| `LOREMASTER_DB_URL` | _(required)_ | PostgreSQL connection string |
| `LOREMASTER_OLLAMA_URL` | `http://localhost:11434` | Ollama base URL |
| `LOREMASTER_OLLAMA_MODEL` | `nomic-embed-text` | Embedding model name |
| `LOREMASTER_EMBED_DIMS` | `768` | Embedding dimensions |
| `LOREMASTER_PROJECT` | _(from loremaster.json)_ | Project slug |

`loremaster.json` fields (`db_url` and `ollama_url` override their env var equivalents and are only written when you pass the flag explicitly to `init`):

```json
{
  "project": "my-novel",
  "db_url": "",
  "ollama_url": "",
  "embedding_model": "nomic-embed-text",
  "exclude": ["drafts/**", "*.tmp"]
}
```

Loremaster walks up the directory tree to find `loremaster.json`, the same way git finds `.git`, so commands work from any subdirectory of your project.

## Running tests

Integration tests use Testcontainers to spin up a real PostgreSQL instance:

```sh
TESTCONTAINERS_RYUK_DISABLED=true go test ./...
```

The `TESTCONTAINERS_RYUK_DISABLED=true` flag is required when running under Podman.
