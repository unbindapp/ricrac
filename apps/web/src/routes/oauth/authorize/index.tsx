import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { CheckIcon, CircleAlertIcon } from "lucide-react";
import { useState } from "react";
import { z } from "zod";

import type { Me, OAuthClientInfo, OAuthDecision } from "@/api/account.gen";
import { WorkspaceAccessList } from "@/components/api-key/access-fields";
import {
  accessFormShape,
  accessItems,
  hasPickedWorkspace,
  pickWorkspaceMessage,
  type TAccess,
} from "@/components/api-key/helpers";
import { AuthShell } from "@/components/layout/auth-shell";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CreateWorkspaceForm } from "@/components/workspace/create-workspace-form";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { scopeDescriptions, type TScope } from "@/lib/queries/api-keys";
import { oauthClientQuery, useOAuthDecisionMutation } from "@/lib/queries/connected-apps";
import { meQuery } from "@/lib/queries/me";

/** The consent page MCP clients and OAuth apps send people to. */
export const Route = createFileRoute("/oauth/authorize/")({
  component: AuthorizePage,
});

function authorizationRequest(query: URLSearchParams): OAuthDecision["request"] {
  return {
    client_id: query.get("client_id") ?? "",
    redirect_uri: query.get("redirect_uri") ?? "",
    response_type: query.get("response_type") ?? "",
    code_challenge: query.get("code_challenge"),
    code_challenge_method: query.get("code_challenge_method"),
    state: query.get("state"),
    scope: query.get("scope"),
    resource: query.get("resource"),
  };
}

function AuthorizePage() {
  const query = window.location.search.replace(/^\?/, "");
  const { data: me } = useQuery(meQuery);
  const client = useQuery(oauthClientQuery(query));

  if (client.isPending || !me) {
    return (
      <AuthShell title="Connect an app" width="wide">
        <ListSkeleton rows={3} />
      </AuthShell>
    );
  }
  if (client.isError) {
    return (
      <AuthShell title="Can't connect this app" width="wide" footer={<SignedInAs me={me} />}>
        <ErrorCard title={client.error.message} />
      </AuthShell>
    );
  }
  return (
    <Consent
      me={me}
      client={client.data}
      request={authorizationRequest(new URLSearchParams(query))}
    />
  );
}

function SignedInAs({ me }: { me: Me }) {
  return (
    <p>
      Signed in as <span className="text-foreground font-medium">{me.email}</span>
    </p>
  );
}

const formSchema = z.object(accessFormShape).refine(hasPickedWorkspace, pickWorkspaceMessage);

function Consent({
  me,
  client,
  request,
}: {
  me: Me;
  client: OAuthClientInfo;
  request: OAuthDecision["request"];
}) {
  const [isLeaving, setIsLeaving] = useState(false);
  const decide = useOAuthDecisionMutation();
  const send = async (allow: boolean, workspaceIds?: string[]) => {
    const { redirect } = await decide.mutateAsync({ request, allow, workspaceIds });
    setIsLeaving(true);
    window.location.assign(redirect);
  };
  const isBusy = decide.isPending || isLeaving;

  const form = useAppForm({
    defaultValues: { access: "all" as TAccess, workspaceIds: [] as string[] },
    validators: { onSubmit: formSchema },
    onSubmit: ({ value }) =>
      send(true, value.access === "all" ? undefined : value.workspaceIds).catch(() => undefined),
  });

  const title = `${client.client_name} wants to use your Ricrac workspaces`;
  const description = `After you decide, you go back to ${client.redirect_host}.`;

  if (me.workspaces.length === 0) {
    return (
      <AuthShell title="Create a workspace first" width="wide" footer={<SignedInAs me={me} />}>
        <p className="text-muted-foreground text-sm">
          {client.client_name} works in your workspaces. Create one, then allow it.
        </p>
        <CreateWorkspaceForm onCreated={() => undefined} />
      </AuthShell>
    );
  }

  return (
    <AuthShell title={title} description={description} width="wide" footer={<SignedInAs me={me} />}>
      <form
        className="bg-card flex w-full flex-col gap-4 rounded-lg border p-4"
        onSubmit={submitHandler(form)}
        noValidate
      >
        {!client.verified && (
          <div className="bg-warning/3-10 text-warning flex items-start gap-1.5 rounded-md px-3 py-2 text-sm">
            <CircleAlertIcon className="mt-0.5 size-4 shrink-0" />
            <p>
              This app registered itself, so Ricrac can't vouch for its name. Only allow it if you
              started connecting it just now.
            </p>
          </div>
        )}
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <p className="flex-1 text-sm font-medium">It will be able to</p>
            {client.verified && <Badge variant="success">Verified</Badge>}
          </div>
          <ul className="flex flex-col gap-1.5">
            {client.scopes.map((scope) => (
              <li key={scope} className="flex items-start gap-2 text-sm">
                <CheckIcon className="text-muted-foreground mt-0.5 size-4 shrink-0" />
                <span>{scopeDescriptions[scope as TScope] ?? scope}</span>
              </li>
            ))}
          </ul>
          <p className="text-muted-foreground text-xs">
            It acts as you, never with more than your permissions.
          </p>
        </div>
        <form.AppField
          name="access"
          children={(field) => <field.SelectField label="Workspaces" items={accessItems} />}
        />
        <form.Subscribe selector={(state) => state.values.access}>
          {(access) => (
            <form.Field
              name="workspaceIds"
              children={(field) => <WorkspaceAccessList field={field} access={access} />}
            />
          )}
        </form.Subscribe>
        <div className="flex items-center justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={isBusy}
            onClick={() => void send(false).catch(() => undefined)}
          >
            Deny
          </Button>
          <form.AppForm>
            <form.SubmitButton disabled={isBusy}>Allow</form.SubmitButton>
          </form.AppForm>
        </div>
      </form>
    </AuthShell>
  );
}
