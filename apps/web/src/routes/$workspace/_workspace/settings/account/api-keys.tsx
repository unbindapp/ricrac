import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { KeySquareIcon } from "lucide-react";

import { ApiKeyRow } from "@/components/api-key/api-key-row";
import { CreateApiKeyDialog } from "@/components/api-key/create-api-key-dialog";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { apiKeysQuery } from "@/lib/queries/api-keys";

export const Route = createFileRoute("/$workspace/_workspace/settings/account/api-keys")({
  component: AccountApiKeysSettingsPage,
});

function AccountApiKeysSettingsPage() {
  const { data: apiKeys = [], isPending, error, refetch } = useQuery(apiKeysQuery);
  const create = <CreateApiKeyDialog takenLabels={apiKeys.map((key) => key.label)} />;
  return (
    <SettingsPage title="API keys" description="Keys for scripts, CI and agents.">
      <SettingsSection
        title="API keys"
        description={
          <>
            Send one as a bearer token to the REST API at{" "}
            <span className="font-mono text-xs">{window.location.origin}/v1</span> or the MCP
            server.
          </>
        }
        actions={create}
      >
        <ListStates
          isPending={isPending}
          error={error}
          onRetry={refetch}
          isEmpty={apiKeys.length === 0}
          empty={{ icon: KeySquareIcon, title: "No API keys yet" }}
        >
          <List>
            {apiKeys.map((apiKey) => (
              <ApiKeyRow key={apiKey.id} apiKey={apiKey} />
            ))}
          </List>
        </ListStates>
      </SettingsSection>
    </SettingsPage>
  );
}
