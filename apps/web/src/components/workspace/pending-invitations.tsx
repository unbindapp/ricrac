import { MailIcon } from "lucide-react";

import type { PendingInvitation, WorkspaceSummary } from "@/api/account.gen";
import { List, ListRow, ListRowText } from "@/components/layout/list";
import { Button } from "@/components/ui/button";
import {
  useAcceptInvitationMutation,
  useDeclineInvitationMutation,
  workspaceRoleLabels,
  WORKSPACE_ROLES,
} from "@/lib/queries/workspaces";

type TProps = {
  invitations: PendingInvitation[];
  onAccepted: (workspace: WorkspaceSummary) => void;
};

/** Invitations to workspaces sent to your email, each with Accept and Decline. */
export function PendingInvitations({ invitations, onAccepted }: TProps) {
  return (
    <List>
      {invitations.map((invitation) => (
        <InvitationRow key={invitation.id} invitation={invitation} onAccepted={onAccepted} />
      ))}
    </List>
  );
}

function roleLabel(role: string) {
  const known = WORKSPACE_ROLES.find((item) => item === role);
  return known ? workspaceRoleLabels[known] : role;
}

function InvitationRow({
  invitation,
  onAccepted,
}: {
  invitation: PendingInvitation;
  onAccepted: TProps["onAccepted"];
}) {
  const accept = useAcceptInvitationMutation();
  const decline = useDeclineInvitationMutation();
  const isBusy = accept.isPending || decline.isPending;
  const details = [
    roleLabel(invitation.role),
    invitation.inviter && `invited by ${invitation.inviter}`,
  ];
  return (
    <ListRow
      actions={
        <>
          <Button
            variant="ghost"
            size="sm"
            disabled={isBusy}
            isPending={decline.isPending}
            onClick={() => decline.mutate(invitation.id)}
          >
            Decline
          </Button>
          <Button
            size="sm"
            disabled={isBusy}
            isPending={accept.isPending}
            onClick={() => accept.mutate(invitation.id, { onSuccess: onAccepted })}
          >
            Join
          </Button>
        </>
      }
    >
      <MailIcon className="text-muted-foreground size-4" />
      <ListRowText
        primary={invitation.workspaceName}
        secondary={details.filter(Boolean).join(" · ")}
      />
    </ListRow>
  );
}
