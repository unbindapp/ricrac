import { useQuery } from "@tanstack/react-query"
import { Link } from "@tanstack/react-router"
import type { ReactNode } from "react"

import { meQuery } from "@/api/queries"
import { Logo } from "@/components/logo"
import { Button } from "@/components/ui/button"

export const PRIVACY_EMAIL = "privacy@ricrac.app"
export const TERMS_EMAIL = "terms@ricrac.app"

export function PublicFooter() {
  return (
    <footer className="flex items-center justify-center gap-5 text-sm text-muted-foreground">
      <Link to="/" className="hover:text-foreground">
        Home
      </Link>
      <Link to="/privacy" className="hover:text-foreground">
        Privacy
      </Link>
      <Link to="/terms" className="hover:text-foreground">
        Terms
      </Link>
    </footer>
  )
}

/** Header, a centered column and the footer: the frame of the home, privacy and terms pages. */
export function PublicPage({ children }: { children: ReactNode }) {
  const { data: me } = useQuery(meQuery)
  return (
    <div className="flex min-h-svh flex-col">
      <header className="border-b">
        <div className="mx-auto flex h-14 max-w-4xl items-center justify-between px-4 md:px-6">
          <Link to="/">
            <Logo />
          </Link>
          <Button asChild variant="ghost" size="sm">
            {me ? <Link to="/">Dashboard</Link> : <a href="/api/auth/sign-in?return_to=%2F">Sign in</a>}
          </Button>
        </div>
      </header>
      <main className="mx-auto w-full max-w-4xl flex-1 px-4 py-10 md:px-6 md:py-14">{children}</main>
      <div className="border-t py-6">
        <PublicFooter />
      </div>
    </div>
  )
}

/** Long-form text for the privacy policy and terms. */
export function Document({ title, updated, children }: { title: string; updated: string; children: ReactNode }) {
  return (
    <article className="mx-auto max-w-2xl space-y-8">
      <header className="space-y-1 border-b pb-6">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        <p className="text-sm text-muted-foreground">Last updated {updated}</p>
      </header>
      <div className="space-y-6 leading-relaxed text-muted-foreground [&_a]:text-foreground [&_a]:underline [&_a]:underline-offset-4 [&_h2]:mb-2 [&_h2]:text-lg [&_h2]:font-medium [&_h2]:text-foreground [&_li]:ml-5 [&_li]:list-disc [&_strong]:font-medium [&_strong]:text-foreground [&_ul]:space-y-1">
        {children}
      </div>
    </article>
  )
}
