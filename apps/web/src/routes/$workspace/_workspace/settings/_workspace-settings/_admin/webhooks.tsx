import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

import { PlusIcon, WebhookIcon } from "@/components/icons";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { Button } from "@/components/ui/button";
import { WebhookDialog } from "@/components/webhooks/webhook-dialog";
import { WebhookRow } from "@/components/webhooks/webhook-row";
import { WebhookSecretDialog } from "@/components/webhooks/webhook-secret-dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { type TWebhook, webhooksQuery } from "@/lib/queries/webhooks";

export const Route = createFileRoute(
  "/$workspace/_workspace/settings/_workspace-settings/_admin/webhooks",
)({
  component: WebhooksSettingsPage,
});

function WebhooksSettingsPage() {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(webhooksQuery(workspace.id));
  const [editing, setEditing] = useState<TWebhook | "new" | null>(null);
  const [secret, setSecret] = useState<{ value: string; isNew: boolean } | null>(null);
  const webhooks = data ?? [];
  const newButton = (
    <Button size="sm" onClick={() => setEditing("new")}>
      <PlusIcon className="size-4" />
      New webhook
    </Button>
  );

  return (
    <SettingsPage
      title="Webhooks"
      description="Send changes in this workspace to other services as they happen."
    >
      <SettingsSection
        title="Webhooks"
        description="Failed deliveries are retried after 1 minute, 5 minutes, 30 minutes, 2 hours and 6 hours. Deliveries are kept for 14 days."
        actions={webhooks.length > 0 && newButton}
      >
        <ListStates
          isPending={isPending}
          error={error}
          onRetry={() => void refetch()}
          isEmpty={webhooks.length === 0}
          empty={{
            icon: WebhookIcon,
            title: "No webhooks yet",
            description:
              "Add a URL to receive issues, comments, projects, cycles and more as JSON.",
            action: newButton,
          }}
        >
          <List>
            {webhooks.map((webhook) => (
              <WebhookRow
                key={webhook.id}
                webhook={webhook}
                onEdit={() => setEditing(webhook)}
                onSecret={(value) => setSecret({ value, isNew: false })}
              />
            ))}
          </List>
        </ListStates>
      </SettingsSection>
      <WebhookDialog
        target={editing}
        onClose={() => setEditing(null)}
        onCreated={(created) => created.secret && setSecret({ value: created.secret, isNew: true })}
      />
      <WebhookSecretDialog secret={secret} onClose={() => setSecret(null)} />
    </SettingsPage>
  );
}
