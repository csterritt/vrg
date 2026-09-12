#!/usr/bin/env bun

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { basename, join } from "node:path";

const execFileP = promisify(execFile);
const ROOT = join(import.meta.dir, "..");
const REMOTE_HOST = "chris@utmtwo";
const REMOTE_DIR = basename(ROOT);
const REMOTE_PATH_PREFIX = "export PATH=/home/linuxbrew/.linuxbrew/bin:$PATH";
const JJ_TEMPLATE =
  'change_id.short() ++ "\\t" ++ if(empty, "EMPTY", "nonempty") ++ "\\t" ++ description.trim() ++ "\\n"';

interface CommitEntry {
  changeId: string;
  description: string;
}

function log(message: string): void {
  const now = new Date();
  const timestamp = `${String(now.getMonth() + 1)}/${String(now.getDate()).padStart(2, "0")} ${String(now.getHours()).padStart(2, "0")}:${String(now.getMinutes()).padStart(2, "0")}`;
  console.log(`[${timestamp}] ${message}`);
}

async function run(command: string, args: string[]): Promise<string> {
  const { stdout } = await execFileP(command, args, { cwd: ROOT });
  return stdout.toString().trim();
}

async function runInteractive(command: string, args: string[]): Promise<void> {
  const child = Bun.spawn([command, ...args], {
    cwd: ROOT,
    stdin: "inherit",
    stdout: "inherit",
    stderr: "inherit",
  });
  const exitCode = await child.exited;
  if (exitCode !== 0) {
    throw new Error(`${command} ${args.join(" ")} exited with status ${exitCode}`);
  }
}

function parseJjLog(output: string): CommitEntry[] {
  const entries: CommitEntry[] = [];
  for (const line of output.split("\n")) {
    if (line === "") continue;
    const parts = line.split("\t");
    const changeId = parts[0] ?? "";
    const emptyFlag = parts[1] ?? "";
    const description = parts.slice(2).join("\t").trim();
    if (emptyFlag === "EMPTY") continue;
    if (description === "") continue;
    entries.push({ changeId, description });
  }
  return entries;
}

async function getLocalCommits(): Promise<CommitEntry[]> {
  const output = await run("jj", ["log", "--no-graph", "--reversed", "-T", JJ_TEMPLATE]);
  return parseJjLog(output);
}

async function getRemoteCommits(): Promise<CommitEntry[]> {
  const remoteCommand = `${REMOTE_PATH_PREFIX} && cd ${REMOTE_DIR} && jj log --no-graph --reversed -T '${JJ_TEMPLATE}'`;
  const output = await run("ssh", [REMOTE_HOST, remoteCommand]);
  return parseJjLog(output);
}

async function main(): Promise<void> {
  for (;;) {
    log("Fetching local and remote commit maps...");
    const localCommits = await getLocalCommits();
    const remoteCommits = await getRemoteCommits();
    const localDescriptions = new Set(localCommits.map((c) => c.description));
    log(`Local: ${localCommits.length} commits, remote: ${remoteCommits.length} commits.`);

    // remoteCommits is oldest-first (reversed); find the first whose description
    // is not yet present on the local machine.
    const target = remoteCommits.find((c) => !localDescriptions.has(c.description));

    if (!target) {
      log("All remote descriptions are present locally. Done.");
      return;
    }

    log(`Selected remote commit ${target.changeId}: ${target.description}`);

    log(`Running on remote: jj edit ${target.changeId}`);
    await runInteractive("ssh", [
      REMOTE_HOST,
      `${REMOTE_PATH_PREFIX} && cd ${REMOTE_DIR} && jj edit ${target.changeId}`,
    ]);

    log("Running: scripts/pull-up-new.sh");
    await runInteractive("bash", ["scripts/pull-up-new.sh"]);

    log(`Running: jj describe -m "${target.description}"`);
    await run("jj", ["describe", "-m", target.description]);

    log("Running: jj new");
    await run("jj", ["new"]);

    log("Iteration complete.");
  }
}

main().catch((error) => {
  console.error(`unify-history failed: ${error instanceof Error ? error.message : error}`);
  process.exit(1);
});
