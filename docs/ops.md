# Loremaster Operations Guide

How to install, configure, and run Loremaster: bringing up the backing
services, initializing a project, indexing markdown, wiring the MCP server into
Claude, and troubleshooting the common failures.

---

## 1. Prerequisites

| Requirement   | Version / notes                                                               |
|---------------|-------------------------------------------------------------------------------|
| Podman        | With `podman compose` (or `podman-compose`). Runs PostgreSQL + Ollama.         |
| Go            | 1.22 or newer, to build the `loremaster` binary.                              |
| PostgreSQL    | 17 with the `pgvector` extension — provided by the Compose file; no host install needed. |
| Ollama        | Optional if you use a cloud embedding provider. Required for local embeddings (the default). |
| Embedding model | `nomic-embed-text` (768 dims) by default. Pulled into Ollama during setup.   |

> Docker works too if you prefer it — substitute `docker compose` for
> `podman compose` throughout. The Compose file is engine-agnostic.

---

## 2. First-time setup

### 2.1 Start the backing services

From the repository root (where `compose.yaml` lives):

```bash
podman compose up -d
```

This starts two containers:

- **postgres** — PostgreSQL 17 with `pgvector`, exposing `5432`, with a named
  volume for persistence.
- **ollama** — the embedding server, exposing `11434`, with a volume for pulled
  models.

Check they are healthy:

```bash
podman compose ps
```

### 2.2 Pull the embedding model

The Ollama container starts empty. Pull the default model once:

```bash
podman compose exec ollama ollama pull nomic-embed-text
```

(If you run Ollama natively instead of in a container, just
`ollama pull nomic-embed-text`.)

### 2.3 Configure connection URLs

Loremaster needs to know how to reach Postgres and Ollama. You can supply these
in either `loremaster.json` (preferred for project-local use) or environment
variables (global fallback).

**Option A — embed in `loremaster.json`** (no env vars needed):
```bash
loremaster init \
  --db-url "postgres://loremaster:loremaster@localhost:5432/loremaster?sslmode=disable" \
  --ollama-url "http://localhost:11434"
```
This writes `db_url` and `ollama_url` directly into the generated
`loremaster.json` so any command run from inside the project directory works
without environment variables being set.

**Option B — environment variables** (global, useful for CI or shared shells):
```bash
export LOREMASTER_DB_URL="postgres://loremaster:loremaster@localhost:5432/loremaster?sslmode=disable"
export LOREMASTER_OLLAMA_URL="http://localhost:11434"
export LOREMASTER_OLLAMA_MODEL="nomic-embed-text"
```

### 2.4 Build the binary

```bash
go build -o loremaster ./cmd/loremaster
# optionally: go install ./cmd/loremaster
```

Verify connectivity:

```bash
./loremaster status
```

`status` reports whether the database is reachable, the schema is migrated, and
Ollama responds with the expected model.

---

## 3. Initialize a new project

Run `init` from the directory that will hold your story's markdown (typically
the project root):

```bash
cd ~/writing/my-novel
loremaster init --slug my-novel --name "My Novel" \
  --description "Epic fantasy set in the river cities."
```

This does two things:

1. **Creates the project row** in the `projects` table (slug, name,
   description).
2. **Writes two files** into the current directory:

   - **`loremaster.json`** — project-local config. Committed alongside your
     prose so collaborators share settings. When `db_url` and `ollama_url` are
     present, no environment variables are needed to run loremaster commands.

     ```json
     {
       "project": "my-novel",
       "db_url": "postgres://loremaster:loremaster@localhost:5432/loremaster?sslmode=disable",
       "ollama_url": "http://localhost:11434",
       "embedding_model": "nomic-embed-text",
       "exclude": ["drafts/**", "**/.obsidian/**"]
     }
     ```

   - **`mcp.json`** — an MCP server definition pointing Claude at
     `loremaster serve` for this project (see §5).

     ```json
     {
       "mcpServers": {
         "loremaster": {
           "command": "loremaster",
           "args": ["serve", "--project", "my-novel"],
           "env": {
             "LOREMASTER_DATABASE_URL": "postgres://loremaster:loremaster@localhost:5432/loremaster?sslmode=disable",
             "LOREMASTER_OLLAMA_URL": "http://localhost:11434"
           }
         }
       }
     }
     ```

`loremaster.json` is discovered by walking **up** the directory tree from the
current working directory (like Git finding `.git`), so you can run commands
from any subfolder of the project and still get the right configuration.

---

## 4. Index your markdown

Point `index` at the directory containing your `.md` files:

```bash
loremaster index ./stories
# or, from anywhere in the project tree:
loremaster index .
```

What happens:

1. Walk the tree, skipping anything matching the `exclude` globs in
   `loremaster.json`.
2. Parse each `.md` file (frontmatter + body) with goldmark.
3. Chunk the body (~400 tokens, 50-token overlap, sentence-aware).
4. Embed each chunk via Ollama.
5. Upsert chunks keyed on `(project_id, file_path, chunk_index)` — re-running is
   idempotent and only changed chunks are re-embedded.

Confirm the result:

```bash
loremaster status                       # totals for the active project
loremaster search "the guild towers"    # quick retrieval sanity check
```

Useful project management commands:

```bash
loremaster projects list                # all projects + document counts
loremaster projects describe my-novel   # details for one project
loremaster projects delete my-novel     # removes project AND all its documents (cascade)
```

---

## 5. Wire Loremaster into Claude

Both clients consume the generated `mcp.json` (or its values).

### Claude Code

From the project directory (where `mcp.json` was written), Claude Code picks up
project-scoped MCP servers automatically. To register explicitly:

```bash
claude mcp add loremaster -- loremaster serve --project my-novel
```

Or copy the `mcpServers.loremaster` block from `mcp.json` into your Claude Code
MCP configuration. Confirm the tools appear, then ask Claude to search your
notes.

### Claude Desktop

Edit Claude Desktop's `claude_desktop_config.json` and merge in the
`mcpServers.loremaster` entry from the generated `mcp.json`:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

Use an **absolute path** for `command` (e.g. `/usr/local/bin/loremaster`) since
Claude Desktop does not inherit your shell `PATH`. Restart Claude Desktop. The
six Loremaster tools (`semantic_search`, `keyword_search`, `hybrid_search`,
`get_document`, `list_documents`, `list_projects`) should now be available.

> **stdio caution.** `loremaster serve` speaks JSON-RPC over stdout. Do not run
> it interactively in a terminal expecting human output, and never let other
> tooling write to its stdout — diagnostics go to stderr. See
> [architecture.md](./architecture.md) §4.

---

## 6. Configuration reference

Configuration is resolved with this priority (highest wins):

```
--flag  >  loremaster.json  >  LOREMASTER_* env var  >  built-in default
```

`loremaster.json` overrides environment variables, which allows a project
directory to be fully self-contained — no environment variables needed once
the file has `db_url` and `ollama_url` set.

### `loremaster.json` fields

| Field             | Type            | Default                  | Description                                               |
|-------------------|-----------------|--------------------------|-----------------------------------------------------------|
| `project`         | string          | (none)                   | Project slug this directory belongs to.                   |
| `db_url`          | string          | (from env or flag)       | PostgreSQL connection string. Overrides `LOREMASTER_DB_URL`. |
| `ollama_url`      | string          | `http://localhost:11434` | Ollama base URL. Overrides `LOREMASTER_OLLAMA_URL`.       |
| `embedding_model` | string          | `nomic-embed-text`       | Per-project model override.                               |
| `exclude`         | array of globs  | `[]`                     | Path globs to skip during `index` (e.g. `drafts/**`).    |

Example `loremaster.json` with all connection fields:
```json
{
  "project": "my-novel",
  "db_url": "postgres://loremaster:loremaster@localhost:5432/loremaster?sslmode=disable",
  "ollama_url": "http://localhost:11434",
  "embedding_model": "nomic-embed-text",
  "exclude": ["drafts/**", "**/.obsidian/**"]
}
```

### Environment variables (global fallback)

| Variable                  | Type   | Default                  | Description                                          |
|---------------------------|--------|--------------------------|------------------------------------------------------|
| `LOREMASTER_DB_URL`       | string | (required if not in JSON)| PostgreSQL connection string.                        |
| `LOREMASTER_OLLAMA_URL`   | string | `http://localhost:11434` | Base URL of the Ollama server.                       |
| `LOREMASTER_OLLAMA_MODEL` | string | `nomic-embed-text`       | Embedding model name (must produce 768-dim vectors). |
| `LOREMASTER_PROJECT`      | string | (from `loremaster.json`) | Active project slug.                                 |
| `LOREMASTER_EMBED_DIMS`   | string | `768`                    | Embedding dimensions.                                |

### Common flags

| Flag              | Applies to    | Description                          |
|-------------------|---------------|--------------------------------------|
| `--project <slug>`| most commands | Override the active project.         |
| `--limit <n>`     | `search`      | Max results to return.               |
| `--db-url`        | all           | Override DB URL (highest priority).  |
| `--ollama-url`    | all           | Override Ollama URL (highest priority).|

---

## 7. Troubleshooting

### Database connection errors

*Symptom:* `connection refused`, `dial tcp ... :5432`, or `status` reports the
DB unreachable.

- Confirm the container is up: `podman compose ps`; restart with
  `podman compose up -d`.
- Verify `LOREMASTER_DATABASE_URL` host/port/credentials match the Compose
  file. The default user/db/password is `loremaster`.
- For a local container, `sslmode=disable` is usually required.
- Check for a port clash on `5432` (another Postgres running):
  `lsof -i :5432`.

### `relation "documents" does not exist` / extension errors

- The schema hasn't been migrated, or `pgvector` isn't installed. Run
  `loremaster status` (it applies migrations) and ensure the Postgres image
  includes `pgvector`. `CREATE EXTENSION vector;` must succeed.

### Ollama not running / model missing

*Symptom:* embedding requests fail, `connection refused` on `:11434`, or
`model "nomic-embed-text" not found`.

- Confirm the container: `podman compose ps`; check
  `curl http://localhost:11434/api/tags`.
- Pull the model:
  `podman compose exec ollama ollama pull nomic-embed-text`.
- Ensure `LOREMASTER_OLLAMA_URL` points at the right host/port.

### Embedding dimension mismatch

*Symptom:* `expected 768 dimensions, got N`, or inserts rejected by the
`VECTOR(768)` column.

- The `embedding` column is fixed at **768** dimensions to match
  `nomic-embed-text`. If you switch to a model with a different output size, the
  vectors won't fit.
- Either keep a 768-dim model, **or** change the column type
  (`ALTER TABLE documents ALTER COLUMN embedding TYPE VECTOR(<dims>)`), recreate
  the HNSW index, and **re-index all content** — embeddings from different
  models are not comparable, so a full re-embed is mandatory after a model
  change.
- Don't mix models within a project: a project's chunks must all be embedded by
  the same model for similarity scores to be meaningful.

### MCP tools don't appear in Claude

- Use an absolute path for `command` in `mcp.json` (Claude Desktop lacks your
  shell `PATH`).
- Ensure nothing else writes to the server's stdout; logs must go to stderr.
- Restart the client after editing its config.
- Test the same query outside Claude with `loremaster search` to isolate
  whether the problem is retrieval or the MCP wiring.

### Searches return nothing

- Confirm content is indexed: `loremaster status` and `loremaster projects
  list` should show non-zero document counts.
- Check you're querying the right project (`--project` / `LOREMASTER_PROJECT`).
- If using a `filter`, loosen or remove it — an over-tight metadata filter can
  exclude every chunk. See [mcp-tools.md](./mcp-tools.md).
