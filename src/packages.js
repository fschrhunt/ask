/*
 * Packages: git repositories shaped like ~/.ask (agents/, hooks/, commands/), so people can share
 * them and stay up to date without forking. `ask install SOURCE` clones one into
 * ~/.ask/packages/HOST/OWNER/REPO; `ask install` alone pulls every package. Nothing in a package
 * runs at install time, and yours in ~/.ask always win over a package's (see find.js).
 */
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readdirSync, rmSync } from 'node:fs';
import { basename, dirname, join, relative, resolve } from 'node:path';
import { packageDirs } from './find.js';
import { PACKAGES, UsageError } from './home.js';

/* Runs git without prompts and returns its output; a failure throws with git's last line. */
function git(args, cwd) {
  try {
    return execFileSync('git', args, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, GIT_TERMINAL_PROMPT: '0' } });
  } catch (error) {
    throw new Error(String(error.stderr || error.message).trim().split('\n').pop());
  }
}

/*
 * Where a source lives: a local repository path, OWNER/REPO for GitHub, or any git URL (https, ssh
 * or scp-style). Returns { url, dir }, dir being ~/.ask/packages/HOST/OWNER/REPO, with HOST "local"
 * for a path.
 */
export function locate(source) {
  if (existsSync(source)) {
    const path = resolve(source);
    return { url: path, dir: join(PACKAGES, 'local', basename(dirname(path)), basename(path).replace(/\.git$/, '')) };
  }
  if (/^[\w.-]+\/[\w.-]+$/.test(source)) source = `https://github.com/${source}`;
  const match = /^(?:[a-z+]+:\/\/(?:[^@/]+@)?|[^@/]+@)?([^/:]+)[/:]([^/]+)\/([^/]+?)(?:\.git)?\/?$/.exec(source);
  if (!match) throw new UsageError(`cannot tell where ${source} lives; give OWNER/REPO or a git URL`);
  return { url: source, dir: join(PACKAGES, match[1], match[2], match[3]) };
}

/* Installs a package, or pulls it if it is already installed. Returns a line saying which. */
export function install(source) {
  const { url, dir } = locate(source);
  if (existsSync(dir)) return update(dir);
  mkdirSync(join(dir, '..'), { recursive: true });
  git(['clone', '--quiet', url, dir]);
  return `installed ${relative(PACKAGES, dir)}: ${contents(dir)}`;
}

/* Pulls one installed package, fast-forward only. Returns a line saying what happened. */
export function update(dir) {
  const before = git(['rev-parse', 'HEAD'], dir).trim();
  git(['pull', '--quiet', '--ff-only'], dir);
  return `${relative(PACKAGES, dir)}: ${git(['rev-parse', 'HEAD'], dir).trim() === before ? 'up to date' : 'updated'}`;
}

/* What a package offers, like "agents: claude, codex · hooks: verify". */
export function contents(dir) {
  const kinds = ['agents', 'hooks', 'commands'].flatMap((kind) => {
    let names = [];
    try {
      names = readdirSync(join(dir, kind)).sort();
    } catch {}
    return names.length ? [`${kind}: ${names.join(', ')}`] : [];
  });
  return kinds.join(' · ') || 'nothing ask uses';
}

/* Every installed package as { name, dir }, name being HOST/OWNER/REPO. */
export const installed = () => packageDirs().map((dir) => ({ name: relative(PACKAGES, dir), dir }));

/* Removes an installed package, named as HOST/OWNER/REPO, OWNER/REPO or REPO. */
export function remove(name) {
  const matches = installed().filter((p) => p.name === name || p.name.endsWith(`/${name}`));
  if (matches.length !== 1)
    throw new UsageError(matches.length ? `${name} matches ${matches.map((p) => p.name).join(' and ')}; name one in full` : `no package ${name}; see \`ask packages\``);
  rmSync(matches[0].dir, { recursive: true, force: true });
  return `removed ${matches[0].name}`;
}
