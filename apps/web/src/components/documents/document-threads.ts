import type { CommentThread } from "@/api/ops.gen";

type TThreadOrder = Pick<CommentThread, "id" | "quotedText" | "resolvedAt" | "createdAt"> &
  Partial<Pick<CommentThread, "suggestionStatus">>;

const time = (value: string) => Date.parse(value);

/**
 * Open threads in the order their quotes appear in the document, then those without a quote
 * (or whose passage is gone) oldest first; resolved threads last resolved first.
 */
export function groupThreads<T extends TThreadOrder>(threads: T[], content: string) {
  const ranked = threads
    .filter((thread) => !thread.resolvedAt)
    .map((thread) => ({ thread, at: thread.quotedText ? content.indexOf(thread.quotedText) : -1 }));
  const anchored = ranked
    .filter(({ at }) => at >= 0)
    .sort((a, b) => a.at - b.at || time(a.thread.createdAt) - time(b.thread.createdAt))
    .map(({ thread }) => thread);
  const loose = ranked
    .filter(({ at }) => at < 0)
    .map(({ thread }) => thread)
    .sort((a, b) => time(a.createdAt) - time(b.createdAt));
  const resolved = threads
    .filter((thread) => thread.resolvedAt)
    .sort((a, b) => time(b.resolvedAt ?? "") - time(a.resolvedAt ?? ""));
  return { open: [...anchored, ...loose], resolved };
}

/** The thread a link points at as #comment-<id>, the id of its first comment or of a reply. */
export function linkedThreadId(threads: { id: string; replies: { id: string }[] }[], hash: string) {
  if (!hash.startsWith("comment-")) return null;
  const id = hash.slice("comment-".length);
  const thread = threads.find((item) => item.id === id || item.replies.some((r) => r.id === id));
  return thread?.id ?? null;
}

/** The passages to highlight: open comments and pending suggestions that quote one. */
export function threadAnnotations(threads: TThreadOrder[]) {
  return threads.flatMap((thread) => {
    if (thread.resolvedAt || !thread.quotedText) return [];
    if (thread.suggestionStatus && thread.suggestionStatus !== "pending") return [];
    return [{ id: thread.id, quote: thread.quotedText }];
  });
}
