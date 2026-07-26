# Workspace conventions

New accounts start with four seeded memories: _MAP, Continuation Brief, _INDEX — Decisions, and a first decision, D001.

Use these conventions whenever the current project has a `.recuerd0.yaml`.

## Start of a session

1. Read `.recuerd0.yaml` and identify its `workspace`.
2. Run `recuerd0 workspace context <workspace> --pretty`.
3. Treat the returned workspace description and pinned memories as project context. Follow any current decisions or preferences they contain.
4. If the task is non-trivial, search Recuerd0 for the task's key terms before changing code.

Do not create a workspace or `.recuerd0.yaml` implicitly. If the project is not configured, continue without workspace context unless the user asks to set it up.

## During work

- Use `recuerd0 search` to recover relevant decisions, discoveries, and preferences as needed.
- For large memories, locate the relevant section with `memory read grep`, then fetch a narrow window with `memory read lines`.
- Treat memories as context, not as a substitute for inspecting the current code and tests.
- If a memory conflicts with the repository, call out the mismatch and verify which source is current.

## Capturing knowledge

Capture knowledge only after it is confirmed and useful beyond the current session:

- decisions and their rationale;
- non-obvious discoveries or bug root causes;
- durable user or team preferences;
- operational details that cost meaningful time to establish.

Do not save routine implementation details, facts obvious from the code, transient progress, secrets, credentials, or unverified guesses.

Before creating a memory, search for the topic in the workspace. Version a strong existing match instead of creating a duplicate. Use `update` only to correct a memory that was wrong rather than superseded.

## End of a session

After substantive work, consider whether the session produced durable knowledge that passes the capture rules above. Save only that distilled knowledge, not a raw transcript, and report what was created, versioned, or updated.
