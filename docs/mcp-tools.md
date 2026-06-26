# Loremaster MCP Tools

> **Audience: the LLM using these tools.** This document tells you what each
> Loremaster tool does, when to choose it, what every parameter means, and what
> the result looks like. Read the "Which search tool to use when" section
> before your first query.

Loremaster exposes a story writer's markdown knowledge base over MCP. The
corpus is split into **chunks** (passages of ~400 tokens) stored per
**project**. You query it to ground your answers in the writer's actual notes —
characters, locations, lore, plot — instead of guessing.

Every tool is scoped to a single **project** (identified by its `slug`). If a
default project is configured the `project` parameter is optional; when in
doubt, call `list_projects` first.

---

## Shared concepts

- **Chunk**: a passage from one `.md` file. Identified by `file_path` +
  `chunk_index`. Search returns chunks, not whole files.
- **Project**: an isolated namespace of documents (e.g. one novel). Queries
  never cross projects.
- **Score**: a relevance number. Higher is more relevant. Do not compare scores
  across different tools — they are produced by different scoring methods.
- **`filter`**: an optional metadata constraint available on all three search
  tools. See [The `filter` parameter](#the-filter-parameter).

---

## Which search tool to use when

| You want to…                                                                 | Use               |
|------------------------------------------------------------------------------|-------------------|
| Find passages about a concept/theme, even with different wording             | `semantic_search` |
| Find an exact name, term, spelling, or quote                                 | `keyword_search`  |
| Get the best of both — a natural-language question that may also contain names | `hybrid_search` (**default choice**) |
| Read the full text of a file you already identified                          | `get_document`    |
| Browse what files exist in a project                                         | `list_documents`  |
| Discover which projects exist                                                | `list_projects`   |

**Rule of thumb:** if you're not sure, use `hybrid_search`. It rarely does
worse than either single-mode search and usually does better. Reach for
`keyword_search` only when you need an exact-token match (a proper noun, an
invented term), and `semantic_search` when wording is likely to differ from the
source (conceptual or thematic questions).

After a search returns a promising chunk, use `get_document` to read its full
file if you need surrounding context the chunk omitted.

---

## `semantic_search`

**Description.** Embeds your query and finds the chunks whose meaning is
closest, using vector (cosine) similarity over an HNSW index. Matches by concept
rather than by literal words — "how magic is taught" can match a passage that
says "apprentices train at the guild towers."

**When to prefer it.** Conceptual, thematic, or paraphrased questions where the
source text may use different vocabulary than your query. Avoid it for exact
name lookups, where embeddings can blur near-synonyms.

**Parameters.**

| Name      | Type    | Required | Description                                                        |
|-----------|---------|----------|--------------------------------------------------------------------|
| `query`   | string  | yes      | Natural-language query. Embedded, then matched by similarity.      |
| `project` | string  | no*      | Project slug to search. *Required if no default project is set.    |
| `limit`   | integer | no       | Max chunks to return. Default `10`.                                |
| `filter`  | object  | no       | Metadata constraint. See [filter](#the-filter-parameter).          |

**Example input.**

```json
{
  "query": "How is magic taught to young people?",
  "project": "my-novel",
  "limit": 5
}
```

**Example output shape.**

```json
{
  "results": [
    {
      "file_path": "lore/magic-system.md",
      "chunk_index": 2,
      "title": "The Guild System",
      "content": "Apprentices are taken in at the river district towers where...",
      "score": 0.83,
      "metadata": { "tags": ["lore", "magic"], "location": "River District" }
    }
  ]
}
```

---

## `keyword_search`

**Description.** Full-text search over the `english` `tsvector` of each chunk,
ranked with `ts_rank`. Matches stems of literal words (so "running" matches
"run"), but does **not** match by meaning.

**When to prefer it.** Exact tokens: a character or place name, an invented
term, a specific quote, an item or spell name. Use it when getting the literal
spelling right matters and a near-synonym would be wrong.

**Parameters.**

| Name      | Type    | Required | Description                                                          |
|-----------|---------|----------|----------------------------------------------------------------------|
| `query`   | string  | yes      | Search terms. Tokenized and stemmed; supports multiple words.        |
| `project` | string  | no*      | Project slug. *Required if no default project is set.                |
| `limit`   | integer | no       | Max chunks to return. Default `10`.                                  |
| `filter`  | object  | no       | Metadata constraint. See [filter](#the-filter-parameter).            |

**Example input.**

```json
{ "query": "Master Coll guild towers", "project": "my-novel", "limit": 5 }
```

**Example output shape.** Same shape as `semantic_search`. `score` here is a
`ts_rank` value (higher = better) and is not comparable to a semantic score.

```json
{
  "results": [
    {
      "file_path": "characters/coll.md",
      "chunk_index": 0,
      "title": "Master Coll",
      "content": "Master Coll oversees the guild towers and...",
      "score": 0.19,
      "metadata": { "characters": ["Master Coll"] }
    }
  ]
}
```

---

## `hybrid_search`

**Description.** Runs `semantic_search` and `keyword_search` in parallel and
merges their rankings with **Reciprocal Rank Fusion (RRF)**. A chunk that ranks
highly in either list — or moderately in both — rises to the top. This is the
most robust general-purpose retrieval tool.

**When to prefer it.** Almost always, and especially for natural-language
questions that also contain specific names ("What did Master Coll teach Aria
about the river towers?"). RRF is rank-based, so it sidesteps the
incomparable-score problem of trying to blend raw similarity and `ts_rank`
values directly.

**Parameters.**

| Name      | Type    | Required | Description                                                                    |
|-----------|---------|----------|--------------------------------------------------------------------------------|
| `query`   | string  | yes      | Natural-language query; used for both the semantic and keyword branches.       |
| `project` | string  | no*      | Project slug. *Required if no default project is set.                          |
| `limit`   | integer | no       | Max merged chunks to return. Default `10`.                                     |
| `filter`  | object  | no       | Metadata constraint applied to both branches. See [filter](#the-filter-parameter). |

**Example input.**

```json
{
  "query": "What did Master Coll teach Aria about the river towers?",
  "project": "my-novel",
  "limit": 8
}
```

**Example output shape.** Same chunk shape; `score` is the fused RRF score
(higher = better).

```json
{
  "results": [
    {
      "file_path": "characters/aria.md",
      "chunk_index": 1,
      "title": "Aria Venn",
      "content": "Under Master Coll, Aria learned to channel the tower wards...",
      "score": 0.0312,
      "metadata": { "characters": ["Aria", "Master Coll"], "location": "River District" }
    }
  ]
}
```

---

## `get_document`

**Description.** Returns the full content of one source file by reassembling all
of its chunks in order (by `chunk_index`). Use this after a search points you at
a file and you need the whole thing, not just the matched passage.

**When to prefer it.** When you have a `file_path` (from a search result or
`list_documents`) and need complete context — the full character sheet, the
entire scene — rather than an isolated chunk.

**Parameters.**

| Name        | Type   | Required | Description                                                  |
|-------------|--------|----------|--------------------------------------------------------------|
| `file_path` | string | yes      | Path of the file to retrieve, exactly as returned by search. |
| `project`   | string | no*      | Project slug. *Required if no default project is set.        |

**Example input.**

```json
{ "file_path": "characters/aria.md", "project": "my-novel" }
```

**Example output shape.**

```json
{
  "file_path": "characters/aria.md",
  "title": "Aria Venn",
  "metadata": { "characters": ["Aria", "Master Coll"], "location": "River District" },
  "chunk_count": 4,
  "content": "Aria grew up in the river district...\n\n...full reassembled text..."
}
```

---

## `list_documents`

**Description.** Lists the files indexed in a project (one entry per file, not
per chunk), with light metadata. Use it to orient yourself before searching, or
to confirm a file exists.

**When to prefer it.** Exploration: "what's in this project?", "is there a file
about the magic system?", or to obtain a `file_path` to pass to `get_document`.

**Parameters.**

| Name      | Type    | Required | Description                                                       |
|-----------|---------|----------|-------------------------------------------------------------------|
| `project` | string  | no*      | Project slug. *Required if no default project is set.             |
| `filter`  | object  | no       | Restrict the listing by metadata. See [filter](#the-filter-parameter). |
| `limit`   | integer | no       | Max files to list. Default `100`.                                 |

**Example input.**

```json
{ "project": "my-novel", "filter": { "tags": "lore" } }
```

**Example output shape.**

```json
{
  "documents": [
    {
      "file_path": "lore/magic-system.md",
      "title": "The Guild System",
      "chunk_count": 3,
      "metadata": { "tags": ["lore", "magic"], "location": "River District" },
      "indexed_at": "2026-06-20T14:02:00Z"
    }
  ]
}
```

---

## `list_projects`

**Description.** Lists every project in the database. Use it first when you
don't know which project to query, or to present available knowledge bases to
the user.

**When to prefer it.** At the start of a session, or whenever a `project`
parameter is required and you don't yet know a valid slug.

**Parameters.** None.

**Example input.**

```json
{}
```

**Example output shape.**

```json
{
  "projects": [
    {
      "slug": "my-novel",
      "name": "My Novel",
      "description": "Epic fantasy set in the river cities.",
      "document_count": 42
    }
  ]
}
```

---

## The `filter` parameter

`filter` is an optional object available on `semantic_search`,
`keyword_search`, `hybrid_search`, and `list_documents`. It constrains results
to chunks whose `metadata` JSONB satisfies the given conditions, applied **in
addition to** (not instead of) the search. Use it to scope a query to a
character, location, tag, or date drawn from frontmatter.

Recognized metadata fields (populated from frontmatter — see
[data-model.md](./data-model.md)):

| Field        | Shape in metadata | Typical filter use                       |
|--------------|-------------------|------------------------------------------|
| `characters` | array of strings  | Only chunks featuring a character        |
| `location`   | string or array   | Only chunks set in a place               |
| `tags`       | array of strings  | Only chunks with a classification tag    |
| `date`       | string            | Only chunks for a given in-world date    |
| `title`      | string            | Match by document title                  |

**Semantics.**

- A scalar value means "this field contains / equals this value." For array
  fields (`characters`, `tags`) it means the array **contains** the value; for
  scalar fields it means equality.
- Multiple keys are combined with **AND** — all conditions must hold.
- An array value for a key means "contains any of these" (OR within that key).

**Examples.**

Only passages featuring Aria, about magic:

```json
{
  "query": "how she learned to cast",
  "project": "my-novel",
  "filter": { "characters": "Aria" }
}
```

Passages set in the River District, tagged as lore:

```json
{
  "query": "history of the towers",
  "project": "my-novel",
  "filter": { "location": "River District", "tags": "lore" }
}
```

Passages featuring either of two characters:

```json
{ "query": "their rivalry", "filter": { "characters": ["Aria", "Coll"] } }
```

If a filter excludes everything, the search returns an empty `results` array —
that means no indexed chunk matched both the query and the metadata constraint,
not that the tool failed. Loosen the filter or drop it and rely on the query.
