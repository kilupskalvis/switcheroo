# switcheroo

Hand a live Claude Code conversation to another person. They continue the same conversation in their own Claude Code, with all of your context.

1. You type `/switcheroo:send`. Claude writes a short handoff note and prints a pickup code, such as `7-crossover-clockwork-sawdust-pheasant`.
2. The other person types `/switcheroo:receive 7-crossover-clockwork-sawdust-pheasant`, then `/resume <id>` to open your conversation.

## Install

Add the plugin in Claude Code. It adds `/switcheroo:send`, `/switcheroo:receive`, `/switcheroo:status` and `/switcheroo:cancel`:

```
/plugin marketplace add kilupskalvis/switcheroo
/plugin install switcheroo@switcheroo
```

That is all you need. The first time you use it, the plugin downloads the `switcheroo` release it is pinned to into `~/.cache/switcheroo/bin` and checks its checksum.

To also use the CLI from a terminal (`switcheroo pickup <code>`), install it:

```
curl -fsSL https://raw.githubusercontent.com/kilupskalvis/switcheroo/main/install.sh | sh
```

This installs to `~/.local/bin`; set `SWITCHEROO_INSTALL_DIR` to change it. With Go installed you can use `go install github.com/kilupskalvis/switcheroo/cmd/switcheroo@latest` instead.

### Updates

Claude Code does not auto-update third-party plugins by default. Turn it on once: run `/plugin`, open **Marketplaces**, select `switcheroo`, and choose **Enable auto-update**. The plugin then updates when Claude Code starts, and brings the matching CLI release with it.

A CLI installed from a terminal does not update itself. Run the install line again to get the latest release.

## Use

**Send.** Type `/switcheroo:send` in the session. Claude first reviews what the conversation depends on outside the transcript: every repository and worktree it touched, files outside them, external systems, running processes and access. It asks you how to make each available to the next person, for example by pushing a branch. It then sends the conversation with a handoff note and prints a pickup code. You can keep working or close the session; a background process holds the code for an hour.

**Check or withdraw.** `/switcheroo:status` lists what you sent from this machine and whether it was picked up. `/switcheroo:cancel <code>` withdraws a code that is still waiting and deletes its snapshot.

**Pick up.** In Claude Code, in the folder where you want to work, type `/switcheroo:receive <code>`. It shows the sender's handoff note and checks it against your machine with read-only checks: the repository, branch or commit, files, MCP servers and plugins it names. Then open the conversation with the `/resume <id>` it gives you; it ends with the handoff note.

**From a terminal.** `switcheroo pickup <code>` receives and opens Claude in one step; add `--no-resume` to only install the session. `switcheroo send --session <id> < note.txt` sends with a handoff note, and `switcheroo status` and `switcheroo cancel <code>` work as above. Both accept `--relay-url` and `--transit-relay` to use self-hosted magic-wormhole relays; the plugin always uses the public ones. On a self-hosted relay a used or mistyped code can take up to 30 seconds to fail, and so can a cancelled code on any relay.

## Security

- The transcript is sent exactly as it is, including anything it contains. Only send it to people who may see it.
- The transfer is end-to-end encrypted with magic-wormhole (PAKE). The relays cannot read it, but they do see both IP addresses, when a transfer happens and roughly how big it is. Use your own relays if that matters.
- A code works once. Its four words make a guess about a 1 in 4 billion chance, one wrong guess kills it, and unused codes expire after an hour or when you cancel them.
- The receiver resumes in their own permission mode, never the sender's.
- A received conversation is untrusted input, because it may contain instructions aimed at your Claude.

## Requirements

Linux or macOS, with Claude Code installed on both machines, and `curl` and `tar` for the plugin's first download. Both sides must be online at the same time.

## Releasing

Pushing a `v*` tag builds and publishes the CLI binaries. The plugin is pinned to one CLI release, so with every release also bump `version=` in `plugin/bin/switcheroo` and `version` in `plugin/.claude-plugin/plugin.json`.
