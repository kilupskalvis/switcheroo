# switcheroo

Hand a live Claude Code conversation to another person. They continue the same conversation in their own Claude Code, with all of your context.

```
you:     /switcheroo
         → Claude writes a short setup note, then prints: Pickup code: 7-crossover-clockwork
them:    /switcheroo 7-crossover-clockwork
         → /resume <id> opens your conversation
```

## Install

Both people need the CLI:

```
curl -fsSL https://raw.githubusercontent.com/kilupskalvis/switcheroo/main/install.sh | sh
```

This installs to `~/.local/bin`; set `SWITCHEROO_INSTALL_DIR` to change it. With Go installed you can use `go install github.com/kilupskalvis/switcheroo/cmd/switcheroo@latest` instead.

To send and receive from inside Claude Code, also add the plugin, which adds `/switcheroo`:

```
/plugin marketplace add kilupskalvis/switcheroo
/plugin install switcheroo@switcheroo
```

### Updates

Claude Code does not auto-update third-party plugins by default. Turn it on once: run `/plugin`, open **Marketplaces**, select `switcheroo`, and choose **Enable auto-update**. The plugin then updates when Claude Code starts.

The CLI does not update itself. Run the install line again to get the latest release.

## Use

**Send.** Type `/switcheroo` in the session. Claude checks for unpushed work and offers to push it. It then sends the conversation with a handoff note describing what the next person needs to set up, and prints a pickup code. You can keep working or close the session; a background process holds the code for an hour.

**Pick up.** In Claude Code, in the folder where you want to work, type `/switcheroo <code>`. It shows the sender's handoff note, then open the conversation with the `/resume <id>` it gives you. From a terminal, `switcheroo pickup <code>` does both and opens Claude; add `--no-resume` to only install the session.

Both commands accept `--relay-url` and `--transit-relay` to use self-hosted magic-wormhole relays.

## Security

- The transcript is sent exactly as it is, including anything it contains. Only send it to people who may see it.
- The transfer is end-to-end encrypted with magic-wormhole (PAKE). The relays only see ciphertext.
- A code works once. One wrong guess also kills it, and unused codes expire after an hour.
- The receiver resumes in their own permission mode, never the sender's.
- A received conversation is untrusted input, because it may contain instructions aimed at your Claude.

## Requirements

Linux or macOS, with Claude Code installed on both machines. Both sides must be online at the same time.
