/*
 * Finding the executables that make ask yours: agents/, hooks/ and commands/. Each kind is looked up
 * in ~/.ask first, then in every installed package (~/.ask/packages/HOST/OWNER/REPO), in path order.
 * The first executable with a name wins, so yours always win over a package's.
 */
import { accessSync, constants, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { HOME, PACKAGES } from './home.js';

// A name ask accepts for an agent, hook or command.
export const NAME = /^[a-z0-9][a-z0-9_-]*$/;

const executable = (path) => {
  try {
    accessSync(path, constants.X_OK);
    return statSync(path).isFile();
  } catch {
    return false;
  }
};
const list = (dir) => {
  try {
    return readdirSync(dir).sort();
  } catch {
    return [];
  }
};

/* The folders of every installed package, in path order. */
export function packageDirs() {
  return list(PACKAGES).flatMap((host) => list(join(PACKAGES, host)).flatMap((owner) => list(join(PACKAGES, host, owner)).map((repo) => join(PACKAGES, host, owner, repo))));
}

/* Every executable of a kind, as a Map of name to path: ~/.ask's first, then each package's. */
export function find(kind) {
  const found = new Map();
  for (const dir of [join(HOME, kind), ...packageDirs().map((pkg) => join(pkg, kind))])
    for (const name of list(dir)) if (NAME.test(name) && !found.has(name) && executable(join(dir, name))) found.set(name, join(dir, name));
  return found;
}
