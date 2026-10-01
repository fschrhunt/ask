/*
 * The read-only command rules for harnesses without a read-only shell (Claude Code and Opencode).
 * Their read runs may run only INSPECT commands, written as plain words: a command containing an
 * UNSAFE character (one that redirects, pipes, chains, substitutes, expands, quotes or escapes, any
 * of which could also spell a refused option past a rule) or a WRITER option (one that writes files
 * or runs another program) is refused. Rules match text, so commands that take abbreviated or
 * bundled options, like git grep, are left out; both agents have their own grep tool. Codex needs
 * none of this: its OS sandbox makes the whole run read-only. Tests and builds write files, so they
 * need -w.
 */
export const INSPECT = ['git diff', 'git log', 'git show', 'git status', 'git blame', 'git ls-files', 'rg', 'grep', 'ls', 'wc', 'cat', 'head', 'tail'];
export const UNSAFE = ['>', '|', ';', '&', '`', '$', '\\', "'", '"', '{'];
export const WRITERS = ['--output', '--ext-diff', '--textconv', '--pre', '--hostname-bin'];
