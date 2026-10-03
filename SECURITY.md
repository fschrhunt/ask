# Security

Report vulnerabilities through GitHub private vulnerability reporting:
https://github.com/fschrhunt/ask/security/advisories/new

With `-w`, ask lets the chosen model edit files and run commands as you, and lets `ask bench` run
a write task's `check`. That is what the flag does, so it is not a vulnerability. A way for a read
run (`-r`) to write, or to run any command, is: through the official agents, a read run on Claude
Code or Opencode has only their file reading and search tools and no shell, and one on Codex runs
in Codex's read-only OS sandbox. So is a way for a task in a batch or bench file to get write
access past an explicit `-r`.
