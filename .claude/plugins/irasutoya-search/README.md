# Irasutoya Search Claude Plugin

This plugin packages the `irasutoya-search` Claude Code skill for reusable, namespaced installation.

## Local Test

From the repository root:

```sh
claude --plugin-dir .claude/plugins/irasutoya-search
```

Invoke the plugin skill:

```text
/irasutoya-search:irasutoya-search cat
```

The skill runs `scripts/irasutoya_search.py`, resolves an `irasutoya` binary from PATH/cache/latest release, and keeps image opening disabled unless explicitly requested.
