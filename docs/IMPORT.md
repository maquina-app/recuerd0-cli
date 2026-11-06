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

After editing any scanner-owned field (`title`, `category`, `tags`, or `links`), re-run `recuerd0 import propose` with the same source, workspace, plan, and ledger arguments. The reviewed edits are preserved while source and content hashes refresh. Only then show the refreshed digest to the human and execute the reviewed plan after approval.

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

- conflict, unparseable, title-too-long, unimportable, and later body-identical duplicates: `skip`;
- guessed titles, case-insensitive duplicate titles, and exports with repeated or blank versions: `create`;
- completed unchanged Markdown: `skip`;
- completed changed Markdown: `version`;
- completed changed exports: conflict and `skip`.

When identical Markdown bodies occur, the lexicographically earliest path remains eligible to create. Only later paths receive `dupe_exact`.

Commit stops at the first row the server refuses, so propose flags what it knows the server won't take:

- `unimportable`: more than 50 tags, a tag over 100 characters, content over 1 MB, a null character, content that isn't valid UTF-8, a Markdown file emptied since it was imported, or an export memory with nothing in any version. The row is skipped and the rest of the import goes ahead. An export memory is skipped whole if any version is affected. For Markdown, trimming the row's `tags` clears a tag limit on the next propose; set `action` back to `create` to import it.
- `repeated_versions`: an export version identical to the one before it. The server refuses these, and they hold nothing new, so they aren't imported.
- `blank_versions`: an export version with no content, following one that has content. The server refuses these too. If the source's current version is blank, the imported memory ends on the last version with content.

Dropped versions are why an imported memory can have fewer versions than its source, with different version numbers. The exception says how many land, so review it if the numbering matters. Setting the row to `skip` is the only alternative.

## 3. Commit

```bash
# Interactive: validates, shows the target workspace, and asks once.
recuerd0 import commit import.plan.yaml --pretty

# Non-interactive: use only after the human approves the digest.
recuerd0 import commit import.plan.yaml --yes --pretty
```

Commit repeats the plan, ledger, agreement, export-fidelity, and source-hash validation immediately before writing. In a terminal it prompts once with the create/version counts and workspace id and name. Declining exits zero without writing. Without a TTY, `--yes` is required.

If the reviewed plan contains no create or version actions, commit reports that it contains no writes and exits zero without authentication or network access.

If source content changed, commit stops before pass 1 with:

> source changed since propose — re-run propose

## 4. Execution and recovery

The default ledger is `import.ledger.jsonl` beside the plan. It is append-only JSONL. Every write has an fsynced `intent` before the POST and an fsynced `committed` record only after a fresh GET matches title, body, tag order, category, and expected server version. The match allows for what the server normalizes on save: a blank line added between a list and a code fence that follows it, and repeated tags dropped.

Keep the ledger with the plan. It fixes:

- the root memory ID for every later ordinal;
- the chain base (0 for fresh imports, or the target's preflight version for appends);
- gap-free committed ordinals;
- dangling intents whose response may have been lost.

On resume, commit uses the persisted base and expected version. It never replaces them with a newly observed server version. Concurrent advancement, ambiguous reconciliation, a 4xx pass-1 response, or read-back mismatch aborts with a non-zero exit (6 when the server refused a row) and a truthful partial summary.

An export that an older CLI stopped partway on a repeated or blank version can't resume, because its chain now has a different length. Propose keeps that row on `skip` with a conflict exception, and the memory stays at the versions that landed. To import it again, delete that memory, then propose with a new ledger path.

After all memory/version rows, links run in a second pass. Link failures are non-fatal but appear in `links_failed` and keep `plan.complete` false. Missing ledger identities appear in `links_skipped_unresolvable`, but do not by themselves keep an otherwise finished plan from reporting `plan.complete: true`.

A clean commit exits 0 and reports:

- invocation `ops`: created, versioned, and reconciled;
- `rows`: completed now, review-skipped, and already committed;
- plan totals, remaining rows, and `plan.complete`;
- links created/existing, skipped, and failed;
- the exact `ledger_path`.

## Agent boundary

The agent's job ends at the plan. Never import by writing memories one-by-one through MCP; always execute through `recuerd0 import commit`, and pass `--yes` only after the human has seen the digest and said go.
