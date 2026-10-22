import { useQuery } from "@tanstack/react-query";
import { useLocation } from "@tanstack/react-router";

import { GithubIcon } from "@/components/icons/github";
import { GithubErrorNotice } from "@/components/integrations/github-error-notice";
import { SettingsSection } from "@/components/layout/settings-section";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  githubAccountConnectHref,
  githubIntegrationQuery,
  useDisconnectGithubAccountMutation,
} from "@/lib/queries/github";
import { meQuery } from "@/lib/queries/me";

const title = "Connected GitHub account";
const description =
  "Your comments on synced issues post on GitHub as you. Pull requests, comments and assignments on GitHub show as yours, and issues assigned to you on GitHub are assigned to you here.";

/** Links your GitHub account in profile settings. */
export function GithubAccountSection() {
  const { workspace } = useWorkspace();
  const { pathname } = useLocation();
  const { data: me } = useQuery(meQuery);
  const integration = useQuery(githubIntegrationQuery(workspace.id));
  const { mutateAsync: unlink } = useDisconnectGithubAccountMutation(workspace.id);
  const login = me?.githubLogin ?? null;

  if (integration.isPending) {
    return (
      <SettingsSection title={title} description={description}>
        <Skeleton className="h-9 w-full" />
      </SettingsSection>
    );
  }
  if (integration.data && !integration.data.configured && !login) {
    return (
      <SettingsSection
        title={title}
        description="This Ricrac server has no GitHub App set up, so GitHub accounts can't be linked yet."
        actions={<Badge variant="muted">Not available</Badge>}
      />
    );
  }
  if (!login) {
    return (
      <>
        <GithubErrorNotice title="Your GitHub account isn't linked" />
        <SettingsSection
          title={title}
          description={description}
          actions={
            <Button
              size="sm"
              variant="outline"
              render={<a href={githubAccountConnectHref(pathname)} />}
            >
              <GithubIcon />
              Connect GitHub
            </Button>
          }
        />
      </>
    );
  }
  return (
    <SettingsSection
      title={title}
      description={description}
      actions={
        <>
          {!me?.githubAuthorized && (
            <Button
              size="sm"
              variant="outline"
              render={<a href={githubAccountConnectHref(pathname)} />}
            >
              <GithubIcon />
              Reconnect
            </Button>
          )}
          <ConfirmDialog
            trigger={
              <Button size="sm" variant="outline">
                Disconnect
              </Button>
            }
            title="Disconnect your GitHub account"
            description={`GitHub activity of @${login} stops showing as yours, and your comments post on GitHub as Ricrac. You can link it again any time.`}
            confirmLabel="Disconnect GitHub account"
            onConfirm={() => unlink()}
          />
        </>
      }
    >
      <div className="flex items-center gap-2 text-sm">
        <GithubIcon />
        <a
          href={`https://github.com/${login}`}
          target="_blank"
          rel="noreferrer noopener"
          className="font-medium outline-hidden focus-visible:underline has-hover:hover:underline"
        >
          @{login}
        </a>
      </div>
      {!me?.githubAuthorized && (
        <p className="text-muted-foreground text-xs">
          Reconnect so your comments post on GitHub as @{login} instead of Ricrac.
        </p>
      )}
    </SettingsSection>
  );
}
