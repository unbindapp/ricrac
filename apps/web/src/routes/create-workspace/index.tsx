import { useQuery } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";

import type { WorkspaceSummary } from "@/api/account.gen";
import { AuthShell } from "@/components/layout/auth-shell";
import { Button } from "@/components/ui/button";
import { CreateWorkspaceForm } from "@/components/workspace/create-workspace-form";
import { PendingInvitations } from "@/components/workspace/pending-invitations";
import { meQuery, signOut } from "@/lib/queries/me";

export const Route = createFileRoute("/create-workspace/")({
  component: CreateWorkspace,
});

function CreateWorkspace() {
  const navigate = useNavigate();
  const { data: me } = useQuery(meQuery);
  const invitations = me?.invitations ?? [];
  const hasWorkspaces = (me?.workspaces.length ?? 0) > 0;
  const open = (workspace: WorkspaceSummary) =>
    navigate({ to: "/$workspace", params: { workspace: workspace.urlKey } });

  return (
    <AuthShell
      title={invitations.length > 0 ? "Join or create a workspace" : "Create a workspace"}
      description={
        hasWorkspaces
          ? "Workspaces hold teams, projects and issues."
          : "You aren't in a workspace yet. Create one, or ask someone to invite you."
      }
      footer={
        <>
          {me && <p>Signed in as {me.email}</p>}
          <Button variant="ghost" size="sm" onClick={() => void signOut()}>
            Sign out
          </Button>
        </>
      }
    >
      {invitations.length > 0 && (
        <section className="flex w-full flex-col gap-2">
          <h2 className="text-base font-semibold">Invitations</h2>
          <PendingInvitations invitations={invitations} onAccepted={open} />
        </section>
      )}
      <section className="flex w-full flex-col gap-2">
        {invitations.length > 0 && <h2 className="text-base font-semibold">New workspace</h2>}
        <CreateWorkspaceForm onCreated={open} />
      </section>
    </AuthShell>
  );
}
