import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"

import { initiativeQuery } from "@/api/queries"
import type { Initiative } from "@/api/v1"
import { LoadError, Loading, Time, Properties, Property } from "@/components/common"
import { Markdown } from "@/components/markdown"
import { DateValue, ProjectLink, ReadOnlyPage, Section, UserName } from "@/components/read-only"
import { StatusUpdateView } from "@/components/status-update"

export const Route = createFileRoute("/$workspace/initiative/$slug")({ component: InitiativePage })

function InitiativePage() {
  const { workspace, slug } = Route.useParams()
  const initiative = useQuery(initiativeQuery(workspace, slug))
  return (
    <ReadOnlyPage workspace={workspace}>
      {initiative.isPending ? (
        <Loading />
      ) : initiative.isError ? (
        <LoadError error={initiative.error} what="Initiative" />
      ) : (
        <InitiativeView workspace={workspace} initiative={initiative.data} />
      )}
    </ReadOnlyPage>
  )
}

function InitiativeView({ workspace, initiative }: { workspace: string; initiative: Initiative }) {
  const body = initiative.content || initiative.description
  const status = typeof initiative.status === "string" ? initiative.status : initiative.status?.name
  return (
    <div className="grid gap-10 lg:grid-cols-[1fr_16rem]">
      <article className="min-w-0 space-y-8">
        <header className="space-y-2">
          <p className="text-sm text-muted-foreground">Initiative</p>
          <h1 className="text-2xl font-semibold tracking-tight break-words">{initiative.name}</h1>
        </header>
        {body ? <Markdown>{body}</Markdown> : <p className="text-sm text-muted-foreground">No description.</p>}
        {initiative.projects && initiative.projects.length > 0 && (
          <Section title="Projects">
            <ul className="divide-y rounded-lg border">
              {initiative.projects.map((project) => (
                <li key={project.id} className="px-3 py-2 text-sm">
                  <ProjectLink workspace={workspace} project={project} />
                </li>
              ))}
            </ul>
          </Section>
        )}
        {initiative.latestUpdate && (
          <Section title="Latest update">
            <StatusUpdateView update={initiative.latestUpdate} />
          </Section>
        )}
      </article>
      <aside className="h-fit rounded-lg border p-4">
        <Properties>
          <Property label="Status">{status ?? null}</Property>
          <Property label="Owner">
            <UserName user={initiative.owner} />
          </Property>
          <Property label="Target date">
            <DateValue date={initiative.targetDate} />
          </Property>
          <Property label="Updated">{initiative.updatedAt ? <Time at={initiative.updatedAt} /> : null}</Property>
        </Properties>
      </aside>
    </div>
  )
}
