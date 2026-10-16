import { useMutation } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import {
  AppWindowIcon,
  BlocksIcon,
  BuildingIcon,
  CheckIcon,
  ChevronsUpDownIcon,
  KeyRoundIcon,
  LogOutIcon,
  PlugIcon,
  PlusIcon,
  UserIcon,
  UsersIcon,
} from "lucide-react"
import { useState, type ReactNode } from "react"
import { toast } from "sonner"

import { api } from "@/api/client"
import type { Me } from "@/api/types/Me"
import type { WorkspaceSummary } from "@/api/types/WorkspaceSummary"
import { CenterPanel } from "@/components/common"
import { CreateWorkspaceForm } from "@/components/create-workspace"
import { InvitationList } from "@/components/invitations"
import { Logo } from "@/components/logo"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Spinner } from "@/components/ui/spinner"
import { WorkspaceContext, selectWorkspace } from "@/lib/workspace"

const nav = [
  { to: "/", label: "Connect", icon: PlugIcon },
  { to: "/settings/api-keys", label: "API keys", icon: KeyRoundIcon },
  { to: "/settings/oauth-apps", label: "OAuth apps", icon: BlocksIcon },
  { to: "/settings/connected-apps", label: "Connected apps", icon: AppWindowIcon },
  { to: "/settings/members", label: "Members", icon: UsersIcon },
  { to: "/settings/workspace", label: "Workspace", icon: BuildingIcon },
  { to: "/settings/profile", label: "Profile", icon: UserIcon },
] as const

export function AppShell({ me, workspace, children }: { me: Me; workspace: WorkspaceSummary; children: ReactNode }) {
  const [creating, setCreating] = useState(false)
  return (
    <WorkspaceContext value={workspace}>
      <div className="min-h-svh">
        <header className="sticky top-0 z-20 border-b bg-background/90 backdrop-blur">
          <div className="mx-auto flex h-14 max-w-6xl items-center gap-3 px-4 md:px-6">
            <Link
              to="/"
              aria-label="Ricrac home"
              className="rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <Logo />
            </Link>
            <span className="text-muted-foreground/50" aria-hidden>
              /
            </span>
            <WorkspaceSwitcher me={me} current={workspace} onCreate={() => setCreating(true)} />
            <div className="flex-1" />
            <UserMenu me={me} />
          </div>
        </header>
        <div className="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-6 md:flex-row md:gap-10 md:px-6 md:py-10">
          <nav aria-label="Settings" className="-mx-4 shrink-0 overflow-x-auto px-4 md:mx-0 md:w-48 md:px-0">
            <ul className="flex gap-1 md:flex-col">
              {nav.map(({ to, label, icon: Icon }) => (
                <li key={to}>
                  <Link
                    to={to}
                    activeOptions={{ exact: true }}
                    className="flex items-center gap-2 rounded-md px-3 py-2 text-sm whitespace-nowrap text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 data-[status=active]:bg-muted data-[status=active]:font-medium data-[status=active]:text-foreground"
                  >
                    <Icon className="size-4" aria-hidden />
                    {label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
          <main className="min-w-0 flex-1 space-y-6">
            {me.invitations.length > 0 && (
              <Alert>
                <AlertTitle>
                  You have {me.invitations.length === 1 ? "an invitation" : `${me.invitations.length} invitations`}
                </AlertTitle>
                <AlertDescription className="mt-2 w-full">
                  <InvitationList invitations={me.invitations} />
                </AlertDescription>
              </Alert>
            )}
            <div>{children}</div>
          </main>
        </div>
      </div>
      <Dialog open={creating} onOpenChange={setCreating}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create a workspace</DialogTitle>
            <DialogDescription>A workspace holds teams, issues, projects and members.</DialogDescription>
          </DialogHeader>
          <CreateWorkspaceForm onCreated={() => setCreating(false)} onCancel={() => setCreating(false)} />
        </DialogContent>
      </Dialog>
    </WorkspaceContext>
  )
}

function WorkspaceSwitcher({ me, current, onCreate }: { me: Me; current: WorkspaceSummary; onCreate: () => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="max-w-56 gap-2 px-2" aria-label={`Workspace: ${current.name}`}>
          <WorkspaceBadge name={current.name} />
          <span className="truncate">{current.name}</span>
          <ChevronsUpDownIcon className="text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-60">
        <DropdownMenuLabel className="text-xs text-muted-foreground">Workspaces</DropdownMenuLabel>
        <DropdownMenuGroup>
          {me.workspaces.map((workspace) => (
            <DropdownMenuItem key={workspace.id} onSelect={() => selectWorkspace(workspace.id)}>
              <WorkspaceBadge name={workspace.name} />
              <span className="flex-1 truncate">{workspace.name}</span>
              {workspace.id === current.id && <CheckIcon aria-label="Current" />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onCreate}>
          <PlusIcon />
          Create workspace
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function WorkspaceBadge({ name }: { name: string }) {
  return (
    <span
      className="flex size-5 shrink-0 items-center justify-center rounded bg-primary text-[0.65rem] font-semibold text-primary-foreground!"
      aria-hidden
    >
      {name.slice(0, 1).toUpperCase()}
    </span>
  )
}

export function UserAvatar({ me, className }: { me: Pick<Me, "name" | "email" | "avatarUrl">; className?: string }) {
  return (
    <Avatar className={className}>
      {me.avatarUrl && <AvatarImage src={me.avatarUrl} alt="" referrerPolicy="no-referrer" />}
      <AvatarFallback>{(me.name || me.email).slice(0, 1).toUpperCase()}</AvatarFallback>
    </Avatar>
  )
}

function useSignOut() {
  return useMutation({
    mutationFn: () => api("POST", "/auth/sign-out"),
    onSuccess: () => window.location.reload(),
    onError: (error) => toast.error(error.message),
  })
}

export function UserMenu({ me }: { me: Me }) {
  const signOut = useSignOut()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="rounded-full" aria-label="Account">
          <UserAvatar me={me} className="size-8" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        <DropdownMenuLabel>
          <div className="truncate">{me.name}</div>
          <div className="truncate text-xs font-normal text-muted-foreground">{me.email}</div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/settings/profile">
            <UserIcon />
            Profile
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem
          onSelect={(event) => {
            event.preventDefault()
            signOut.mutate()
          }}
        >
          {signOut.isPending || signOut.isSuccess ? <Spinner /> : <LogOutIcon />}
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Signed in without a workspace: create one, or accept an invitation. */
export function Onboarding({ me }: { me: Me }) {
  return (
    <CenterPanel wide>
      {me.invitations.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>You're invited</CardTitle>
            <CardDescription>Join a workspace someone invited you to.</CardDescription>
          </CardHeader>
          <CardContent>
            <InvitationList invitations={me.invitations} />
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle>Create a workspace</CardTitle>
          <CardDescription>
            Welcome, {me.name || me.email}. A workspace holds your teams, issues and projects; agents work in it through
            MCP and the REST API.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <CreateWorkspaceForm />
        </CardContent>
      </Card>
      <SignedInAs me={me} />
    </CenterPanel>
  )
}

export function SignedInAs({ me }: { me: Me }) {
  const signOut = useSignOut()
  return (
    <p className="flex items-center justify-center gap-1 text-sm text-muted-foreground">
      Signed in as {me.email}.
      <Button
        variant="link"
        size="sm"
        className="h-auto px-1"
        disabled={signOut.isPending}
        onClick={() => signOut.mutate()}
      >
        Sign out
      </Button>
    </p>
  )
}
