import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"

import { projectQuery } from "@/api/queries"
import type { Project } from "@/api/v1"
import { LoadError, Loading, Time, Properties, Property } from "@/components/common"
import { Markdown } from "@/components/markdown"
import { DateValue, ReadOnlyPage, Section, UserName } from "@/components/read-only"
import { StatusUpdateView } from "@/components/status-update"
import { formatDate } from "@/lib/time"

export const Route = createFileRoute("/$workspace/project/$slug")({ component: ProjectPage })

function ProjectPage() {
  const { workspace, slug } = Route.useParams()
  const project = useQuery(projectQuery(workspace, slug))
  return (
    <ReadOnlyPage workspace={workspace}>
      {project.isPending ? (
        <Loading />
      ) : project.isError ? (
        <LoadError error={project.error} what="Project" />
      ) : (
        <ProjectView project={project.data} />
      )}
    </ReadOnlyPage>
  )
}

function ProjectView({ project }: { project: Project }) {
  const body = project.content || project.description
  const progress = project.progress == null ? null : Math.round(project.progress * (project.progress <= 1 ? 100 : 1))
  return (
    <div className="grid gap-10 lg:grid-cols-[1fr_16rem]">
      <article className="min-w-0 space-y-8">
        <header className="space-y-2">
          <p className="text-sm text-muted-foreground">Project</p>
          <h1 className="text-2xl font-semibold tracking-tight break-words">{project.name}</h1>
          {project.summary && <p className="text-muted-foreground">{project.summary}</p>}
        </header>
        {body ? <Markdown>{body}</Markdown> : <p className="text-sm text-muted-foreground">No description.</p>}
        {project.milestones && project.milestones.length > 0 && (
          <Section title="Milestones">
            <ul className="divide-y rounded-lg border">
              {project.milestones.map((milestone) => (
                <li key={milestone.id} className="flex items-center justify-between gap-3 px-3 py-2 text-sm">
                  <span className="min-w-0 truncate">{milestone.name}</span>
                  <span className="shrink-0 text-muted-foreground">
                    {milestone.targetDate ? formatDate(milestone.targetDate) : "No date"}
                  </span>
                </li>
              ))}
            </ul>
          </Section>
        )}
        {project.latestUpdate && (
          <Section title="Latest update">
            <StatusUpdateView update={project.latestUpdate} />
          </Section>
        )}
      </article>
      <aside className="h-fit space-y-6 rounded-lg border p-4">
        <Properties>
          <Property label="Status">
            {project.status ? (
              <span className="inline-flex items-center gap-1.5">
                <span className="size-2 rounded-full" style={{ backgroundColor: project.status.color }} aria-hidden />
                {project.status.name}
              </span>
            ) : null}
          </Property>
          <Property label="Lead">
            <UserName user={project.lead} />
          </Property>
          <Property label="Start date">
            <DateValue date={project.startDate} />
          </Property>
          <Property label="Target date">
            <DateValue date={project.targetDate} />
          </Property>
          <Property label="Progress">
            {progress == null ? null : (
              <div className="space-y-1">
                <div
                  className="h-1.5 overflow-hidden rounded-full bg-muted"
                  role="progressbar"
                  aria-valuenow={progress}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-label="Progress"
                >
                  <div className="h-full bg-primary" style={{ width: `${progress}%` }} />
                </div>
                <span className="text-xs text-muted-foreground">{progress}%</span>
              </div>
            )}
          </Property>
          <Property label="Updated">{project.updatedAt ? <Time at={project.updatedAt} /> : null}</Property>
        </Properties>
      </aside>
    </div>
  )
}
