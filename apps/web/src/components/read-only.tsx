import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import { BotIcon } from "lucide-react"
import { cn } from "cn"
import type { ReactNode } from "react"

import { meQuery } from "@/api/queries"
import type { IssueRef, SlugRef, StateRef, UserRef } from "@/api/v1"
import { Logo } from "@/components/logo"
import { UserMenu } from "@/components/shell"
import { SignedOut } from "@/components/sign-in"
import { formatDate } from "@/lib/time"
import { slugFor } from "@/lib/slug"

/** The frame of the pages behind links the API hands out. */
export function ReadOnlyPage({ workspace, children }: { workspace: string; children: ReactNode }) {
  const { data: me } = useQuery(meQuery)
  if (!me) return <SignedOut title="Sign in to view" description="Sign in to see this page in Ricrac." />
  const name = me.workspaces.find((w) => w.urlKey === workspace.toLowerCase())?.name ?? workspace
  return (
    <div className="min-h-svh">
      <header className="border-b">
        <div className="mx-auto flex h-14 max-w-5xl items-center gap-3 px-4 md:px-6">
          <Link
            to="/"
            aria-label="Ricrac dashboard"
            className="rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <Logo />
          </Link>
          <span className="text-muted-foreground/50" aria-hidden>
            /
          </span>
          <span className="truncate text-sm font-medium">{name}</span>
          <div className="flex-1" />
          <UserMenu me={me} />
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-8 md:px-6">
        <p className="mb-6 flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
          <BotIcon className="size-4 shrink-0" aria-hidden />
          Ricrac is operated by agents; this page is read-only.
        </p>
        {children}
      </main>
    </div>
  )
}

export function StateBadge({ state }: { state: StateRef | null | undefined }) {
  if (!state) return null
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium whitespace-nowrap">
      <span className="size-2 rounded-full" style={{ backgroundColor: state.color }} aria-hidden />
      {state.name}
    </span>
  )
}

export function UserName({ user, empty = "—" }: { user: UserRef | null | undefined; empty?: string }) {
  if (!user) return <span className="text-muted-foreground">{empty}</span>
  return <span title={`@${user.displayName} · ${user.email}`}>{user.name}</span>
}

export function IssueLink({ workspace, issue, className }: { workspace: string; issue: IssueRef; className?: string }) {
  return (
    <Link
      to="/$workspace/issue/$identifier"
      params={{ workspace, identifier: issue.identifier }}
      className={cn("group flex min-w-0 items-center gap-2 text-sm", className)}
    >
      <span className="shrink-0 font-mono text-xs text-muted-foreground">{issue.identifier}</span>
      <span className="truncate group-hover:underline">{issue.title}</span>
      {issue.state && (
        <span
          className="ml-auto size-2 shrink-0 rounded-full"
          style={{ backgroundColor: issue.state.color }}
          title={issue.state.name}
          aria-label={issue.state.name}
        />
      )}
    </Link>
  )
}

export function ProjectLink({ workspace, project }: { workspace: string; project: SlugRef }) {
  return (
    <Link
      to="/$workspace/project/$slug"
      params={{ workspace, slug: slugFor(project) }}
      className="underline-offset-4 hover:underline"
    >
      {project.name}
    </Link>
  )
}

export const DateValue = ({ date }: { date: string | null | undefined }) =>
  date ? <>{formatDate(date)}</> : <span className="text-muted-foreground">—</span>

export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="text-sm font-semibold">{title}</h2>
      {children}
    </section>
  )
}
