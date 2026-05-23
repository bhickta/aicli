import type { InboxDestinationDiff } from "../../types";

export interface DiffRow {
  id: string;
  type: "same" | "changed" | "inserted" | "deleted";
  beforeNumber?: number;
  afterNumber?: number;
  beforeText: string;
  afterText: string;
}

interface DiffOp {
  kind: "same" | "delete" | "insert";
  text: string;
  beforeNumber?: number;
  afterNumber?: number;
}

export function buildDiffRows(diff: InboxDestinationDiff): DiffRow[] {
  const before = splitLines(diff.before || "");
  const after = splitLines(diff.after || "");
  if (!before.length && !after.length && diff.diff) return buildRowsFromUnifiedDiff(diff.diff);
  return pairChangeBlocks(buildOps(before, after));
}

function splitLines(value: string): string[] {
  if (!value) return [];
  const lines = value.replace(/\r\n/g, "\n").split("\n");
  if (lines[lines.length - 1] === "") lines.pop();
  return lines;
}

function buildOps(before: string[], after: string[]): DiffOp[] {
  if (before.length * after.length > 500_000) return buildIndexedOps(before, after);

  const dp = Array.from({ length: before.length + 1 }, () => Array(after.length + 1).fill(0));
  for (let i = before.length - 1; i >= 0; i--) {
    for (let j = after.length - 1; j >= 0; j--) {
      dp[i][j] = before[i] === after[j]
        ? dp[i + 1][j + 1] + 1
        : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }

  const ops: DiffOp[] = [];
  let i = 0;
  let j = 0;
  while (i < before.length || j < after.length) {
    if (i < before.length && j < after.length && before[i] === after[j]) {
      ops.push({ kind: "same", text: before[i], beforeNumber: i + 1, afterNumber: j + 1 });
      i++;
      j++;
    } else if (j < after.length && (i === before.length || dp[i][j + 1] >= dp[i + 1][j])) {
      ops.push({ kind: "insert", text: after[j], afterNumber: j + 1 });
      j++;
    } else if (i < before.length) {
      ops.push({ kind: "delete", text: before[i], beforeNumber: i + 1 });
      i++;
    }
  }
  return ops;
}

function buildIndexedOps(before: string[], after: string[]): DiffOp[] {
  const max = Math.max(before.length, after.length);
  const ops: DiffOp[] = [];
  for (let i = 0; i < max; i++) {
    if (before[i] === after[i]) {
      ops.push({ kind: "same", text: before[i] || "", beforeNumber: i + 1, afterNumber: i + 1 });
    } else {
      if (i < before.length) ops.push({ kind: "delete", text: before[i], beforeNumber: i + 1 });
      if (i < after.length) ops.push({ kind: "insert", text: after[i], afterNumber: i + 1 });
    }
  }
  return ops;
}

function pairChangeBlocks(ops: DiffOp[]): DiffRow[] {
  const rows: DiffRow[] = [];
  let index = 0;
  let rowIndex = 0;

  while (index < ops.length) {
    const op = ops[index];
    if (op.kind === "same") {
      rows.push({
        id: `same-${rowIndex++}`,
        type: "same",
        beforeNumber: op.beforeNumber,
        afterNumber: op.afterNumber,
        beforeText: op.text,
        afterText: op.text,
      });
      index++;
      continue;
    }

    const deleted: DiffOp[] = [];
    const inserted: DiffOp[] = [];
    while (index < ops.length && ops[index].kind !== "same") {
      if (ops[index].kind === "delete") deleted.push(ops[index]);
      if (ops[index].kind === "insert") inserted.push(ops[index]);
      index++;
    }

    const blockLength = Math.max(deleted.length, inserted.length);
    for (let i = 0; i < blockLength; i++) {
      const left = deleted[i];
      const right = inserted[i];
      rows.push({
        id: `change-${rowIndex++}`,
        type: left && right ? "changed" : left ? "deleted" : "inserted",
        beforeNumber: left?.beforeNumber,
        afterNumber: right?.afterNumber,
        beforeText: left?.text || "",
        afterText: right?.text || "",
      });
    }
  }

  return rows;
}

function buildRowsFromUnifiedDiff(diff: string): DiffRow[] {
  const rows: DiffRow[] = [];
  let beforeNumber = 0;
  let afterNumber = 0;
  for (const rawLine of splitLines(diff)) {
    if (rawLine.startsWith("---") || rawLine.startsWith("+++")) continue;
    if (rawLine.startsWith("-")) {
      rows.push({
        id: `delete-${rows.length}`,
        type: "deleted",
        beforeNumber: ++beforeNumber,
        beforeText: rawLine.slice(1),
        afterText: "",
      });
    } else if (rawLine.startsWith("+")) {
      rows.push({
        id: `insert-${rows.length}`,
        type: "inserted",
        afterNumber: ++afterNumber,
        beforeText: "",
        afterText: rawLine.slice(1),
      });
    } else {
      const text = rawLine.startsWith(" ") ? rawLine.slice(1) : rawLine;
      rows.push({
        id: `same-${rows.length}`,
        type: "same",
        beforeNumber: ++beforeNumber,
        afterNumber: ++afterNumber,
        beforeText: text,
        afterText: text,
      });
    }
  }
  return rows;
}
