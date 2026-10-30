type TBackfill = {
  status: "pending" | "running" | "completed" | "failed";
  issuesCreated: number;
  issuesUpdated: number;
  commentsCreated: number;
  skipped: number;
};

type TRepoSyncImport = { backfill: TBackfill | null };

export function isImporting(repoSync: TRepoSyncImport) {
  const status = repoSync.backfill?.status;
  return status === "pending" || status === "running";
}

function plural(count: number, one: string, many: string) {
  return `${count} ${count === 1 ? one : many}`;
}

/** What the import of a repository's existing issues did so far, like "12 issues created, 3 comments". */
export function importSummary(backfill: TBackfill | null): string | null {
  if (!backfill || backfill.status === "pending") return null;
  const parts = [
    `${plural(backfill.issuesCreated, "issue", "issues")} created`,
    `${backfill.issuesUpdated} updated`,
    plural(backfill.commentsCreated, "comment", "comments"),
  ];
  if (backfill.skipped > 0) parts.push(`${backfill.skipped} skipped`);
  return parts.join(", ");
}

const githubErrors: Record<string, string> = {
  expired: "The link to GitHub expired before you came back. Start again.",
  signed_out: "You were signed out while on GitHub. Sign in and try again.",
  not_admin: "Only workspace admins can connect GitHub.",
  requested:
    "GitHub asked an owner of the organization to approve the app. Connect again once they have.",
  no_installation: "GitHub didn't say which installation was made. Try again.",
  not_authorized:
    "GitHub didn't let Ricrac confirm who you are. Try again and authorize the app when GitHub asks.",
  github_failed: "GitHub didn't answer. Try again in a moment.",
  not_your_installation: "Your GitHub account can't reach that installation.",
  connected_elsewhere: "That GitHub installation is connected to another workspace.",
  not_configured: "This Ricrac server has no GitHub App set up.",
  failed: "Something went wrong while saving the connection. Try again.",
};

/** The message for a github_error the server sends people back with. */
export function githubErrorMessage(reason: string) {
  return githubErrors[reason] ?? githubErrors.failed;
}

/** Turns "pull_request.opened" into "Pull request opened". */
export function githubEventLabel(event: string) {
  const text = event.replace(/[._]/g, " ").trim();
  return text.charAt(0).toUpperCase() + text.slice(1);
}

type TIssueRepoSync = {
  linkedIssueCount: number;
  createdAt: string;
  project: { id: string } | null;
};

/** The synced repositories a new issue can open in, the one the server picks for its project first. */
export function issueRepositories<T extends TIssueRepoSync>(repoSyncs: T[], projectId?: string) {
  const isProjects = (repoSync: T) => Boolean(projectId) && repoSync.project?.id === projectId;
  return [...repoSyncs].sort(
    (a, b) =>
      Number(isProjects(b)) - Number(isProjects(a)) ||
      b.linkedIssueCount - a.linkedIssueCount ||
      a.createdAt.localeCompare(b.createdAt),
  );
}
