#!/usr/bin/env bun
/**
 * Convert a session-history file into CSV on stdout.
 *
 * The input contains one block per task separated by hyphen-only lines:
 *
 *   Task 001-go-scaffold-cli-positionals-and-root.md
 *
 *   Agent SWE-2
 *
 *   Usage
 *    Agent messages: 90 messages
 *    Input tokens: 389366 tokens
 *    Output tokens: 100161 tokens
 *    Cached input tokens: 11361369 tokens
 *
 *   ----
 *
 * Each block becomes one row:
 *   001-go-scaffold-cli-positionals-and-root.md,SWE-2,90,389366,100161,11361369
 */

import { readFile } from "node:fs/promises";

const HEADER =
  "Task,Agent,Agent messages,Input tokens,Output tokens,Cached input tokens";
const SEPARATOR_RE = /^-+$/m;
const TASK_RE = /^Task\s+(.+)$/;
const AGENT_MESSAGES_RE = /^Agent messages:\s*(\d+)/;
const CACHED_INPUT_TOKENS_RE = /^Cached input tokens:\s*(\d+)/;
const INPUT_TOKENS_RE = /^Input tokens:\s*(\d+)/;
const OUTPUT_TOKENS_RE = /^Output tokens:\s*(\d+)/;
const AGENT_RE = /^Agent\s+(.+)$/;

const INPUT_PATH = Bun.argv.slice(2).find((arg) => !arg.startsWith("--"));

if (!INPUT_PATH) {
  console.error("Usage: bun scripts/history-to-csv.ts <history-file>");
  process.exit(1);
}

interface HistoryBlock {
  task: string;
  agent: string;
  agentMessages: string;
  inputTokens: string;
  outputTokens: string;
  cachedInputTokens: string;
}

// Order matters: "Agent messages:" must be tried before "Agent", and
// "Cached input tokens:" before "Input tokens:".
const FIELD_PATTERNS: ReadonlyArray<readonly [keyof HistoryBlock, RegExp]> = [
  ["task", TASK_RE],
  ["agentMessages", AGENT_MESSAGES_RE],
  ["cachedInputTokens", CACHED_INPUT_TOKENS_RE],
  ["inputTokens", INPUT_TOKENS_RE],
  ["outputTokens", OUTPUT_TOKENS_RE],
  ["agent", AGENT_RE],
];

function parseBlock(blockText: string): HistoryBlock | null {
  const block: HistoryBlock = {
    task: "",
    agent: "",
    agentMessages: "",
    inputTokens: "",
    outputTokens: "",
    cachedInputTokens: "",
  };
  for (const rawLine of blockText.split("\n")) {
    const line = rawLine.trim();
    if (line === "") continue;
    for (const [field, pattern] of FIELD_PATTERNS) {
      const match = line.match(pattern);
      if (!match) continue;
      block[field] = match[1]!;
      break;
    }
  }
  if (block.task === "") return null;
  return block;
}

function csvField(value: string): string {
  if (!/[",\n]/.test(value)) return value;
  return `"${value.replace(/"/g, '""')}"`;
}

function toCsvRow(block: HistoryBlock): string {
  return [
    block.task,
    block.agent,
    block.agentMessages,
    block.inputTokens,
    block.outputTokens,
    block.cachedInputTokens,
  ]
    .map(csvField)
    .join(",");
}

async function main(): Promise<void> {
  const text = await readFile(INPUT_PATH, "utf8");
  const chunks = text
    .replace(/\r\n/g, "\n")
    .split(SEPARATOR_RE)
    .map((chunk) => chunk.trim())
    .filter((chunk) => chunk !== "");
  const rows = chunks
    .map(parseBlock)
    .filter((block): block is HistoryBlock => block !== null)
    .map(toCsvRow);
  process.stdout.write(`${HEADER}\n${rows.join("\n")}\n`);
}

main().catch((error) => {
  console.error(
    `history-to-csv failed: ${error instanceof Error ? error.message : error}`,
  );
  process.exit(1);
});
