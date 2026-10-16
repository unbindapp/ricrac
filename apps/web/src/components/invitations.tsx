import { useMutation, useQueryClient } from "@tanstack/react-query"
import { MailIcon } from "lucide-react"
import { toast } from "sonner"

import { api } from "@/api/client"
import { meQuery } from "@/api/queries"
import type { PendingInvitation } from "@/api/types/PendingInvitation"
import type { WorkspaceSummary } from "@/api/types/WorkspaceSummary"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { selectWorkspace } from "@/lib/workspace"

/** Pending invitations to workspaces, each with Accept and Decline. */
export function InvitationList({ invitations }: { invitations: PendingInvitation[] }) {
  return (
    <ul className="divide-y">
      {invitations.map((invitation) => (
        <InvitationRow key={invitation.id} invitation={invitation} />
      ))}
    </ul>
  )
}

function InvitationRow({ invitation }: { invitation: PendingInvitation }) {
  const queryClient = useQueryClient()
  const refresh = () => queryClient.invalidateQueries({ queryKey: meQuery.queryKey })
  const accept = useMutation({
    mutationFn: () => api<WorkspaceSummary>("POST", `/invitations/${invitation.id}/accept`),
    onSuccess: async (workspace) => {
      selectWorkspace(workspace.id)
      await refresh()
      toast.success(`You joined ${workspace.name}`)
    },
    onError: (error) => toast.error(error.message),
  })
  const decline = useMutation({
    mutationFn: () => api("DELETE", `/invitations/${invitation.id}`),
    onSuccess: refresh,
    onError: (error) => toast.error(error.message),
  })
  const busy = accept.isPending || decline.isPending
  return (
    <li className="flex flex-wrap items-center gap-3 py-3 first:pt-0 last:pb-0">
      <MailIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
      <div className="min-w-0 flex-1 text-sm">
        <span className="font-medium">{invitation.workspaceName}</span>
        <span className="text-muted-foreground">
          {" "}
          · {invitation.role}
          {invitation.inviter && `, invited by ${invitation.inviter}`}
        </span>
      </div>
      <div className="flex gap-2">
        <Button size="sm" variant="outline" disabled={busy} onClick={() => decline.mutate()}>
          {decline.isPending && <Spinner />}
          Decline
        </Button>
        <Button size="sm" disabled={busy} onClick={() => accept.mutate()}>
          {accept.isPending && <Spinner />}
          Accept
        </Button>
      </div>
    </li>
  )
}
