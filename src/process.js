/*
 * Starting and stopping agent processes. Each agent runs in its own process group, and when it
 * exits, anything left in its group is stopped too, so an agent CLI never outlives its run. Stopping
 * ask (SIGINT or SIGTERM) stops every group it started and exits 130. Stopping means SIGTERM, then
 * SIGKILL after GRACE_MS. Once stopping, nothing new starts.
 */
import { spawn } from 'node:child_process';

// Milliseconds a stopped process group gets to exit before it is killed.
const GRACE_MS = 5000;
// Bytes of stderr kept: enough for the failure reason, not a flood.
const STDERR_KEEP = 64 * 1024;

const groups = new Set();
let stopping = false;

const signalGroup = (pid, signal) => {
  try {
    process.kill(-pid, signal);
  } catch {}
};
const groupAlive = (pid) => {
  try {
    process.kill(-pid, 0);
    return true;
  } catch {
    return false;
  }
};
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

/* Stops process groups: SIGTERM, SIGKILL for any left after GRACE_MS, then waits briefly for them to go. */
async function stopGroups(pids) {
  pids = pids.filter(groupAlive);
  pids.forEach((pid) => signalGroup(pid, 'SIGTERM'));
  for (const deadline = Date.now() + GRACE_MS; pids.some(groupAlive) && Date.now() < deadline; ) await pause(50);
  pids.forEach((pid) => signalGroup(pid, 'SIGKILL'));
  for (const deadline = Date.now() + 1000; pids.some(groupAlive) && Date.now() < deadline; ) await pause(20);
}

for (const signal of ['SIGINT', 'SIGTERM'])
  process.on(signal, async () => {
    if (stopping) return;
    stopping = true;
    await stopGroups([...groups]);
    process.exit(130);
  });

/*
 * Runs a command with `input` on stdin and a hard timeout. Resolves with { code, stdout, stderr,
 * timedOut } once the command and everything in its process group have ended; never rejects.
 */
export function run(command, args, { input = '', cwd, env, timeoutMs }) {
  return new Promise((resolve) => {
    if (stopping) return resolve({ code: null, stdout: '', stderr: 'ask was stopped', timedOut: false });
    const child = spawn(command, args, { cwd, env, stdio: ['pipe', 'pipe', 'pipe'], detached: true });
    if (child.pid) groups.add(child.pid);
    let stdout = '';
    let stderr = '';
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      stopGroups([child.pid]);
    }, timeoutMs);
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr = (stderr + d).slice(-STDERR_KEEP)));
    child.on('error', (error) => (stderr += `${error.message}\n`));
    child.stdin.on('error', () => {});
    child.on('close', async (code) => {
      clearTimeout(timer);
      if (child.pid) {
        await stopGroups([child.pid]);
        groups.delete(child.pid);
      }
      resolve({ code, stdout, stderr, timedOut });
    });
    child.stdin.end(input);
  });
}
