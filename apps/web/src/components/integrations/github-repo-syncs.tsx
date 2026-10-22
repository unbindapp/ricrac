import type { GitHubRepoSync } from "@/api/ops.gen";
import { GithubIcon } from "@/components/icons/github";
import { FolderSyncIcon, PencilIcon, PlusIcon, Trash2Icon } from "@/components/icons/symbols.gen";
import { GithubRepoSyncDialog } from "@/components/integrations/github-repo-sync-dialog";
import { EmptyState } from "@/components/layout/empty-state";
import { List, ListRow } from "@/components/layout/list";
import { SettingsSection } from "@/components/layout/settings-section";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { importSummary } from "@/lib/helpers/github-sync";
import { useDeleteGithubRepoSyncMutation } from "@/lib/queries/github";

export function GithubRepoSyncs({ repoSyncs }: { repoSyncs: GitHubRepoSync[] }) {
  const addButton = (
    <GithubRepoSyncDialog
      trigger={
        <Button size="sm" variant="outline">
          <PlusIcon className="size-4" />
          Sync repository
        </Button>
      }
    />
  );
  return (
    <SettingsSection
      title="Synced repositories"
      description="Issues, comments, labels, assignees and status stay in step both ways with these repositories."
      actions={repoSyncs.length > 0 && addButton}
    >
      {repoSyncs.length === 0 ? (
        <EmptyState
          icon={FolderSyncIcon}
          title="No synced repositories yet"
          description="Sync a repository to bring its GitHub issues into Ricrac. Pull requests link to issues without a sync."
          action={addButton}
        />
      ) : (
        <List>
          {repoSyncs.map((repoSync) => (
            <RepoSyncRow key={repoSync.id} repoSync={repoSync} />
          ))}
        </List>
      )}
    </SettingsSection>
  );
}

function RepoSyncRow({ repoSync }: { repoSync: GitHubRepoSync }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: remove } = useDeleteGithubRepoSyncMutation(workspace.id);
  const name = repoSync.repository.fullName;

  return (
    <ListRow
      actions={
        <>
          <SyncStatus repoSync={repoSync} />
          <span className="text-muted-foreground hidden text-xs whitespace-nowrap md:inline">
            {repoSync.linkedIssueCount} {repoSync.linkedIssueCount === 1 ? "issue" : "issues"}
          </span>
          {repoSync.lastSyncedAt && (
            <span className="text-muted-foreground hidden text-xs lg:inline">
              <RelativeTime value={repoSync.lastSyncedAt} />
            </span>
          )}
          <GithubRepoSyncDialog
            repoSync={repoSync}
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Edit ${name}`}>
                <PencilIcon className="size-4" />
              </Button>
            }
          />
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Stop syncing ${name}`}>
                <Trash2Icon className="size-4" />
              </Button>
            }
            title={`Stop syncing ${name}`}
            description="Its issues stay in Ricrac, no longer linked to GitHub. Pull requests keep linking to issues."
            confirmLabel="Stop syncing repository"
            onConfirm={() => remove(repoSync.id)}
          />
        </>
      }
    >
      <GithubIcon className="text-muted-foreground" />
      <a
        href={repoSync.repository.url}
        target="_blank"
        rel="noreferrer noopener"
        className="min-w-0 truncate outline-hidden focus-visible:underline has-hover:hover:underline"
      >
        {name}
      </a>
      {repoSync.project && (
        <span className="text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs">
          <span aria-hidden>→</span>
          <span className="truncate">{repoSync.project.name}</span>
        </span>
      )}
    </ListRow>
  );
}

function SyncStatus({ repoSync }: { repoSync: GitHubRepoSync }) {
  const backfill = repoSync.backfill;
  const summary = importSummary(backfill);
  if (backfill?.status === "failed") {
    return (
      <SimpleTooltip content={repoSync.lastError ?? "The import failed."}>
        <Badge variant="destructive">Import failed</Badge>
      </SimpleTooltip>
    );
  }
  if (repoSync.lastError) {
    return (
      <SimpleTooltip content={repoSync.lastError}>
        <Badge variant="warning">Sync error</Badge>
      </SimpleTooltip>
    );
  }
  if (backfill?.status === "pending" || backfill?.status === "running") {
    return (
      <SimpleTooltip content={summary ?? "Starting the import"}>
        <Badge variant="process">Importing</Badge>
      </SimpleTooltip>
    );
  }
  return (
    <SimpleTooltip content={summary}>
      <Badge variant="success">Synced</Badge>
    </SimpleTooltip>
  );
}
