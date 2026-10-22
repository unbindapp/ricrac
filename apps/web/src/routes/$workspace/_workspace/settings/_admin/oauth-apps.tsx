import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

import type { OAuthApp, OAuthAppSecret } from "@/api/account.gen";
import {
  AppWindowIcon,
  KeyRoundIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
} from "@/components/icons/symbols.gen";
import { List, ListRow, ListRowText } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { OAuthAppDialog } from "@/components/oauth-apps/oauth-app-dialog";
import { OAuthAppSecretDialog } from "@/components/oauth-apps/oauth-app-secret-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { CopyButton } from "@/components/shared/copy-button";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  oauthAppsQuery,
  useDeleteOAuthAppMutation,
  useRotateOAuthSecretMutation,
} from "@/lib/queries/oauth-apps";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin/oauth-apps")({
  component: OAuthAppsSettingsPage,
});

function OAuthAppsSettingsPage() {
  const { workspace } = useWorkspace();
  const { data: apps = [], isPending, error, refetch } = useQuery(oauthAppsQuery(workspace.id));
  const [secret, setSecret] = useState<OAuthAppSecret | null>(null);
  const create = (
    <OAuthAppDialog
      onCreated={setSecret}
      trigger={
        <Button size="sm">
          <PlusIcon className="size-4" />
          Create app
        </Button>
      }
    />
  );
  return (
    <SettingsPage
      title="OAuth apps"
      description="Apps of your own that people in this workspace connect with OAuth, with a client ID and secret."
    >
      <SettingsSection title="OAuth apps" actions={create}>
        <ListStates
          isPending={isPending}
          error={error}
          onRetry={refetch}
          isEmpty={apps.length === 0}
          empty={{
            icon: AppWindowIcon,
            title: "No OAuth apps yet",
            description: "MCP clients don't need one: they register themselves when they connect.",
          }}
        >
          <List>
            {apps.map((app) => (
              <OAuthAppRow key={app.id} app={app} onSecret={setSecret} />
            ))}
          </List>
        </ListStates>
      </SettingsSection>
      <OAuthAppSecretDialog secret={secret} onClose={() => setSecret(null)} />
    </SettingsPage>
  );
}

function OAuthAppRow({
  app,
  onSecret,
}: {
  app: OAuthApp;
  onSecret: (secret: OAuthAppSecret) => void;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: remove } = useDeleteOAuthAppMutation(workspace.id);
  const { mutateAsync: rotate } = useRotateOAuthSecretMutation(workspace.id);
  const users = `${app.users} connected`;
  return (
    <ListRow
      actions={
        <>
          <span className="text-muted-foreground hidden text-xs md:inline">{users}</span>
          <CopyButton value={app.id} label={`Copy ${app.name} client ID`} variant="ghost" />
          <OAuthAppDialog
            app={app}
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Edit ${app.name}`}>
                <PencilIcon className="size-4" />
              </Button>
            }
          />
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`New secret for ${app.name}`}>
                <KeyRoundIcon className="size-4" />
              </Button>
            }
            variant="default"
            title="Make a new client secret"
            description="The current secret stops working right away; the app needs the new one to sign people in."
            confirmLabel="Make new secret"
            onConfirm={async () => onSecret(await rotate(app.id))}
          />
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Delete ${app.name}`}>
                <Trash2Icon className="size-4" />
              </Button>
            }
            title={`Delete ${app.name}`}
            description="Everyone who connected it loses access immediately."
            confirmLabel="Delete app"
            onConfirm={() => remove(app.id)}
          />
        </>
      }
    >
      <AppWindowIcon className="text-muted-foreground size-4" />
      <ListRowText primary={app.name} secondary={<span className="font-mono">{app.id}</span>} />
    </ListRow>
  );
}
