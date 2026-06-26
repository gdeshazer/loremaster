# Loremaster Architecture

Loremaster is a Go tool that turns a story writer's collection of markdown
files into a searchable knowledge base that an LLM (Claude) can query through
the Model Context Protocol (MCP). It indexes `.md` files into PostgreSQL,
generates vector embeddings via Ollama, and exposes both semantic and
full-text search over the resulting corpus.

This document describes the high-level structure of the system, the
responsibilities of each component, and how data flows from a file on disk to
a search result delivered to Claude.

---

## 1. System diagram

```
                         ┌──────────────────────────────────────────┐
                         │              loremaster binary           │
                         │                                          │
  ┌───────────────┐      │  ┌────────────┐        ┌──────────────┐  │
  │ Claude Desktop│◄────►│  │ MCP server │        │  CLI (Cobra) │  │
  │ / Claude Code │ stdio│  │ (mcp-go)   │        │  subcommands │  │
  └───────────────┘ JSON │  └─────┬──────┘        └──────┬───────┘  │
                    -RPC │        │                      │          │
                         │        └──────────┬───────────┘          │
                         │                   ▼                      │
                         │         ┌───────────────────┐            │
                         │         │   Core services   │            │
                         │         │                   │            │
                         │         │  ┌─────────────┐  │            │
   ┌─────────────┐       │         │  │   Ingest    │  │            │
   │  ./stories  │──────►│─────────┼─►│  pipeline   │  │            │
   │   *.md      │ walk  │         │  └──────┬──────┘  │            │
   └─────────────┘       │         │         ▼         │            │
                         │         │  ┌─────────────┐  │            │
                         │         │  │ Embed layer │◄─┼──┐         │
                         │         │  └──────┬──────┘  │  │         │
                         │         │         ▼         │  │         │
                         │         │  ┌─────────────┐  │  │         │
                         │         │  │  DB store   │  │  │         │
                         │         │  │  (pgx)      │  │  │         │
                         │         │  └──────┬──────┘  │  │         │
                         │         └─────────┼─────────┘  │         │
                         └───────────────────┼────────────┼─────────┘
                                             ▼            │ HTTP
                                  ┌─────────────────┐  ┌──┴──────────┐
                                  │  PostgreSQL 17  │  │   Ollama    │
                                  │  + pgvector     │  │ nomic-embed │
                                  │                 │  │   -text     │
                                  │  projects       │  └─────────────┘
                                  │  documents      │
                                  │  (HNSW + GIN)   │
                                  └─────────────────┘

         (Postgres + Ollama run as containers via Podman Compose)
```

---

## 2. Component responsibilities

### CLI (Cobra + Viper)

The user-facing surface. Cobra defines the command tree (`init`, `serve`,
`index`, `search`, `status`, `projects ...`) and Viper resolves configuration
across flags, environment variables, and the project-local `loremaster.json`.
The CLI is the entry point for humans: indexing content, inspecting status,
running one-off searches, and managing projects.

### MCP server (`mark3labs/mcp-go`)

Started by `loremaster serve`. Speaks JSON-RPC over **stdio** to an MCP client
(Claude Desktop or Claude Code). It registers six tools — `semantic_search`,
`keyword_search`, `hybrid_search`, `get_document`, `list_documents`,
`list_projects` — and translates each tool call into a query against the core
services. The MCP server is read-only with respect to content: it never
ingests or mutates documents, it only retrieves.

### Ingest pipeline

Responsible for turning files into chunked, embeddable records:

1. **Walk** the target directory tree, honoring `exclude` globs.
2. **Parse** each `.md` file with `yuin/goldmark`, extracting YAML
   frontmatter and the rendered text body. Title and tags are derived
   automatically when frontmatter omits them: the title falls back to the first
   H1 heading then the filename stem; tags fall back to the directory path
   components (e.g. `lore/magic/` → `lore, magic`).
3. **Chunk** the body into ~400-token sliding windows with 50-token overlap,
   preferring sentence boundaries.
4. Hand each chunk to the embed layer, then persist via the DB store.

The pipeline is idempotent per `(project_id, file_path, chunk_index)`: re-running
`index` upserts rather than duplicating.

### Embed layer

Wraps the Ollama HTTP API. Given chunk text, it requests an embedding from the
configured model (default `nomic-embed-text`, 768 dimensions) and returns a
`pgvector.Vector`. This layer is the single place that knows the embedding
model and vector dimensionality, so swapping models is localized. It is used
both at ingest time (to embed chunks) and at query time (to embed the search
query for semantic search).

### DB store (`jackc/pgx/v5` + `pgvector/pgvector-go`)

Owns all SQL. It manages the connection pool, runs schema migrations, and
exposes typed methods for upserting documents, and for the three search
strategies. Vector values are marshaled through `pgvector-go`. All queries are
scoped by `project_id`.

---

## 3. Data flow: file → chunk → search result

### Ingest path (write)

```
 ./stories/characters/aria.md
        │
        ▼  goldmark parse
 ┌───────────────────────────────┐
 │ frontmatter: {title, tags,    │
 │   characters, location, date} │
 │ body:        "Aria grew up..." │
 └───────────────┬───────────────┘
        │  chunk (≈400 tok, 50 overlap, sentence-aware)
        ▼
 chunk[0] "Aria grew up in the..."   chunk[1] "...the river district where..."
        │                                   │
        ▼  embed layer → Ollama             ▼
 vector(768)                          vector(768)
        │                                   │
        ▼  DB store: INSERT ... ON CONFLICT (project_id, file_path, chunk_index)
        ▼
 documents row:
   project_id, file_path, chunk_index, title, content,
   embedding VECTOR(768), fts TSVECTOR (generated), metadata JSONB
```

The `fts` column is computed by PostgreSQL automatically (`GENERATED ALWAYS AS
to_tsvector('english', content) STORED`), so the application only writes
`content`.

### Query path (read)

```
 Claude calls hybrid_search(query="Where did Aria grow up?", project="my-novel")
        │
        ▼  resolve project slug → project_id
        ├──────────────────────────────┐
        ▼ semantic branch              ▼ keyword branch
 embed query → vector(768)      to_tsquery('english', ...)
        │                              │
        ▼ ORDER BY embedding           ▼ ORDER BY ts_rank(fts, query)
   <=> q  (cosine, HNSW)               (GIN index)
        │                              │
        └──────────────┬───────────────┘
                       ▼  Reciprocal Rank Fusion (RRF)
              merged, re-ranked result set
                       │
                       ▼
        [{file_path, chunk_index, title, content, score, metadata}, ...]
                       │
                       ▼  formatted as MCP tool result → Claude
```

For pure `semantic_search` only the left branch runs; for `keyword_search`
only the right branch runs.

---

## 4. Dual-mode binary: MCP stdio vs. CLI

`loremaster` is a single binary that operates in two modes, selected by the
subcommand:

| Invocation             | Mode      | Transport          | Consumer            |
|------------------------|-----------|--------------------|---------------------|
| `loremaster serve`     | MCP server| stdio (JSON-RPC)   | Claude Desktop/Code |
| `loremaster <cmd>`     | CLI       | terminal stdout    | a human             |

Both modes share the same core services (ingest, embed, DB store) and the same
configuration resolution. The only difference is the outer shell:

- In **CLI mode**, Cobra parses `os.Args`, runs the requested command, prints
  human-readable output to stdout, and exits.
- In **MCP mode** (`serve`), the process stays alive and reads JSON-RPC
  messages from stdin / writes responses to stdout. **Nothing else may write
  to stdout** in this mode — diagnostic logging must go to stderr, or it will
  corrupt the JSON-RPC stream. This is the single most important operational
  constraint of MCP stdio servers.

Because the search logic is identical between modes, `loremaster search` is
effectively a CLI harness over the same code the MCP `*_search` tools call,
which makes it the fastest way to debug retrieval behavior without involving
Claude.

---

## 5. Project isolation

Projects are first-class. Every document belongs to exactly one project, and
every query is scoped to a single project.

- The `projects` table holds `slug` (stable, URL-safe identifier), `name`, and
  `description`.
- `documents.project_id` is a `BIGINT` foreign key referencing `projects.id`
  with `ON DELETE CASCADE`.
- A unique constraint on `(project_id, file_path, chunk_index)` means the same
  file path can exist independently in different projects without collision.
- Search SQL always includes `WHERE project_id = $1`, and the `project_id`
  index keeps that filter cheap. The HNSW and GIN indexes then operate on the
  project-filtered set.

This gives each story project an isolated namespace within one shared
database. Deleting a project (`loremaster projects delete <slug>`) removes the
`projects` row, and the cascade automatically deletes every associated
`documents` row — no orphan cleanup logic required. See
[data-model.md](./data-model.md) for the rationale behind the foreign key and
cascade design.

The active project is determined by configuration: the `project` field in the
nearest `loremaster.json` (found by walking up the directory tree, like
`.git`), overridable by `--project` or `LOREMASTER_PROJECT`. See
[ops.md](./ops.md) for the full configuration reference.
