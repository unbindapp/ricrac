import type { TImportIssue } from "@/lib/queries/imports";

export const MAX_IMPORT_ISSUES = 2000;

export type TParsedIssues = {
  issues: TImportIssue[];
  comments: number;
  /** Issues that keep a number of their own. */
  numbered: number;
};

type TResult = { ok: true; parsed: TParsedIssues } | { ok: false; error: string };

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

/** Reads issues to import: a JSON array of issues, or an object with an issues array. */
export function parseIssuesJson(text: string): TResult {
  let json: unknown;
  try {
    json = JSON.parse(text);
  } catch (error) {
    const reason = error instanceof Error ? error.message : "";
    return { ok: false, error: `This isn't valid JSON. ${reason}`.trim() };
  }
  const list = Array.isArray(json) ? json : isObject(json) ? json.issues : undefined;
  if (!Array.isArray(list)) {
    return { ok: false, error: 'Expected a list of issues, or an object with an "issues" list.' };
  }
  if (list.length === 0) return { ok: false, error: "The list has no issues." };
  if (list.length > MAX_IMPORT_ISSUES) {
    return {
      ok: false,
      error: `That's ${list.length} issues; import at most ${MAX_IMPORT_ISSUES} at a time. Split the file, parents first.`,
    };
  }
  for (const [index, issue] of list.entries()) {
    if (!isObject(issue)) return { ok: false, error: `Issue ${index + 1} isn't an object.` };
    if (typeof issue.title !== "string" || !issue.title.trim()) {
      return { ok: false, error: `Issue ${index + 1} has no title.` };
    }
  }
  const issues = list as TImportIssue[];
  return {
    ok: true,
    parsed: {
      issues,
      comments: issues.reduce((sum, issue) => sum + (issue.comments?.length ?? 0), 0),
      numbered: issues.filter((issue) => typeof issue.number === "number").length,
    },
  };
}

export const EXAMPLE_IMPORT = `{
  "issues": [
    {
      "number": 42,
      "title": "Checkout fails on Safari",
      "description": "Steps to reproduce:\\n\\n1. Add an item\\n2. Pay",
      "state": "In Progress",
      "priority": 2,
      "assignee": "ann@acme.test",
      "creator": "bob@acme.test",
      "labels": ["Bug"],
      "createdAt": "2025-03-02T10:15:00Z",
      "comments": [
        { "author": "bob@acme.test", "body": "Only on iOS 17.", "createdAt": "2025-03-02T11:00:00Z" }
      ]
    },
    { "number": 43, "title": "Retry failed payments", "parent": "ENG-42", "state": "Todo" }
  ]
}`;
