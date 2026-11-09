---
description: Hand this conversation to another person with a one-time code
allowed-tools: Bash(switcheroo send:*), Bash(git status:*), Bash(git log:*)
---

The user wants to hand this conversation to another person, who will continue it in their own Claude Code on their own machine. They receive this transcript exactly as it is, but nothing else from this machine.

1. If the working directory is a git repository, run `git status --short --branch` and `git log --oneline @{u}..` to find uncommitted changes and unpushed commits. If there are any, tell the user what you found and ask whether to push them before handing off. Stop and wait for the answer; do not continue in this turn.

2. Write your handoff message as your reply. Address the next person directly and cover what they need to continue on their machine:
   - the task: goal, what is done, what is in progress, and the next step
   - repository, branch and commit, and any work that is not pushed
   - tools, MCP servers, skills and plugins this conversation used
   - files, services, credentials or environment outside the repository that the work depends on
   - anything else that will be different on their machine

   Keep it short and concrete. Do not paste secrets into it.

3. In the same turn, right after the message, run:

   ```
   switcheroo send --session ${CLAUDE_SESSION_ID}
   ```

   Then tell the user the pickup code and the command the other person runs. The code works once and expires after an hour. If `switcheroo` is not installed, say so and point to `curl -fsSL https://raw.githubusercontent.com/kilupskalvis/switcheroo/main/install.sh | sh`.
