# Planning and execution

- Ask questions rigourously until you and I both have a proper understanding of the task, scope and direction.
- Break any planned work into nice and small PR-sized reviewable units.
- Break each PR into clear subtasks before implementation.
- Each subtask shouldn't take more than 15-20% of the context <150k tokens
- Tasks must be clearly defined as a todo list to track progress with proper acceptance criteria. We must maintain 2 docs 1. roadmap and 2. current state. Roadmap will have all the bugs,features,etc and current state will contain context on the last task done, so cross thread and cross agents handoffs are super easy
- Simplicity is appreciated, code must be maintainable and readable, and up to proper industry standards.
- Security and performance should not be after thoughts and must be taken into active consideration while in the design phase.
- resources like ram (memory usage), cpu but be taken into active consideration, the programs we design must be efficient and snappy
- Functions and files must be small. Code must be resused where possible to avoid unecessary drift in logic.
- Before making changes ask me if it needs a new branch, although not all work needs branch, if you are updating something that is gitignored or not editing any tracked files then a branch is unncessary. Always ask before creating a branch.
- Use atomic commits after each subtask so each commit represents one logical change and can be reverted cleanly.
- Commit messages should be one succint lines that clearly explains the change and shouldn't mention anything about the agent/llm contributor.
- DO NOT USE COMMENTS OR EMOJIS ANYWHERE

## Non-negotiable rules

- Do not combine unrelated changes in one commit.
- Do not skip verification.
- Do not  directly push to remote
- If asked to commit use succint messages

## OUTPUT STYLE

Be extremely concise, sacrifice grammer for the sake of concision
Explain in very very very simple skip the jargon, the sentences should be easy to read and understandable
Use examples
