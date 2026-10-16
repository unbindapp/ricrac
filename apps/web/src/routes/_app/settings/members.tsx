import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { MailPlusIcon, UsersIcon } from "lucide-react"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { isMissingOp, op } from "@/api/client"
import { invitationsQuery, membersQuery, workspaceQuery } from "@/api/queries"
import type { Member, Role } from "@/api/v1"
import { LoadError, Loading, PageHeader, Time } from "@/components/common"
import { Confirm } from "@/components/confirm"
import { UserAvatar } from "@/components/shell"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Field, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { isAdmin, useWorkspace } from "@/lib/workspace"

export const Route = createFileRoute("/_app/settings/members")({ component: MembersPage })

const roles: { value: Role; label: string }[] = [
  { value: "owner", label: "Owner" },
  { value: "admin", label: "Admin" },
  { value: "member", label: "Member" },
  { value: "guest", label: "Guest" },
]

function MembersPage() {
  const workspace = useWorkspace()
  const admin = isAdmin(workspace.role)
  const members = useQuery(membersQuery(workspace.id))
  const queryClient = useQueryClient()
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["members", workspace.id] })

  const updateRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: Role }) => op("update_user", { id, role }, workspace.id),
    onSuccess: () => {
      toast.success("Role changed")
      return refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => op("remove_user", { id }, workspace.id),
    onSuccess: () => {
      toast.success("Member removed")
      return refresh()
    },
    onError: (error) => toast.error(error.message),
  })

  return (
    <>
      <PageHeader
        title="Members"
        description={`People in ${workspace.name}.`}
        actions={admin && !isMissingOp(members.error) && <InviteDialog />}
      />
      <div className="space-y-8">
        {members.isPending ? (
          <Loading />
        ) : members.isError ? (
          <LoadError error={members.error} what="Members" />
        ) : (
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">Name</TableHead>
                  <TableHead>Teams</TableHead>
                  <TableHead>Joined</TableHead>
                  <TableHead>Role</TableHead>
                  {admin && (
                    <TableHead className="pr-4">
                      <span className="sr-only">Actions</span>
                    </TableHead>
                  )}
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.data.nodes.map((member) => (
                  <TableRow key={member.id}>
                    <TableCell className="pl-4">
                      <MemberName member={member} />
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {member.teams.map((t) => t.key).join(", ") || "—"}
                    </TableCell>
                    <TableCell>
                      <Time at={member.joinedAt} />
                    </TableCell>
                    <TableCell>
                      {admin && !member.isMe ? (
                        <Select
                          value={member.role}
                          onValueChange={(role) => updateRole.mutate({ id: member.id, role: role as Role })}
                        >
                          <SelectTrigger size="sm" className="w-28" aria-label={`Role of ${member.name}`}>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {roles.map((role) => (
                              <SelectItem key={role.value} value={role.value}>
                                {role.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      ) : (
                        <span className="text-sm capitalize">{member.role}</span>
                      )}
                    </TableCell>
                    {admin && (
                      <TableCell className="pr-4 text-right">
                        {!member.isMe && (
                          <Confirm
                            trigger={
                              <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive">
                                Remove
                              </Button>
                            }
                            title={`Remove ${member.name}?`}
                            description={`They lose access to ${workspace.name}, and their API keys and connected apps here stop working. Their issues and comments stay.`}
                            action="Remove member"
                            onConfirm={() => remove.mutateAsync(member.id)}
                          />
                        )}
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
        {admin && !isMissingOp(members.error) && <PendingInvitations />}
      </div>
    </>
  )
}

function MemberName({ member }: { member: Member }) {
  return (
    <div className="flex items-center gap-3">
      <UserAvatar me={member} className="size-8" />
      <div className="min-w-0">
        <div className="flex items-center gap-2 font-medium">
          <span className="truncate">{member.name}</span>
          {member.isMe && <Badge variant="secondary">You</Badge>}
          {!member.active && <Badge variant="outline">Inactive</Badge>}
        </div>
        <div className="truncate text-xs text-muted-foreground">
          @{member.displayName} · {member.email}
        </div>
      </div>
    </div>
  )
}

function PendingInvitations() {
  const workspace = useWorkspace()
  const invitations = useQuery(invitationsQuery(workspace.id))
  const queryClient = useQueryClient()
  const cancel = useMutation({
    mutationFn: (id: string) => op("delete_invitation", { id }, workspace.id),
    onSuccess: () => {
      toast.success("Invitation canceled")
      return queryClient.invalidateQueries({ queryKey: ["invitations", workspace.id] })
    },
    onError: (error) => toast.error(error.message),
  })

  return (
    <section className="space-y-3" aria-labelledby="invitations-heading">
      <h2 id="invitations-heading" className="text-lg font-semibold">
        Pending invitations
      </h2>
      {invitations.isPending ? (
        <Loading />
      ) : invitations.isError ? (
        <LoadError error={invitations.error} what="Invitations" />
      ) : invitations.data.nodes.length === 0 ? (
        <Empty className="border p-8">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <UsersIcon />
            </EmptyMedia>
            <EmptyTitle>No pending invitations</EmptyTitle>
            <EmptyDescription>Invite people by email; they join when they sign in with Google.</EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-4">Email</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Teams</TableHead>
                <TableHead>Invited by</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead className="pr-4">
                  <span className="sr-only">Actions</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {invitations.data.nodes.map((invitation) => (
                <TableRow key={invitation.id}>
                  <TableCell className="pl-4 font-medium">{invitation.email}</TableCell>
                  <TableCell className="capitalize">{invitation.role}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {invitation.teams.map((t) => t.key).join(", ") || "—"}
                  </TableCell>
                  <TableCell>{invitation.inviter?.name ?? "—"}</TableCell>
                  <TableCell>
                    <Time at={invitation.expiresAt} />
                  </TableCell>
                  <TableCell className="pr-4 text-right">
                    <Confirm
                      trigger={
                        <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive">
                          Cancel
                        </Button>
                      }
                      title={`Cancel the invitation to ${invitation.email}?`}
                      description="They can no longer join with it."
                      action="Cancel invitation"
                      onConfirm={() => cancel.mutateAsync(invitation.id)}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </section>
  )
}

function InviteDialog() {
  const workspace = useWorkspace()
  const details = useQuery(workspaceQuery(workspace.id))
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [email, setEmail] = useState("")
  const [role, setRole] = useState<Role>("member")
  const [teams, setTeams] = useState<string[]>([])

  const invite = useMutation({
    mutationFn: () =>
      op("invite_user", { email: email.trim(), role, teams: teams.length ? teams : undefined }, workspace.id),
    onSuccess: () => {
      toast.success(`Invited ${email.trim()}`)
      setOpen(false)
      setEmail("")
      setTeams([])
      return queryClient.invalidateQueries({ queryKey: ["invitations", workspace.id] })
    },
    onError: (error) => toast.error(error.message),
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    invite.mutate()
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <MailPlusIcon />
          Invite
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={submit} className="space-y-6">
          <DialogHeader>
            <DialogTitle>Invite to {workspace.name}</DialogTitle>
            <DialogDescription>They join when they sign in with Google using this email address.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="invite-email">Email</FieldLabel>
              <Input
                id="invite-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="ann@example.com"
                required
                autoFocus
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="invite-role">Role</FieldLabel>
              <Select value={role} onValueChange={(value) => setRole(value as Role)}>
                <SelectTrigger id="invite-role" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {roles.map((r) => (
                    <SelectItem key={r.value} value={r.value}>
                      {r.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            {details.data && details.data.teams.length > 0 && (
              <FieldSet>
                <FieldLegend variant="label">Teams</FieldLegend>
                <div className="grid gap-2 sm:grid-cols-2">
                  {details.data.teams.map((team) => (
                    <Field key={team.id} orientation="horizontal">
                      <Checkbox
                        id={`invite-team-${team.id}`}
                        checked={teams.includes(team.key)}
                        onCheckedChange={(checked) =>
                          setTeams((current) =>
                            checked === true ? [...current, team.key] : current.filter((k) => k !== team.key),
                          )
                        }
                      />
                      <FieldLabel htmlFor={`invite-team-${team.id}`} className="font-normal">
                        {team.name} <span className="font-mono text-xs text-muted-foreground">{team.key}</span>
                      </FieldLabel>
                    </Field>
                  ))}
                </div>
              </FieldSet>
            )}
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={invite.isPending || !email.trim()}>
              {invite.isPending && <Spinner />}
              Send invitation
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
