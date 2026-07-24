# Import protocol

Import is propose → review → commit: `propose` writes a reviewable `import.plan.yaml` and never touches the server; `commit` executes the plan you approved.

The importer is designed for large Markdown/Obsidian folders and Recuerd0 workspace-export v1 JSON files. Bodies stay source-backed: the plan controls metadata and write decisions, while commit re-reads the source and refuses to write if it drifted.

## 1. Propose

```bash
recuerd0 import propose ./vault --workspace 12 --pretty
recuerd0 import propose workspace-export.json --workspace 12 --pretty
```

Directories auto-detect as `obsidian_markdown`. Valid export-v1 JSON files auto-detect as `workspace_export`. You can lock the expected shape with `--adapter`.

Propose:

- walks Markdown paths deterministically, skipping hidden directories;
- reads YAML frontmatter, headings, folder tags, supported wiki/Markdown links, duplicates, embeds, and excluded binaries;
- replays and validates every ordered version in a workspace export;
- uses paginated GET requests for case-insensitive title conflicts;
- atomically writes `import.plan.yaml`;
- never POSTs, PATCHes, or DELETEs.

Use `--plan PATH` to choose the plan location and `--ledger PATH` to classify prior imports from a non-default ledger. An existing plan seeds re-propose only when its absolute source path, adapter, and workspace match. Use `--fresh` to discard prior review provenance deliberately.

## 2. Review

The command digest contains:

- `adapter`
- `counts` for create, version, skip, conflicts, unparseable, and excluded
- `titles_from_h1_pct`
- `links_proposed` and `tags_proposed`
- structured `exceptions`
- `thin`, optional `hint`, and `warnings`

If the locked thin heuristic fires, relay this hint unchanged:

> This plan looks thin — refine it by hand or hand it to your agent (see the recuerd0 skill's import protocol).

Review each manifest row and its exceptions:

1. Keep the row's `action` and every exception `resolution` for that path identical.
2. Confirm `target_memory_id` before approving `version`.
3. Never set `target_memory_id` on `create`.
4. Leave a different-revision partial export chain on `skip`.
5. Preserve the source path, hashes, export identity fields, and `versions`.

Scanner-owned fields are title, category, tags, and links. Review-owned fields include action, resolution, target ID, and notes. If a scanner-owned field is edited, future re-proposes preserve the reviewed metadata and its original fingerprint, then add `row edited — rules changes not applied` once. `--fresh` is the explicit way to discard that sticky provenance.

After editing any scanner-owned field (`title`, `category`, `tags`, or `links`), re-run `recuerd0 import propose` with the same source, workspace, plan, and ledger arguments. The reviewed edits are preserved while source and content hashes refresh. Only then preview with `recuerd0 import commit <plan>` and, after human approval, execute with `--yes`.

`rules.tag_map` maps each raw folder/frontmatter contribution to zero or more contributions before normalization. A missing key keeps the original contribution, an empty list removes it, and multiple entries fan out:

```yaml
rules:
  tag_map:
    legacy: []
    Team Notes:
      - area
      - Knowledge Base
```

The example removes `legacy` and turns `Team Notes` into the normalized tags `area` and `knowledge_base`.

Propose assigns conservative defaults:

- conflict, unparseable, title-too-long, and later body-identical duplicates: `skip`;
- guessed titles and case-insensitive duplicate titles: `create`;
- completed unchanged Markdown: `skip`;
- completed changed Markdown: `version`;
- completed changed exports: conflict and `skip`.

When identical Markdown bodies occur, the lexicographically earliest path remains eligible to create. Only later paths receive `dupe_exact`.

## 3. Preview or dry-run

```bash
recuerd0 import commit import.plan.yaml --pretty
recuerd0 import commit import.plan.yaml --yes --dry-run --pretty
```

Both commands validate the plan, ledger, agreement rules, export fidelity, and source hashes; both emit the same digest shape as propose; both perform no writes and exit 1. `--dry-run` wins over `--yes`.

If source content changed, commit stops before pass 1 with:

> source changed since propose — re-run propose

## 4. Commit

```bash
recuerd0 import commit import.plan.yaml --yes --pretty
```

The default ledger is `import.ledger.jsonl` beside the plan. It is append-only JSONL. Every write has an fsynced `intent` before the POST and an fsynced `committed` record only after a fresh GET matches title, body, tag order, category, and expected server version.

Keep the ledger with the plan. It fixes:

- the root memory ID for every later ordinal;
- the chain base (0 for fresh imports, or the target's preflight version for appends);
- gap-free committed ordinals;
- dangling intents whose response may have been lost.

On resume, commit uses the persisted base and expected version. It never replaces them with a newly observed server version. Concurrent advancement, ambiguous reconciliation, a 4xx pass-1 response, or read-back mismatch aborts with exit 2 and a truthful partial summary.

After all memory/version rows, links run in a second pass. Link failures are non-fatal but appear in `links_failed` and keep `plan.complete` false. Missing ledger identities appear in `links_skipped_unresolvable`, but do not by themselves keep an otherwise finished plan from reporting `plan.complete: true`.

A clean commit exits 0 and reports:

- invocation `ops`: created, versioned, and reconciled;
- `rows`: completed now, review-skipped, and already committed;
- plan totals, remaining rows, and `plan.complete`;
- links created/existing, skipped, and failed;
- the exact `ledger_path`.

## Agent boundary

The agent's job ends at the plan. Never import by writing memories one-by-one through MCP; always execute through `recuerdo import commit`, and pass `--yes` only after the human has seen the digest and said go.
