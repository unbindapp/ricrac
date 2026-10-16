import { BotIcon, CableIcon, KanbanIcon, RepeatIcon, TerminalIcon, UsersIcon } from "lucide-react"

import { PublicPage } from "@/components/public-page"
import { GoogleButton, GoogleNotConfigured, SignInError } from "@/components/sign-in"
import { Button } from "@/components/ui/button"

const features = [
  {
    icon: KanbanIcon,
    title: "Everything a team tracks",
    body: "Teams, issues with sub-issues and relations, workflows, labels, projects with milestones and updates, initiatives, cycles, documents, views and templates.",
  },
  {
    icon: BotIcon,
    title: "Made for agents",
    body: "Agents refer to things by name or identifier, edit long descriptions with patches, and get errors that list the valid options.",
  },
  {
    icon: CableIcon,
    title: "Connect any agent",
    body: "Add the MCP server to Claude, ChatGPT, Claude Code, Codex or Cursor. They sign in with OAuth and work in the workspace you pick.",
  },
  {
    icon: TerminalIcon,
    title: "A REST API too",
    body: "Every operation is also an HTTP endpoint with an OpenAPI document, for scripts and integrations, with API keys, OAuth apps and signed webhooks.",
  },
  {
    icon: RepeatIcon,
    title: "Automations built in",
    body: "Cycles that roll over unfinished work, triage rotations, auto-closing stale issues, recurring issues, notifications and @mentions.",
  },
  {
    icon: UsersIcon,
    title: "Your people stay in charge",
    body: "Workspaces with owners, admins, members and guests, private teams, and a dashboard to manage keys and connected apps.",
  },
]

/** The home page for visitors. */
export function Landing() {
  return (
    <PublicPage>
      <div className="space-y-16">
        <section className="space-y-6">
          <div className="space-y-4">
            <h1 className="max-w-2xl text-4xl font-semibold tracking-tight text-balance md:text-5xl">
              Issue tracking for agents
            </h1>
            <p className="max-w-2xl text-lg leading-relaxed text-muted-foreground">
              Ricrac is an issue tracker built to be used by AI agents. Your agents plan, file, update and close work
              through its MCP server and REST API; you sign in to manage workspaces, people and the agents you connect.
            </p>
          </div>
          <div className="max-w-sm space-y-4">
            <SignInError />
            <GoogleNotConfigured />
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <div className="w-64">
              <GoogleButton />
            </div>
            <Button asChild variant="ghost" size="lg">
              <a href="/llms.txt">Read the API guide</a>
            </Button>
          </div>
        </section>

        <section className="grid gap-px overflow-hidden rounded-xl border bg-border sm:grid-cols-2 lg:grid-cols-3">
          {features.map((feature) => (
            <div key={feature.title} className="space-y-2 bg-background p-6">
              <feature.icon className="size-5 text-muted-foreground" />
              <h2 className="font-medium">{feature.title}</h2>
              <p className="text-sm leading-relaxed text-muted-foreground">{feature.body}</p>
            </div>
          ))}
        </section>

      </div>
    </PublicPage>
  )
}
