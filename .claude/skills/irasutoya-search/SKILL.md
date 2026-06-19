---
name: irasutoya-search
description: Search Irasutoya illustrations through this project's irasutoya CLI. Use when Claude needs to find Irasutoya/irasutoya.com illustrations, run irasutoya search, download or reuse the local CLI binary, summarize Irasutoya search results, or explicitly open/preview returned images.
argument-hint: "<query>"
---

# Irasutoya Search

Use the bundled script to perform real searches instead of reimplementing the scraper or guessing URLs.

## Quick Start

Run the search script from this skill directory:

```bash
python "${CLAUDE_SKILL_DIR}/scripts/irasutoya_search.py" $ARGUMENTS
```

If the user gives a query in natural language rather than slash-command arguments, pass that query to the same script:

```bash
python "${CLAUDE_SKILL_DIR}/scripts/irasutoya_search.py" cat
python "${CLAUDE_SKILL_DIR}/scripts/irasutoya_search.py" "office worker"
```

Open images only when the user explicitly asks to open or preview them:

```bash
python "${CLAUDE_SKILL_DIR}/scripts/irasutoya_search.py" --open-images dog
```

The summary shows up to five image URLs per result by default. Use `--all-images` only when the user asks for every image URL.

## Behavior

- Resolve `irasutoya` on PATH first.
- If PATH resolution fails, reuse a cached downloaded binary from `scripts/bin/<tag>/<os>_<arch>/`.
- If no cache exists, query the latest GitHub release for `Mineru98/irasutoya-cli`, download the matching OS/arch archive and `checksums.txt`, verify the archive SHA-256, extract only inside `scripts/bin/<tag>/<os>_<arch>/`, then run it.
- Keep default output text-only. Pass `--open-images` to the CLI only for explicit image-open requests.
- If no published release exists, report the blocker. Do not claim live release fallback succeeded.

## Validation

Use deterministic checks before trusting edits to the script:

```bash
python "${CLAUDE_SKILL_DIR}/scripts/test_irasutoya_search.py"
```
