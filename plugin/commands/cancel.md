---
description: Withdraw a pickup code you sent that has not been picked up yet
argument-hint: "<pickup code>"
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/switcheroo cancel:*), Bash(${CLAUDE_PLUGIN_ROOT}/bin/switcheroo status)
---

Pickup code: `$ARGUMENTS`

1. The pickup code must be a number, a dash, then lowercase words joined by dashes, like `7-crossover-clockwork-sawdust-pheasant`. If it is missing, run `${CLAUDE_PLUGIN_ROOT}/bin/switcheroo status`, show the waiting codes and ask which one to cancel. If it is anything else, say so and stop.

2. Run `${CLAUDE_PLUGIN_ROOT}/bin/switcheroo cancel <code>` and tell the user the result. A cancelled code no longer works and the snapshot of the conversation is deleted from this machine.
