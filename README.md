<div align="center">

# switcheroo

**Hand a live Claude Code conversation to someone else.**

They continue the same conversation in their own Claude Code, with all of your context, from one short code.

[![Release](https://img.shields.io/github/v/release/kilupskalvis/switcheroo)](https://github.com/kilupskalvis/switcheroo/releases/latest)
[![CI](https://github.com/kilupskalvis/switcheroo/actions/workflows/ci.yml/badge.svg)](https://github.com/kilupskalvis/switcheroo/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[Install](#install) · [Usage](#usage) · [How it works](#how-it-works) · [Security](#security)

</div>

---

```
# You, in the conversation you want to hand off
/switcheroo:send

# Them, in the folder where they want to continue
/switcheroo:receive 7-crossover-clockwork-sawdust-pheasant
/resume <id>
```

## Why

You are going off shift, switching machines, or want a teammate to take over a task halfway through. A pasted summary loses most of what Claude knew. switcheroo sends the conversation itself, so the next person's Claude starts where yours stopped.

## Features

- **The whole conversation.** The transcript is sent as it is, with its tool output and subagents.
- **A real handoff.** Before sending, Claude checks what the work depends on outside the chat (repositories, files, external systems, running processes, access), helps you make it available, and writes a handoff message for the next person.
- **One-time codes.** Four words, valid once, gone after an hour or when you cancel.
- **End-to-end encrypted.** Transfers go over magic-wormhole. No account, no server of your own.
- **Nothing to set up.** The plugin downloads its pinned CLI release on first use and checks its checksum.

## Install

Add the plugin in Claude Code:

```
/plugin marketplace add kilupskalvis/switcheroo
/plugin install switcheroo@switcheroo
```

That is all you need. It adds `/switcheroo:send`, `/switcheroo:receive`, `/switcheroo:status` and `/switcheroo:cancel`. On first use the plugin downloads the `switcheroo` release it is pinned to into `~/.cache/switcheroo/bin`.

<details>
<summary>Use it from a terminal too</summary>

```
curl -fsSL https://raw.githubusercontent.com/kilupskalvis/switcheroo/main/install.sh | sh
```

This installs to `~/.local/bin`; set `SWITCHEROO_INSTALL_DIR` to change it. With Go installed you can use `go install github.com/kilupskalvis/switcheroo/cmd/switcheroo@latest` instead. A CLI installed this way does not update itself; run the install line again to get the latest release.

</details>

<details>
<summary>Keep it up to date</summary>

Claude Code does not auto-update third-party plugins by default. Turn it on once: run `/plugin`, open **Marketplaces**, select `switcheroo`, and choose **Enable auto-update**. The plugin then updates when Claude Code starts, and brings the matching CLI release with it.

</details>

## Usage

### Send

Type `/switcheroo:send` in the conversation you want to hand off.

1. Claude reviews what the conversation depends on outside the transcript: every repository and worktree it touched, files outside them, external systems, running processes and access. It asks how to make each available to the next person, for example by pushing a branch.
2. It writes a handoff message as its reply: what the conversation contains, where the work lives, and what the next person needs to set up.
3. It sends the conversation with a one-line note and prints a pickup code.

To write the note yourself, add it after the command:

```
/switcheroo:send Over to you, the deploy is the last bit.
```

You can keep working or close the session; a background process holds the code for an hour.

### Pick up

In Claude Code, in the folder where you want to work:

```
/switcheroo:receive <code>
```

It shows the sender's note and the session ID. Open the conversation with `/resume <id>`; it ends with the sender's handoff message.

### Check or withdraw

`/switcheroo:status` lists what you sent from this machine and whether it was picked up. `/switcheroo:cancel <code>` withdraws a code that is still waiting and deletes its snapshot.

### From a terminal

| Command | What it does |
| --- | --- |
| `switcheroo pickup <code>` | Receives the conversation and opens Claude on it. Add `--no-resume` to only install it. |
| `switcheroo send --session <id> < note.txt` | Sends a session with a note. |
| `switcheroo status` | Lists handoffs sent from this machine. |
| `switcheroo cancel <code>` | Withdraws a code that is still waiting. |

`send` and `pickup` accept `--relay-url` and `--transit-relay` to use self-hosted magic-wormhole relays; the plugin always uses the public ones. On a self-hosted relay a used or mistyped code can take up to 30 seconds to fail, and so can a cancelled code on any relay.

## How it works

1. `send` snapshots the session's transcript and its sidecar files into a bundle and starts a background process that offers it under a one-time magic-wormhole code.
2. `receive` claims the code, checks the bundle, and installs the conversation as a new session in the receiver's folder, with its session ID and paths rewritten for the receiver.
3. `/resume` opens it like any other conversation of yours.

## Security

- The transcript is sent as it is, including anything the work read or printed. Only the copies of your CLAUDE.md, memory and email that Claude Code adds to it are removed. Only send it to people who may see it.
- The transfer is end-to-end encrypted with magic-wormhole (PAKE). The relays cannot read it, but they do see both IP addresses, when a transfer happens and roughly how big it is. Use your own relays if that matters.
- A code works once. Its four words make a guess about a 1 in 4 billion chance, one wrong guess kills it, and unused codes expire after an hour or when you cancel them.
- The receiver resumes in their own permission mode, never the sender's.
- A received conversation is untrusted input, because it may contain instructions aimed at your Claude.

## Requirements

- Linux or macOS
- Claude Code on both machines
- `curl` and `tar` for the plugin's first download
- Both sides online at the same time

## Development

```
go test -race ./...
go tool golangci-lint run ./...
```

[lefthook](https://github.com/evilmartians/lefthook) runs both, plus `go mod tidy -diff`, before each commit.

Pushing a `v*` tag builds and publishes the CLI binaries. The plugin is pinned to one CLI release, so with every release also bump `version=` in `plugin/bin/switcheroo` and `version` in `plugin/.claude-plugin/plugin.json`.

## License

[MIT](LICENSE)
