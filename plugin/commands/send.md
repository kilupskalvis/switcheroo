---
description: Hand this conversation to another person with a one-time pickup code
allowed-tools: Bash(switcheroo send:*), Bash(git status:*), Bash(git log:*)
---

The other person will continue this conversation in their own Claude Code on their own machine. They receive this transcript exactly as it is, but nothing else from this machine.

If `switcheroo` is not installed, say so and point to `curl -fsSL https://raw.githubusercontent.com/kilupskalvis/switcheroo/main/install.sh | sh`.

1. If the working directory is a git repository, run `git status --short --branch` and `git log --oneline @{u}..` to find uncommitted changes and unpushed commits. If there are any, tell the user what you found and ask whether to push them before handing off. Stop and wait for the answer; do not continue in this turn.

2. Send the conversation with a handoff note for the next person, passed on stdin:

   ```
   switcheroo send --session ${CLAUDE_SESSION_ID} <<'EOF'
   <handoff note>
   EOF
   ```

   The note is required; the command fails without it. Address the next person directly and cover what they need to continue on their machine:
   - the task: goal, what is done, what is in progress, and the next step
   - repository, branch and commit, and any work that is not pushed
   - tools, MCP servers, skills and plugins this conversation used
   - files, services, credentials or environment outside the repository that the work depends on
   - anything else that will be different on their machine

   Keep it short and concrete. Do not put secrets in it.

3. Tell the user the pickup code. The other person receives it by typing `/switcheroo:receive <code>` in Claude Code in their copy of the repository, or by running `switcheroo pickup <code>` in a terminal. The code works once and expires after an hour.
