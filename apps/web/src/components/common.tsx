import { CheckIcon, CopyIcon, TriangleAlertIcon } from "lucide-react"
import { cn } from "cn"
import { useState, type ReactNode } from "react"

import { errorMessage, isMissingOp } from "@/api/client"
import { Logo } from "@/components/logo"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Spinner } from "@/components/ui/spinner"
import { formatDateTime, relativeTime } from "@/lib/time"

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
}) {
  return (
    <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0 flex-1 space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description && <p className="max-w-prose text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  )
}

async function copyText(value: string) {
  if (navigator.clipboard && window.isSecureContext) return navigator.clipboard.writeText(value)
  const area = document.createElement("textarea")
  area.value = value
  area.style.position = "fixed"
  area.style.opacity = "0"
  document.body.append(area)
  area.select()
  document.execCommand("copy")
  area.remove()
}

export function CopyButton({
  value,
  label = "Copy",
  className,
}: {
  value: string
  label?: string
  className?: string
}) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    await copyText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      onClick={copy}
      aria-label={copied ? "Copied" : label}
      title={label}
      className={cn("text-muted-foreground", className)}
    >
      {copied ? <CheckIcon /> : <CopyIcon />}
    </Button>
  )
}

/** A monospace value with a copy button, for URLs, client ids and secrets. */
export function CopyValue({ value, label, className }: { value: string; label?: string; className?: string }) {
  return (
    <div className={cn("flex h-9 min-w-0 items-center gap-1 rounded-md border bg-muted/40 pr-0.5 pl-3", className)}>
      <code className="min-w-0 flex-1 truncate font-mono text-sm" title={value}>
        {value}
      </code>
      <CopyButton value={value} label={label ? `Copy ${label}` : "Copy"} />
    </div>
  )
}

/** A block of code or commands with a copy button. */
export function CodeBlock({ code, className }: { code: string; className?: string }) {
  return (
    <div className={cn("relative rounded-md border bg-muted/40", className)}>
      <pre className="overflow-x-auto py-2.5 pr-11 pl-3 font-mono text-sm leading-relaxed whitespace-pre">{code}</pre>
      <CopyButton value={code} className="absolute top-1 right-1" />
    </div>
  )
}

/** An ISO timestamp as relative time, with the full date on hover. */
export function Time({ at, empty = "Never" }: { at: string | null | undefined; empty?: string }) {
  const [now] = useState(Date.now)
  if (!at) return <span className="text-muted-foreground">{empty}</span>
  return (
    <time dateTime={at} title={formatDateTime(at)} className="whitespace-nowrap">
      {relativeTime(at, now)}
    </time>
  )
}

export function Loading({ className }: { className?: string }) {
  return (
    <div
      className={cn("flex justify-center py-12 text-muted-foreground", className)}
      role="status"
      aria-label="Loading"
    >
      <Spinner className="size-5" />
    </div>
  )
}

/** A failed query, with a gentler message when the server doesn't have the operation yet. */
export function LoadError({ error, what }: { error: unknown; what: string }) {
  const missing = isMissingOp(error)
  return (
    <Empty className="border">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <TriangleAlertIcon />
        </EmptyMedia>
        <EmptyTitle>{missing ? `${what} isn't available yet` : `Couldn't load ${what.toLowerCase()}`}</EmptyTitle>
        <EmptyDescription>
          {missing ? "This server doesn't support it yet. Try again after it's updated." : errorMessage(error)}
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

/** Full-screen centered panel: sign in, onboarding, consent, errors. */
export function CenterPanel({ children, wide }: { children: ReactNode; wide?: boolean }) {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center bg-muted/30 p-4 md:p-8">
      <div className={cn("w-full space-y-6", wide ? "max-w-lg" : "max-w-sm")}>
        <div className="flex justify-center">
          <Logo />
        </div>
        {children}
      </div>
    </div>
  )
}

export function Scopes({ scopes }: { scopes: string[] }) {
  return (
    <div className="flex flex-wrap gap-1">
      {scopes.map((scope) => (
        <Badge key={scope} variant={scope === "admin" ? "default" : "secondary"}>
          {scope}
        </Badge>
      ))}
    </div>
  )
}

/** Label/value rows, like the sidebar of an issue. */
export function Properties({ children }: { children: ReactNode }) {
  return <dl className="grid grid-cols-[6.5rem_1fr] gap-x-3 gap-y-3 text-sm">{children}</dl>
}

export function Property({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0">{children ?? <span className="text-muted-foreground">—</span>}</dd>
    </>
  )
}
