import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
  BotIcon,
  CableIcon,
  KanbanIcon,
  type LucideIcon,
  MailIcon,
  MonitorIcon,
  RepeatIcon,
  UsersIcon,
} from "lucide-react";

import { GoogleSignInButton, SignInNotConfigured } from "@/components/auth/sign-in";
import { Logo } from "@/components/icons/logo";
import { Button, LinkButton } from "@/components/ui/button";
import { appConfigQuery } from "@/lib/queries/me";

const features: { icon: LucideIcon; heading: string; body: string }[] = [
  {
    icon: KanbanIcon,
    heading: "Everything a team tracks",
    body: "Teams, issues with sub-issues and relations, workflows, labels, projects with milestones and updates, initiatives, cycles, documents, views and templates.",
  },
  {
    icon: BotIcon,
    heading: "Made for agents too",
    body: "Agents refer to things by name or identifier, edit long descriptions with patches, and get errors that list the valid options.",
  },
  {
    icon: CableIcon,
    heading: "MCP and REST",
    body: "Add the MCP server to Claude, ChatGPT, Codex or Cursor, or call the REST API with an API key. Every operation is both, with an OpenAPI document and signed webhooks.",
  },
  {
    icon: MonitorIcon,
    heading: "A web app for people",
    body: "The same operations behind a fast web app: boards and lists, an inbox, a command panel and keyboard shortcuts.",
  },
  {
    icon: RepeatIcon,
    heading: "Automations built in",
    body: "Cycles that roll over unfinished work, triage rotations, auto-closing stale issues, recurring issues, notifications and @mentions.",
  },
  {
    icon: UsersIcon,
    heading: "Your people stay in charge",
    body: "Workspaces with owners, admins, members and guests, private teams, and control over every key and connected app.",
  },
];

export function LandingPage() {
  const { data: config } = useQuery(appConfigQuery);
  return (
    <main className="flex min-h-svh w-full flex-col">
      <header className="flex w-full items-center justify-between px-6 py-4">
        <Link to="/" aria-label="Ricrac home">
          <Logo />
        </Link>
        <LinkButton to="/sign-in" variant="outline" size="sm">
          Sign in
        </LinkButton>
      </header>

      <section className="flex w-full flex-col items-center gap-6 px-6 pt-24 pb-16 text-center">
        <div className="flex max-w-3xl flex-col items-center gap-4">
          <h1 className="text-4xl font-semibold tracking-tight text-balance sm:text-6xl">
            The issue tracker for people and agents
          </h1>
          <p className="text-muted-foreground max-w-xl text-base text-balance">
            Your team works in the web app. Your agents work through MCP and the API. Same issues,
            same permissions.
          </p>
        </div>
        <div className="w-full max-w-sm">
          <SignInNotConfigured />
        </div>
        <div className="flex flex-wrap items-center justify-center gap-2">
          {config?.googleConfigured !== false && (
            <div className="w-64">
              <GoogleSignInButton />
            </div>
          )}
          {config?.emailConfigured && (
            <LinkButton to="/sign-in" variant="outline">
              <MailIcon className="size-4" />
              Continue with email
            </LinkButton>
          )}
          <Button variant="ghost" render={<a href="/llms.txt" />}>
            Read the API guide
          </Button>
        </div>
      </section>

      <section className="mx-auto grid w-full max-w-5xl gap-3 px-6 pb-16 sm:grid-cols-2 lg:grid-cols-3">
        {features.map((feature) => (
          <div key={feature.heading} className="bg-card flex flex-col gap-2 rounded-lg border p-4">
            <feature.icon className="text-muted-foreground size-5" />
            <h2 className="text-base font-semibold">{feature.heading}</h2>
            <p className="text-muted-foreground text-sm">{feature.body}</p>
          </div>
        ))}
      </section>

      <PublicFooter />
    </main>
  );
}

export function PublicFooter() {
  return (
    <footer className="text-muted-foreground mt-auto flex w-full flex-wrap items-center justify-center gap-4 px-6 py-6 text-xs">
      <Link to="/" className="underline-offset-4 hover:underline">
        Home
      </Link>
      <Link to="/privacy" className="underline-offset-4 hover:underline">
        Privacy
      </Link>
      <Link to="/terms" className="underline-offset-4 hover:underline">
        Terms
      </Link>
    </footer>
  );
}
