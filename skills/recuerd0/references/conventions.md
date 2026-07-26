# Workspace conventions

Every new recuerd0 account starts with **My Workspace**, already shaped around four memories: _MAP, Continuation Brief, _INDEX — Decisions, and D001 — the first decision.

Apply the writing, titling, decision, map, hub, import, and governance conventions whenever working in a Recuerd0 workspace, whether the workspace came from `.recuerd0.yaml`, `--workspace`, an environment variable, or account configuration. Only the automatic session-boot behavior below depends on `.recuerd0.yaml`.

## Boot

At session boot, if the current project contains a `.recuerd0.yaml`:

1. Read `.recuerd0.yaml` and identify its `workspace`.
2. Run `recuerd0 workspace context <workspace> --pretty`.
3. Read `_MAP` first and `Continuation Brief` second.
4. Treat the workspace description, map, brief, and other pinned memories as project context. Follow current decisions and preferences unless the repository proves they are stale.

Do not create a workspace or `.recuerd0.yaml` implicitly. If the project is not configured, continue without automatic boot context unless the user asks to set it up. This gate does not disable the conventions below when a workspace is selected another way.

## Searching

Use `_MAP` as the front door. Follow its route directly or through one hub when the question is phrased in the user's own words. The map supplies the judgment literal keyword search cannot.

Use `recuerd0 search` for exact memory titles, decision numbers, code identifiers, tags, and phrases likely to occur in the text. Keep queries short and distinctive.

Write `_MAP`, hub lines, and titles in asking-vocabulary: the words someone arriving cold would actually use. “Where do the images live?” routes better than “storage architecture.”

## Writing

Treat every title as a retrieval surface. Make it concrete, scoped, and recognizable in search results; avoid vague titles such as “Notes,” “Update,” or “Architecture.”

Reuse a small, stable vocabulary of lowercase tags already established in the workspace. Choose the narrowest category that fits:

- `decision` for a choice and its rationale;
- `discovery` for a finding, constraint, or bug root cause;
- `preference` for a durable user or team preference;
- `general` only when none of the specific categories fit.

Search before creating. If a memory already covers the topic, create a new version instead of a duplicate. Use `recuerd0 memory version create` for substantive title, body, tag, or category changes so history remains visible. Reserve `memory update` for correcting an accidental write, not for evolving knowledge.

Version `Continuation Brief` at the end of each substantive session. Its version history is the session log.

## Decisions

Number locked decisions sequentially as D001, D002, and so on.

Give each decision its own memory titled `D### — <decision>`. Record what was chosen, why, and what was rejected. Add one corresponding line to `_INDEX — Decisions` so every decision remains two hops from `_MAP`.

Do not rewrite a decision when the choice changes. Create the next D-numbered decision, state which earlier decision it supersedes, and point back to it. Preserve the old rationale; if the old memory needs a forward pointer, add that marker as a new version rather than erasing its original state. Keep both entries in the decision index.

## Map maintenance

Maintain structure in two passes:

1. Create or version the content memories first.
2. After every target exists, create a new `_MAP` or hub version that routes to them.

Keep every memory reachable within two hops of `_MAP`: directly from `_MAP`, or from one hub linked by `_MAP`. Never leave a route pointing at a memory that does not exist.

Write one concise line of judgment per route in asking-vocabulary. A line should tell a cold reader why to follow it, not merely repeat the destination's title. Add, change, or remove map lines whenever the underlying workspace structure changes.

## Hubs

Keep `_MAP` flat until roughly twenty memories. Hubs earn their place only when a real cluster makes the map crowded.

Promote a cluster in four steps:

1. Identify a coherent cluster already represented by several `_MAP` lines.
2. Create a `Hub — <topic>` memory with one asking-vocabulary routing line per existing cluster memory.
3. Create a new `_MAP` version that replaces those cluster lines with one line pointing to the hub.
4. Verify that every moved memory is still reachable within two hops and that every route points at a real memory.

Do not seed empty hubs or create filler to justify them.

## After an import

Do not leave an import as an unstructured pile. After `import commit` completes:

1. Group the imported memories into useful clusters and draft asking-vocabulary `_MAP` lines for them.
2. Identify weak titles that will not retrieve well. Fix each one with `recuerd0 memory version create <memory_id> --workspace <workspace> --title "<better title>"`, preserving the imported version.
3. If the workspace is now past roughly twenty memories, propose real cluster promotions behind hubs; do not create hubs automatically.
4. Present the title changes, cluster routes, and hub proposals for review.
5. Apply approved content or hub writes first, then apply the reviewed routing structure as a new `_MAP` version.

## Write governance

Read and search freely. Before filing memories or changing `_MAP`, a hub, a decision, or an index, show the proposed write and get human confirmation. Import execution still follows its stricter propose → review → commit protocol and requires explicit approval before `--yes`.

Never create structural filler, dangling routes, duplicate memories, or silent decision rewrites. Preserve provenance and history through versions. After writing, verify the command response and report what was created or versioned, including memory IDs.

## Session lifecycle and capture discipline

### During work

- Use `recuerd0 search` to recover relevant decisions, discoveries, and preferences as needed.
- For large memories, locate the relevant section with `memory read grep`, then fetch a narrow window with `memory read lines`.
- Treat memories as context, not as a substitute for inspecting the current code and tests.
- If a memory conflicts with the repository, call out the mismatch and verify which source is current.

### Capturing knowledge

Capture knowledge only after it is confirmed and useful beyond the current session:

- decisions and their rationale;
- non-obvious discoveries or bug root causes;
- durable user or team preferences;
- operational details that cost meaningful time to establish.

Do not save routine implementation details, facts obvious from the code, transient progress, secrets, credentials, or unverified guesses.

Before creating a memory, search for the topic in the workspace. Version a strong existing match instead of creating a duplicate. Use `update` only to correct a memory that was wrong rather than superseded.

### End of a session

After substantive work, consider whether the session produced durable knowledge that passes the capture rules above. Save only that distilled knowledge, not a raw transcript, and report what was created, versioned, or updated.
