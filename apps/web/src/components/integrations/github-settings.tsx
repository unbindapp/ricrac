import { useQuery } from "@tanstack/react-query";

import type { GitHubInstallation, GitHubIntegration } from "@/api/ops.gen";
import { GithubIcon } from "@/components/icons/github";
import {
  ExternalLinkIcon,
  GitMergeIcon,
  GitPullRequestIcon,
  RefreshCwIcon,
  type TIcon,
  UnplugIcon,
} from "@/components/icons/symbols.gen";
import { GithubRepoSyncs } from "@/components/integrations/github-repo-syncs";
import { ErrorCard } from "@/components/layout/error-card";
import { List, ListRow } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SettingsSection } from "@/components/layout/settings-section";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { RelativeTime } from "@/components/shared/relative-time";
import { TeamIcon } from "@/components/shared/team-icon";
import { Badge } from "@/components/ui/badge";
import { Button, LinkButton } from "@/components/ui/button";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { githubEventLabel } from "@/lib/helpers/github-sync";
import {
  githubIntegrationQuery,
  useConnectGithubMutation,
  useDisconnectGithubMutation,
} from "@/lib/queries/github";

const description =
  "Pull requests, branches and commits that mention an issue like ENG-123 link to it and move it through the workflow. Synced repositories keep their issues in step with a team.";

export function GithubSettings() {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(githubIntegrationQuery(workspace.id));

  if (isPending) return <ListSkeleton rows={3} />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  if (!data.configured) return <GithubNotConfigured />;
  if (data.installations.length === 0) return <GithubNotConnected />;
  return <GithubConnected integration={data} />;
}

function GithubNotConfigured() {
  return (
    <SettingsSection
      title="GitHub"
      description={description}
      actions={<Badge variant="muted">Not set up</Badge>}
    >
      <div className="bg-muted flex flex-col gap-1 rounded-lg p-4">
        <p className="text-sm font-medium">This server has no GitHub App</p>
        <p className="text-muted-foreground text-sm">
          Whoever runs this Ricrac server creates a GitHub App and sets the GITHUB_ variables, as
          described in docs/github-app.md. Connect GitHub here once they have.
        </p>
      </div>
    </SettingsSection>
  );
}

function GithubNotConnected() {
  return (
    <SettingsSection title="GitHub" description={description} actions={<ConnectButton />}>
      <ul className="flex flex-col gap-3">
        <Feature
          icon={GitPullRequestIcon}
          title="Link pull requests"
          text="Branches, pull requests and commits that mention an issue show on it."
        />
        <Feature
          icon={GitMergeIcon}
          title="Move issues automatically"
          text="Opening a pull request starts the issue; merging one that fixes it completes it."
        />
        <Feature
          icon={RefreshCwIcon}
          title="Sync GitHub issues"
          text="Keep a repository's issues and comments in step with a team, both ways."
        />
      </ul>
    </SettingsSection>
  );
}

function Feature({ icon: Icon, title, text }: { icon: TIcon; title: string; text: string }) {
  return (
    <li className="flex items-start gap-3">
      <span className="bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-lg">
        <Icon className="size-4" />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="text-sm font-medium">{title}</span>
        <span className="text-muted-foreground text-sm">{text}</span>
      </span>
    </li>
  );
}

type TConnectButtonProps = { label?: string; variant?: "default" | "outline" };

function ConnectButton({ label = "Connect GitHub", variant = "default" }: TConnectButtonProps) {
  const { workspace } = useWorkspace();
  const { mutate: connect, isPending, isSuccess } = useConnectGithubMutation(workspace.id);
  return (
    <Button
      size="sm"
      variant={variant}
      isPending={isPending || isSuccess}
      onClick={() =>
        connect(undefined, { onSuccess: ({ installUrl }) => window.location.assign(installUrl) })
      }
    >
      <GithubIcon />
      {label}
    </Button>
  );
}

function GithubConnected({ integration }: { integration: GitHubIntegration }) {
  return (
    <>
      <SettingsSection
        title="GitHub"
        description={description}
        actions={<ConnectButton label="Add account" variant="outline" />}
      >
        <List>
          {integration.installations.map((installation) => (
            <InstallationRow key={installation.id} installation={installation} />
          ))}
        </List>
        {integration.githubLogin && (
          <p className="text-muted-foreground text-xs">
            You are linked to GitHub as @{integration.githubLogin}.
          </p>
        )}
      </SettingsSection>
      <GithubRepoSyncs repoSyncs={integration.repoSyncs} />
      <PullRequestStatuses />
    </>
  );
}

function InstallationRow({ installation }: { installation: GitHubInstallation }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: disconnect } = useDisconnectGithubMutation(workspace.id);
  const account = installation.account;
  return (
    <ListRow
      actions={
        <>
          <span className="text-muted-foreground hidden text-xs sm:inline">
            {installation.lastEvent && installation.lastEventAt ? (
              <>
                {githubEventLabel(installation.lastEvent)} ·{" "}
                <RelativeTime value={installation.lastEventAt} />
              </>
            ) : (
              "No events yet"
            )}
          </span>
          <Button
            variant="ghost"
            size="sm"
            render={<a href={installation.manageUrl} target="_blank" rel="noreferrer noopener" />}
          >
            Manage
            <ExternalLinkIcon className="size-3.5" />
          </Button>
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Disconnect ${account}`}>
                <UnplugIcon className="size-4" />
              </Button>
            }
            title={`Disconnect ${account}`}
            description="Its repositories stop syncing and its pull requests stop linking. Issues, comments and links already in Ricrac stay. Uninstall the app on GitHub with Manage to remove its access there too."
            confirmLabel="Disconnect GitHub account"
            onConfirm={() => disconnect(String(installation.id))}
          />
        </>
      }
    >
      <GithubIcon />
      <span className="truncate font-medium">{account}</span>
      <Badge variant="muted">
        {installation.accountType === "Organization" ? "Organization" : "Personal"}
      </Badge>
      {installation.suspendedAt && <Badge variant="warning">Suspended</Badge>}
    </ListRow>
  );
}

function PullRequestStatuses() {
  const { workspace } = useWorkspace();
  const { teams } = useTeams();
  if (teams.length === 0) return null;
  return (
    <SettingsSection
      title="Pull request statuses"
      description="Each team picks the statuses its issues move to when pull requests open and merge."
    >
      <List>
        {teams.map((team) => (
          <ListRow
            key={team.id}
            actions={
              <LinkButton
                to="/$workspace/settings/team/$key/github"
                params={{ workspace: workspace.urlKey, key: team.key }}
                variant="ghost"
                size="sm"
              >
                Configure
              </LinkButton>
            }
          >
            <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />
            <span className="truncate">{team.name}</span>
          </ListRow>
        ))}
      </List>
    </SettingsSection>
  );
}
