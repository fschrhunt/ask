/*
 * Starting and stopping harness processes. Each harness runs in its own process group, so stopping
 * it also stops the agent CLI it started. Stopping ask (SIGINT or SIGTERM) stops every group it
 * started: SIGTERM first, SIGKILL after GRACE_MS, then ask exits 130. Once stopping, nothing new
 * starts.
 */
import { spawn } from 'node:child_process';

// Milliseconds a stopped process group gets to exit before it is killed.
const GRACE_MS = 5000;

const groups = new Set();
let stopping = false;

/* Sends a signal to a whole process group; a group that is already gone is ignored. */
function signalGroup(pid, signal) {
  try {
    process.kill(-pid, signal);
  } catch {}
}

/* True while any process of the group is alive. */
function groupAlive(pid) {
  try {
    process.kill(-pid, 0);
    return true;
  } catch {
    return false;
  }
}

for (const signal of ['SIGINT', 'SIGTERM'])
  process.on(signal, async () => {
    if (stopping) return;
    stopping = true;
    const live = [...groups];
    for (const pid of live) signalGroup(pid, 'SIGTERM');
    const deadline = Date.now() + GRACE_MS;
    while (live.some(groupAlive)) {
      if (Date.now() > deadline) live.forEach((pid) => signalGroup(pid, 'SIGKILL'));
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    process.exit(130);
  });

/*
 * Runs a command with `input` on stdin and a hard timeout. Resolves with { code, stdout, stderr,
 * timedOut }; never rejects. A timed-out run's whole process group gets SIGTERM, then SIGKILL.
 */
export function run(command, args, { input = '', cwd, env, timeoutMs }) {
  return new Promise((resolve) => {
    if (stopping) return resolve({ code: null, stdout: '', stderr: 'ask was stopped', timedOut: false });
    const child = spawn(command, args, { cwd, env: { ...process.env, ...env }, stdio: ['pipe', 'pipe', 'pipe'], detached: true });
    if (child.pid) groups.add(child.pid);
    let stdout = '';
    let stderr = '';
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      signalGroup(child.pid, 'SIGTERM');
      setTimeout(() => signalGroup(child.pid, 'SIGKILL'), GRACE_MS).unref();
    }, timeoutMs);
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', (error) => (stderr += `${error.message}\n`));
    child.stdin.on('error', () => {});
    child.on('close', (code) => {
      clearTimeout(timer);
      groups.delete(child.pid);
      resolve({ code, stdout, stderr, timedOut });
    });
    child.stdin.end(input);
  });
}
