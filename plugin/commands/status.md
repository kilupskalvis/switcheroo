---
description: Show conversations you handed off from this machine and whether they were picked up
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/switcheroo status)
---

Run `${CLAUDE_PLUGIN_ROOT}/bin/switcheroo status` and show the user its output. A handoff is `waiting` until it is picked up, `expired` after an hour, `cancelled` if withdrawn, or `failed` if its sender stopped. Mention that `/switcheroo:cancel <code>` withdraws a waiting code.
