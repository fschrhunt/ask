/*
 * What a write run changed, and worktrees for --worktree runs. Both work on any git repository and
 * know nothing about the agent: a snapshot before the run and one after are compared file by file,
 * so files that were already modified before the run are only reported if the run changed them.
 */
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync } from 'node:fs';
import { join, relative } from 'node:path';
import { WORKTREES } from './home.js';

/* Runs git in `dir` and returns its output, or null when git fails (not a repository, no HEAD). */
function git(dir, args, input) {
  try {
    return execFileSync('git', ['-C', dir, ...args], { input, encoding: 'utf8', stdio: ['pipe', 'pipe', 'ignore'], maxBuffer: 256 * 1024 * 1024 });
  } catch {
    return null;
  }
}

/* Paths git reports as changed or untracked, relative to the repository root. */
function dirtyPaths(root) {
  const fields = (git(root, ['status', '--porcelain=v1', '-z', '--untracked-files=all']) || '').split('\0');
  const paths = [];
  for (let i = 0; i < fields.length; i++) {
    if (!fields[i]) continue;
    paths.push(fields[i].slice(3));
    if (/^[RC]/.test(fields[i])) paths.push(fields[++i]);
  }
  return paths;
}

/* The content hash of each path's working file, or null for a path with no file. */
function hashFiles(root, paths) {
  const present = paths.filter((path) => existsSync(join(root, path)));
  const hashes = present.length ? (git(root, ['hash-object', '--stdin-paths'], present.join('\n') + '\n') || '').trim().split('\n') : [];
  const map = new Map(paths.map((path) => [path, null]));
  present.forEach((path, i) => map.set(path, hashes[i] || null));
  return map;
}

/* The blob hash of each path at a commit, or null where the commit has no such file. */
function hashAt(root, commit, paths) {
  const map = new Map(paths.map((path) => [path, null]));
  if (!commit || !paths.length) return map;
  for (const entry of (git(root, ['ls-tree', '-z', commit, '--', ...paths]) || '').split('\0')) {
    const match = /^\S+ blob (\S+)\t(.*)$/s.exec(entry);
    if (match) map.set(match[2], match[1]);
  }
  return map;
}

/* The repository state a later `changes` call compares against, or null when `dir` is not in a repository. */
export function snapshot(dir) {
  const root = git(dir, ['rev-parse', '--show-toplevel'])?.trim();
  if (!root) return null;
  const head = git(root, ['rev-parse', '--verify', '-q', 'HEAD'])?.trim() || null;
  return { root, head, files: hashFiles(root, dirtyPaths(root)) };
}

/*
 * What changed since `before`: { files: [{ path, change }], commits }, where change is added,
 * modified or deleted and commits counts commits made on top of the starting HEAD.
 */
export function changes(before) {
  const { root } = before;
  const head = git(root, ['rev-parse', '--verify', '-q', 'HEAD'])?.trim() || null;
  const committed = head && before.head && head !== before.head ? (git(root, ['diff', '--name-only', '-z', before.head, head]) || '').split('\0') : [];
  const paths = [...new Set([...before.files.keys(), ...dirtyPaths(root), ...committed.filter(Boolean)])];
  const clean = paths.filter((path) => !before.files.has(path));
  const start = new Map([...hashAt(root, before.head, clean), ...before.files]);
  const end = hashFiles(root, paths);
  const files = paths
    .filter((path) => start.get(path) !== end.get(path))
    .sort()
    .map((path) => ({ path, change: !start.get(path) ? 'added' : !end.get(path) ? 'deleted' : 'modified' }));
  const commits = head && before.head && head !== before.head ? Number(git(root, ['rev-list', '--count', `${before.head}..${head}`]) || 0) : 0;
  return { files, commits };
}

/*
 * A worktree for a --worktree run, made from the HEAD of the repository holding `dir`, on a new
 * branch ask/NAME at WORKTREES/NAME. An existing one at that path is reused, so a follow-up runs where
 * the first run left off. Returns { path, branch, dir } where dir is `dir`'s counterpart inside it,
 * and dirty when the source had uncommitted changes the worktree does not include.
 */
export function addWorktree(dir, name) {
  const root = git(dir, ['rev-parse', '--show-toplevel'])?.trim();
  if (!root) throw new Error(`--worktree needs a git repository; ${dir} is not in one`);
  const path = join(WORKTREES, name);
  const branch = `ask/${name}`;
  if (!existsSync(path)) {
    mkdirSync(WORKTREES, { recursive: true });
    const exists = git(root, ['rev-parse', '--verify', '-q', `refs/heads/${branch}`]) !== null;
    if (git(root, ['worktree', 'add', ...(exists ? [path, branch] : ['-b', branch, path, 'HEAD'])]) === null)
      throw new Error(`could not create a worktree for ${root}; does it have a commit?`);
  }
  return { path, branch, dir: join(path, relative(root, dir)), dirty: dirtyPaths(root).length > 0 };
}

/* Removes a worktree and its branch; for a run that changed nothing. */
export function removeWorktree({ path, branch }) {
  const root = git(path, ['rev-parse', '--git-common-dir'])?.trim();
  git(path, ['worktree', 'remove', '--force', path]);
  if (root) git(join(root, '..'), ['branch', '-D', branch]);
}
