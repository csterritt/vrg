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
const DRY_RUN = Bun.argv.includes("--dry-run");
const POSITIONAL_ARGS = Bun.argv.slice(2).filter((arg) => !arg.startsWith("--"));
const BRANCH = POSITIONAL_ARGS[0];
const START_MATCH = POSITIONAL_ARGS[1];

if (!BRANCH || !START_MATCH) {
  console.error(
    "Usage: bun scripts/unify-history.ts <local-branch> <start-commit-match> [--dry-run]",
  );
  process.exit(1);
}

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
  const output = await run("jj", [
    "log",
    "--no-graph",
    "--reversed",
    "-r",
    `::${BRANCH}`,
    "-T",
    JJ_TEMPLATE,
  ]);
  return parseJjLog(output);
}

async function getRemoteCommits(): Promise<CommitEntry[]> {
  const remoteCommand = `${REMOTE_PATH_PREFIX} && cd ${REMOTE_DIR} && jj log --no-graph --reversed -T '${JJ_TEMPLATE}'`;
  const output = await run("ssh", [REMOTE_HOST, remoteCommand]);
  return parseJjLog(output);
}

// remoteCommits is oldest-first; drop everything before the first commit whose
// change id or description contains START_MATCH.
function remoteCommitsFromMatch(remoteCommits: CommitEntry[]): CommitEntry[] {
  const startIndex = remoteCommits.findIndex(
    (commit) => commit.description.includes(START_MATCH) || commit.changeId.includes(START_MATCH),
  );
  if (startIndex === -1) {
    throw new Error(`No remote commit matches "${START_MATCH}".`);
  }
  const matched = remoteCommits[startIndex]!;
  log(`Starting at remote commit ${matched.changeId}: ${matched.description} (skipped ${startIndex} older).`);
  return remoteCommits.slice(startIndex);
}

async function printDryRunPlan(): Promise<void> {
  log("Dry run: fetching local and remote commit maps...");
  const localCommits = await getLocalCommits();
  const remoteCommits = remoteCommitsFromMatch(await getRemoteCommits());
  const localDescriptions = new Set(localCommits.map((c) => c.description));
  log(`Local branch '${BRANCH}': ${localCommits.length} commits, remote: ${remoteCommits.length} commits.`);
  const missing = remoteCommits.filter((c) => !localDescriptions.has(c.description));
  if (missing.length === 0) {
    log("All remote descriptions are present on the local branch. Nothing to do.");
    return;
  }
  log(`Would apply ${missing.length} remote commit(s) to '${BRANCH}':`);
  for (const commit of missing) {
    log(`${commit.changeId}: ${commit.description}`);
    log(`  ssh ${REMOTE_HOST} '${REMOTE_PATH_PREFIX} && cd ${REMOTE_DIR} && jj edit ${commit.changeId}'`);
    log(`  jj new ${BRANCH}`);
    log("  bash scripts/pull-up-new.sh");
    log(`  jj describe -m "${commit.description}"`);
    log(`  jj bookmark set ${BRANCH} -r @`);
  }
}

async function main(): Promise<void> {
  if (DRY_RUN) {
    await printDryRunPlan();
    return;
  }
  for (;;) {
    log("Fetching local and remote commit maps...");
    const localCommits = await getLocalCommits();
    const remoteCommits = remoteCommitsFromMatch(await getRemoteCommits());
    const localDescriptions = new Set(localCommits.map((c) => c.description));
    log(`Local branch '${BRANCH}': ${localCommits.length} commits, remote: ${remoteCommits.length} commits.`);

    // remoteCommits is oldest-first (reversed); find the first whose description
    // is not yet present on the local branch.
    const target = remoteCommits.find((c) => !localDescriptions.has(c.description));

    if (!target) {
      log(`All remote descriptions are present on '${BRANCH}'. Done.`);
      return;
    }

    log(`Selected remote commit ${target.changeId}: ${target.description}`);

    log(`Running on remote: jj edit ${target.changeId}`);
    await runInteractive("ssh", [
      REMOTE_HOST,
      `${REMOTE_PATH_PREFIX} && cd ${REMOTE_DIR} && jj edit ${target.changeId}`,
    ]);

    log(`Running: jj new ${BRANCH}`);
    await run("jj", ["new", BRANCH]);

    log("Running: scripts/pull-up-new.sh");
    await runInteractive("bash", ["scripts/pull-up-new.sh"]);

    log(`Running: jj describe -m "${target.description}"`);
    await run("jj", ["describe", "-m", target.description]);

    log(`Running: jj bookmark set ${BRANCH} -r @`);
    await run("jj", ["bookmark", "set", BRANCH, "-r", "@"]);

    log("Iteration complete.");

    break
  }
}

main().catch((error) => {
  console.error(`unify-history failed: ${error instanceof Error ? error.message : error}`);
  process.exit(1);
});
