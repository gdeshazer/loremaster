# Loremaster Data Model

This document describes the PostgreSQL schema that backs Loremaster, the
reasoning behind each design decision, the chunking strategy used during
ingest, and the frontmatter conventions that map into structured metadata.

Loremaster targets **PostgreSQL 17** with the **pgvector** extension. Vector
similarity search uses an HNSW index; full-text search uses a generated
`tsvector` column with a GIN index.

---

## 1. Schema

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE projects (
  id          BIGSERIAL PRIMARY KEY,
  slug        TEXT NOT NULL UNIQUE,
  name        TEXT NOT NULL,
  description TEXT,
  created_at  TIMESTAMPTZ DEFAULT NOW(),
  updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE documents (
  id          BIGSERIAL PRIMARY KEY,
  project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  file_path   TEXT NOT NULL,
  chunk_index INT  NOT NULL,
  title       TEXT,
  content     TEXT NOT NULL,
  embedding   VECTOR(768),
  fts         TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
  metadata    JSONB DEFAULT '{}',
  indexed_at  TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE (project_id, file_path, chunk_index)
);

CREATE INDEX ON documents USING hnsw (embedding vector_cosine_ops);
CREATE INDEX ON documents USING GIN (fts);
CREATE INDEX ON documents (project_id);
```

---

## 2. Field-by-field reference

### `projects`

| Column        | Type          | Notes                                                                 |
|---------------|---------------|-----------------------------------------------------------------------|
| `id`          | `BIGSERIAL`   | Surrogate primary key. Referenced by `documents.project_id`.          |
| `slug`        | `TEXT UNIQUE` | Stable, URL/CLI-safe identifier (e.g. `my-novel`). The value stored in `loremaster.json` and passed as `--project`. |
| `name`        | `TEXT`        | Human-readable display name.                                          |
| `description` | `TEXT`        | Optional free-text description shown by `projects describe` / `list_projects`. |
| `created_at`  | `TIMESTAMPTZ` | Set on insert.                                                        |
| `updated_at`  | `TIMESTAMPTZ` | Bumped when project metadata changes.                                 |

### `documents`

| Column        | Type          | Notes                                                                                       |
|---------------|---------------|---------------------------------------------------------------------------------------------|
| `id`          | `BIGSERIAL`   | Surrogate primary key for an individual chunk.                                               |
| `project_id`  | `BIGINT FK`   | Owning project. `ON DELETE CASCADE`. Every query filters on this column.                     |
| `file_path`   | `TEXT`        | Path of the source `.md` file, stored relative to the project root for portability.          |
| `chunk_index` | `INT`         | Zero-based position of this chunk within the file. Together with `file_path` it reconstructs document order. |
| `title`       | `TEXT`        | Document title — from frontmatter `title`, else the first H1, else the filename.             |
| `content`     | `TEXT`        | The chunk text. The only field written that drives both `embedding` and `fts`.              |
| `embedding`   | `VECTOR(768)` | Dense embedding of `content` from the configured model. 768 dims matches `nomic-embed-text`.|
| `fts`         | `TSVECTOR`    | Generated column: `to_tsvector('english', content)`. Computed by Postgres, never written by the app. |
| `metadata`    | `JSONB`       | Structured frontmatter (see §5). Default `'{}'`. Targeted by the `filter` parameter.        |
| `indexed_at`  | `TIMESTAMPTZ` | When this chunk was last (re)indexed. Useful for staleness checks.                          |

**Unique constraint** `(project_id, file_path, chunk_index)` makes ingest
idempotent: re-indexing a file upserts each chunk in place rather than
duplicating it.

---

## 3. Why `project_id` FK instead of a `slug` column on `documents`

A tempting shortcut is to store the project slug directly on every `documents`
row. Loremaster deliberately uses a foreign key to `projects.id` instead, for
three reasons:

1. **Normalization.** The project's `name`, `description`, and slug live in one
   place. Storing the slug on millions of chunk rows would duplicate it and
   risk drift if a project is ever renamed. With the FK, a project rename is a
   single-row update.

2. **Cascade deletes.** `ON DELETE CASCADE` lets Postgres delete every chunk of
   a project the instant the `projects` row is removed. A slug column would
   require an explicit `DELETE FROM documents WHERE slug = ...` in application
   code, which is easy to forget and not transactional with the project delete.

3. **Cheaper filtering and joins.** An integer FK is narrower than a text slug,
   so the `project_id` index and the per-project filter on every search are
   smaller and faster. The `list_projects` tool joins/aggregates against
   `projects` directly.

The slug still exists — it is the *external* identifier humans and config files
use — but internally everything keys on the integer `id`.

---

## 4. Chunking strategy

Embedding models have a bounded context and produce a single vector per input,
so a whole document must be split into chunks before embedding. Loremaster uses
a **sliding window of ~400 tokens with 50-token overlap**, preferring sentence
boundaries.

### Why ~400 tokens

- It comfortably fits within `nomic-embed-text`'s context window with headroom.
- It is large enough to carry a coherent idea (a scene beat, a character note,
  a paragraph or two of worldbuilding) so the resulting vector is semantically
  meaningful, but small enough that a single chunk stays topically focused.
  Oversized chunks average several topics into one vector and retrieve poorly.

### Why 50-token overlap

Overlap prevents information loss at chunk seams. A fact or sentence that would
otherwise be split across a boundary appears in full in at least one chunk, so
a query matching that fact still retrieves a chunk that contains it with
context on both sides. 50 tokens (~12% of the window) is enough to preserve
local context without materially inflating storage or duplicate hits.

### How boundaries are chosen (sentence-aware)

The chunker walks the text accumulating sentences until adding the next
sentence would exceed the ~400-token target. It then closes the chunk **at the
sentence boundary** rather than mid-sentence, and starts the next chunk
backtracked by ~50 tokens (again snapped to a sentence boundary where
possible). Splitting on sentence boundaries keeps each chunk grammatically
whole, which both reads better when returned to Claude and embeds more cleanly.
If a single "sentence" is pathologically long (e.g. a table or code block), the
chunker falls back to a hard token cut.

---

## 5. Frontmatter conventions and the `metadata` JSONB column

Each `.md` file may begin with a YAML frontmatter block. Loremaster recognizes
a fixed set of keys and copies them into the `metadata` JSONB column on every
chunk derived from that file (so metadata filters work regardless of which
chunk matched).

```markdown
---
title: Aria Venn
tags: [protagonist, mage]
characters: [Aria, Master Coll]
location: River District
date: 1247-03-12
---

Aria grew up in the river district where the guild towers...
```

| YAML key     | Type            | Maps to `metadata` field | Purpose                                              |
|--------------|-----------------|--------------------------|------------------------------------------------------|
| `title`      | string          | `title` (and `documents.title`) | Display title; also promoted to the column.   |
| `tags`       | list of strings | `tags`                   | Free-form classification (`protagonist`, `lore`...). |
| `characters` | list of strings | `characters`             | Characters featured in this document.                |
| `location`   | string or list  | `location`               | Setting(s) the document concerns.                    |
| `date`       | string          | `date`                   | In-world or real date associated with the document.  |

The resulting JSONB looks like:

```json
{
  "tags": ["protagonist", "mage"],
  "characters": ["Aria", "Master Coll"],
  "location": "River District",
  "date": "1247-03-12"
}
```

These fields are what the MCP `filter` parameter queries — for example,
restricting a search to chunks where `characters` contains `"Aria"`. See
[mcp-tools.md](./mcp-tools.md) for filter syntax. Unrecognized frontmatter keys
are ignored (not stored), keeping `metadata` predictable for filtering.

---

## 6. The generated `tsvector` column vs. application-side FTS

`fts` is a **generated, stored** column:

```sql
fts TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', content)) STORED
```

Postgres computes and maintains this value automatically whenever `content` is
inserted or updated. Advantages over building a search index in application
code:

- **Always consistent.** The `tsvector` can never drift from `content` — there
  is no second write path to forget. With application-side indexing you must
  remember to re-index on every edit and handle partial failures.
- **Less code, fewer round-trips.** The app writes only `content`; it never
  computes or sends the `tsvector`. Stemming, stop-word removal, and
  normalization are handled by the `'english'` text-search configuration in the
  database.
- **Server-side ranking.** Because the vector lives next to the data and has a
  GIN index, `to_tsquery` matching and `ts_rank` scoring happen entirely in the
  database, returning only the top results over the wire.
- **Transactional.** The index entry commits in the same transaction as the
  row, so a crash can't leave `content` and its search index out of sync.

The `GIN (fts)` index makes `@@`-style matching fast, and is what
`keyword_search` and the keyword branch of `hybrid_search` query.

---

## 7. HNSW index and cosine distance

```sql
CREATE INDEX ON documents USING hnsw (embedding vector_cosine_ops);
```

### Why HNSW

HNSW (Hierarchical Navigable Small World) is an approximate-nearest-neighbor
index. For semantic search it provides sub-linear query time over large vector
sets with high recall, and supports incremental inserts (new chunks can be
added without rebuilding the whole index), which suits an incrementally indexed
corpus. Key tunables:

| Parameter        | Where        | Meaning                                                              |
|------------------|--------------|----------------------------------------------------------------------|
| `m`              | build time   | Max links per node per layer. Higher = better recall, more memory.   |
| `ef_construction`| build time   | Candidate list size while building. Higher = better recall, slower build. |
| `ef_search`      | query time   | Candidate list size while searching (`SET hnsw.ef_search`). Higher = better recall, slower query. |

Defaults (`m=16`, `ef_construction=64`) are reasonable for a single-author
knowledge base; `ef_search` can be raised at query time to trade latency for
recall.

### Why cosine (`vector_cosine_ops`) over L2

Text embeddings from models like `nomic-embed-text` encode *direction* in the
vector space as meaning; magnitude is largely an artifact of input length and
not semantically meaningful. **Cosine distance** compares only the angle
between vectors, so two passages about the same topic score as similar even if
one is longer than the other. **L2 (Euclidean)** distance is sensitive to
magnitude, so it would penalize length differences that have nothing to do with
relevance. The index operator class (`vector_cosine_ops`) must match the
distance operator used in queries (`<=>` for cosine) for the index to be used.

---

## 8. `ON DELETE CASCADE` and project deletion

Because `documents.project_id` declares `REFERENCES projects(id) ON DELETE
CASCADE`, deleting a project is a single statement:

```sql
DELETE FROM projects WHERE slug = $1;
```

Postgres automatically removes every `documents` row whose `project_id` matched
the deleted project — including their embeddings, generated `fts` values, and
index entries — all within the same transaction. This means
`loremaster projects delete <slug>` cannot leave orphaned chunks behind, needs
no manual two-step cleanup, and is atomic: either the project and all its
documents are gone, or nothing changed.
