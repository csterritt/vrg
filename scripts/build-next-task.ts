#!/usr/bin/env bun

import { appendFile, readdir, readFile, stat, unlink } from "node:fs/promises";
import { join } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execFileP = promisify(execFile);
const ROOT = join(import.meta.dir, "..");
const TASKS_DIR = join(ROOT, "Notes", "tasks");
const WALKTHROUGHS_DIR = join(ROOT, "Notes", "walkthroughs");
const HISTORY_FILE = join(ROOT, "Notes", "History-my-base-workflow.md");
const STOP_FILE = join(ROOT, "stop");
const DEVIN_MODEL = "glm-5-2";
const MODEL_LABEL = "GLM-5.2-high";
const MIN_WALKTHROUGH_BYTES = 1024;
const RETRY_DELAYS_MS = [30_000, 60_000, 120_000, 300_000, 600_000];
const PREFIX_RE = /^(\d{3})/;
const WALKTHROUGH_RE = /Notes\/walkthroughs\/(\d{3})-(\d+)\/code-walkthrough/;
const DRY_RUN = process.argv.includes("--dry-run");

interface Task {
  file: string;
  prefix: string;
  stepCount: number;
  walkthrough: string;
}

interface Usage {
  agentMessages: number;
  inputTokens: number;
  outputTokens: number;
  cachedInputTokens: number;
}

interface DevinExport {
  session_id?: string;
  steps?: AtifStep[];
}

interface AtifStep {
  metrics?: {
    prompt_tokens?: number;
    completion_tokens?: number;
    cached_tokens?: number;
  };
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

async function isCompleteWalkthrough(path: string): Promise<boolean> {
  try {
    const details = await stat(path);
    return details.isFile() && details.size > MIN_WALKTHROUGH_BYTES;
  } catch {
    return false;
  }
}

async function tasks(): Promise<Task[]> {
  const entries = await readdir(TASKS_DIR);
  const result: Task[] = [];
  for (const file of entries.sort((a, b) => a.localeCompare(b))) {
    if (!file.endsWith(".md")) continue;
    const match = file.match(PREFIX_RE);
    if (!match) continue;
    const taskText = await readFile(join(TASKS_DIR, file), "utf8");
    const walkthroughMatch = taskText.match(WALKTHROUGH_RE);
    if (!walkthroughMatch || walkthroughMatch[1] !== match[1]) {
      throw new Error(`Task ${file} does not declare its expected walkthrough directory`);
    }
    const stepCount = Number(walkthroughMatch[2]);
    const walkthrough = join(
      WALKTHROUGHS_DIR,
      `${walkthroughMatch[1]}-${walkthroughMatch[2]}`,
      "code-walkthrough",
      "walkthrough.md",
    );
    result.push({ file, prefix: match[1], stepCount, walkthrough });
  }
  return result;
}

async function nextTask(): Promise<Task | undefined> {
  for (const task of await tasks()) {
    if (!(await isCompleteWalkthrough(task.walkthrough))) return task;
  }
  return undefined;
}

function emptyUsage(): Usage {
  return { agentMessages: 0, inputTokens: 0, outputTokens: 0, cachedInputTokens: 0 };
}

async function readDevinExport(exportPath: string): Promise<{ sessionId?: string; usage: Usage }> {
  const transcript = JSON.parse(await readFile(exportPath, "utf8")) as DevinExport;
  const usage = emptyUsage();
  for (const step of transcript.steps ?? []) {
    if (!step.metrics) continue;
    const promptTokens = step.metrics.prompt_tokens ?? 0;
    const cachedTokens = step.metrics.cached_tokens ?? 0;
    usage.agentMessages++;
    usage.inputTokens += Math.max(0, promptTokens - cachedTokens);
    usage.outputTokens += step.metrics.completion_tokens ?? 0;
    usage.cachedInputTokens += cachedTokens;
  }
  return { sessionId: transcript.session_id, usage };
}

async function runDevin(
  task: Task,
  attempt: number,
  sessionId?: string,
): Promise<{ exitCode: number; sessionId?: string; usage: Usage }> {
  const exportPath = join(
    "/tmp",
    `build-next-task-${process.pid}-${task.prefix}-${attempt}-${Date.now()}.json`,
  );
  const prompt = `please read through Notes/tasks/${task.file} and do all the work described in that task`;
  let exitCode = -1;
  try {
    const child = Bun.spawn(
      [
        "devin",
        "--model",
        DEVIN_MODEL,
        "--permission-mode",
        "dangerous",
        "--respect-workspace-trust",
        "false",
        ...(sessionId ? ["--resume", sessionId] : []),
        "--export",
        exportPath,
        "--print",
        prompt,
      ],
      { cwd: ROOT, stdin: "ignore", stdout: "inherit", stderr: "inherit" },
    );
    exitCode = await child.exited;
  } catch (error) {
    log(`Could not start Devin for attempt ${attempt}: ${error}`);
  }
  let usage = emptyUsage();
  try {
    const exported = await readDevinExport(exportPath);
    sessionId = exported.sessionId ?? sessionId;
    usage = exported.usage;
  } catch (error) {
    log(`Could not read Devin usage export for attempt ${attempt}: ${error}`);
  } finally {
    await unlink(exportPath).catch(() => undefined);
  }
  return { exitCode, sessionId, usage };
}

async function wait(milliseconds: number): Promise<void> {
  log(`Waiting ${milliseconds / 1000} seconds before retrying...`);
  await Bun.sleep(milliseconds);
}

async function implement(task: Task): Promise<Usage> {
  let usage = emptyUsage();
  let sessionId: string | undefined;
  const maxAttempts = RETRY_DELAYS_MS.length + 1;
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    log(`Invoking Devin for ${task.file}, attempt ${attempt}/${maxAttempts}, model ${DEVIN_MODEL}...`);
    const result = await runDevin(task, attempt, sessionId);
    sessionId = result.sessionId;
    usage = result.usage;
    const walkthroughComplete = await isCompleteWalkthrough(task.walkthrough);
    if (result.exitCode === 0 && walkthroughComplete) {
      log(`Walkthrough verified at ${task.walkthrough}`);
      return usage;
    }
    const reasons = [
      result.exitCode === 0 ? undefined : `Devin exited with status ${result.exitCode}`,
      walkthroughComplete ? undefined : `walkthrough is missing or not over ${MIN_WALKTHROUGH_BYTES} bytes`,
    ].filter(Boolean);
    log(`Attempt ${attempt} failed: ${reasons.join("; ")}`);
    if (attempt === maxAttempts) break;
    await wait(RETRY_DELAYS_MS[attempt - 1]);
  }
  throw new Error(`Giving up on ${task.file} after ${maxAttempts} failed attempts`);
}

async function updateHistory(task: Task, usage: Usage): Promise<void> {
  const entry = [
    "",
    "----",
    `Task ${task.file}`,
    "",
    "Usage",
    ` Agent messages: ${usage.agentMessages} messages`,
    ` Input tokens: ${usage.inputTokens} tokens`,
    ` Output tokens: ${usage.outputTokens} tokens`,
    ` Cached input tokens: ${usage.cachedInputTokens} tokens`,
  ].join("\n") + "\n";
  await appendFile(HISTORY_FILE, entry, "utf8");
}

async function main(): Promise<void> {
  for (;;) {
    // Check for a "stop" file in the project root to break the loop early.
    try {
      await stat(STOP_FILE);
      log("Stop file detected; breaking out of loop.");
      break;
    } catch {
      // No stop file — continue.
    }

    const task = await nextTask();
    if (!task) {
      log("All tasks are implemented. Nothing to do.");
      return;
    }
    const relativeWalkthrough = task.walkthrough.slice(ROOT.length + 1);
    log(`Next task: Notes/tasks/${task.file}`);
    log(`Expected walkthrough: ${relativeWalkthrough} (${task.stepCount} steps, over 1 KiB)`);
    if (DRY_RUN) {
      log(`[dry-run] would run Devin with model ${DEVIN_MODEL} and retry delays of 30s, 1m, 2m, 5m, and 10m.`);
      log(`[dry-run] would describe as: Task Notes/tasks/${task.file} implemented by ${MODEL_LABEL}`);
      return;
    }
    const usage = await implement(task);
    await updateHistory(task, usage);
    const description = `Task Notes/tasks/${task.file} implemented by ${MODEL_LABEL}`;
    log(`Running: jj describe -m "${description}"`);
    await run("jj", ["describe", "-m", description]);
    log("Running: jj new");
    await run("jj", ["new"]);
    log(
      `Recorded Devin usage: ${usage.agentMessages} messages, ${usage.inputTokens} input, ${usage.outputTokens} output, ${usage.cachedInputTokens} cached input tokens.`,
    );
  }
}

main().catch((error) => {
  console.error(`build-next-task failed: ${error instanceof Error ? error.message : error}`);
  process.exit(1);
});
