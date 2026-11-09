# switcheroo

Hand a live Claude Code conversation to another person. They continue the same conversation in their own Claude Code, with all of your context.

```
you:     /switcheroo
         → Claude writes a short setup note, then prints: Pickup code: 7-crossover-clockwork
them:    switcheroo pickup 7-crossover-clockwork
         → claude opens with your conversation resumed
```

## Install

Both people need the CLI:

```
curl -fsSL https://raw.githubusercontent.com/kilupskalvis/switcheroo/main/install.sh | sh
```

This installs to `~/.local/bin`; set `SWITCHEROO_INSTALL_DIR` to change it. With Go installed you can use `go install github.com/kilupskalvis/switcheroo/cmd/switcheroo@latest` instead.

The sender also needs the Claude Code plugin, which adds `/switcheroo`:

```
/plugin marketplace add kilupskalvis/switcheroo
/plugin install switcheroo@switcheroo
```

## Use

**Send.** Type `/switcheroo` in the session. Claude checks for unpushed work and offers to push it. It then writes a handoff message describing what the next person needs to set up, and prints a pickup code. You can keep working or close the session; a background process holds the code for an hour.

**Pick up.** In the folder where you want to work, run `switcheroo pickup <code>`. Use `--no-resume` to install the session without opening it.

Both commands accept `--relay-url` and `--transit-relay` to use self-hosted magic-wormhole relays.

## Security

- The transcript is sent exactly as it is, including anything it contains. Only send it to people who may see it.
- The transfer is end-to-end encrypted with magic-wormhole (PAKE). The relays only see ciphertext.
- A code works once. One wrong guess also kills it, and unused codes expire after an hour.
- The receiver resumes in their own permission mode, never the sender's.
- A received conversation is untrusted input, because it may contain instructions aimed at your Claude.

## Requirements

Linux or macOS, with Claude Code installed on both machines. Both sides must be online at the same time.
