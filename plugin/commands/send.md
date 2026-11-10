---
description: Hand this conversation to another person with a one-time pickup code
argument-hint: "[note for the receiver]"
allowed-tools: Bash(${CLAUDE_PLUGIN_ROOT}/bin/switcheroo send:*), Read, Grep, Glob, Bash(ls:*), Bash(ps:*), Bash(git status:*), Bash(git log:*), Bash(git branch:*), Bash(git remote:*), Bash(git worktree list:*), Bash(git rev-parse:*)
---

The other person will continue this conversation in their own Claude Code on their own machine. They receive this transcript exactly as it is, but nothing else from this machine: no files, no repositories, no running processes, no access.

1. Find everything this conversation depends on that lives outside the transcript. Go through the whole conversation, not just the last task. If it is long or was compacted, search its transcript for the paths you touched: it is `${CLAUDE_SESSION_ID}.jsonl` under the `projects` folder of the Claude config directory (`$CLAUDE_CONFIG_DIR` or `~/.claude`). Look for:
   - every git repository and worktree you read or changed, not only the current directory; in each, check for uncommitted changes and unpushed commits
   - files outside any repository: drafts, scratch files, downloads, generated output
   - external systems: tickets, documents, dashboards, deployments, databases, chats
   - running processes, servers, background jobs and their state
   - environment variables, credentials, access rights and tools the work needs

   This may not be a coding task at all; cover whatever the work actually involved. Tell the user what you found and ask how to make each piece available to the next person, for example pushing a branch, sharing a file or granting access. Stop and wait for the answer; do not continue in this turn. If nothing needs action, say so briefly and continue.

2. Write the handoff message as your reply. It is the last thing the next person reads when they open this conversation. Address them directly and cover what they need to continue on their machine:
   - what this conversation contains: the task, its goal, what is done, what is in progress, and the next step
   - where the work lives: repositories with branch and commit, files, documents and other artifacts, and anything that stayed on this machine
   - external state: systems, tickets, deployments, processes and anything else that matters
   - what they need to set up: tools, MCP servers, skills, plugins, environment, credentials and access
   - anything else that will be different on their machine

   Keep it concrete. Do not put secrets in it.

3. Directly after the message, with no other tool call in between, send the conversation with a short note on stdin: one or two sentences saying what this is, shown to the receiver before they open it:

   ```
   ${CLAUDE_PLUGIN_ROOT}/bin/switcheroo send --session ${CLAUDE_SESSION_ID} <<'EOF'
   <short note>
   EOF
   ```

   If the user wrote a note after the command, use it word for word as the short note: `$ARGUMENTS`. If that is empty, write the note yourself.

   The command fails without the note, or when your handoff message does not come right before it.

4. Tell the user the pickup code. The other person receives it by typing `/switcheroo:receive <code>` in Claude Code, in the folder where they want to continue, or by running `switcheroo pickup <code>` in a terminal. The code works once and expires after an hour; `/switcheroo:status` shows whether it was picked up and `/switcheroo:cancel <code>` withdraws it.
