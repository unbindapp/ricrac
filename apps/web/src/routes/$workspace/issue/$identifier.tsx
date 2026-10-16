import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { PaperclipIcon } from "lucide-react"

import { issueQuery } from "@/api/queries"
import type { Comment, Issue, IssueRef } from "@/api/v1"
import { LoadError, Loading, Time, Properties, Property } from "@/components/common"
import { Markdown } from "@/components/markdown"
import { DateValue, IssueLink, ProjectLink, ReadOnlyPage, Section, StateBadge, UserName } from "@/components/read-only"
import { Badge } from "@/components/ui/badge"
import { formatDate, formatDateTime } from "@/lib/time"

export const Route = createFileRoute("/$workspace/issue/$identifier")({ component: IssuePage })

function IssuePage() {
  const { workspace, identifier } = Route.useParams()
  const issue = useQuery(issueQuery(workspace, identifier))
  return (
    <ReadOnlyPage workspace={workspace}>
      {issue.isPending ? (
        <Loading />
      ) : issue.isError ? (
        <LoadError error={issue.error} what="Issue" />
      ) : (
        <IssueView workspace={workspace} issue={issue.data} />
      )}
    </ReadOnlyPage>
  )
}

function IssueView({ workspace, issue }: { workspace: string; issue: Issue }) {
  const relations = issue.relations
  const relationGroups: [string, IssueRef[]][] = relations
    ? [
        ["Blocks", relations.blocks],
        ["Blocked by", relations.blockedBy],
        ["Related", relations.related],
        ["Duplicate of", relations.duplicateOf ? [relations.duplicateOf] : []],
        ["Duplicates", relations.duplicates],
      ]
    : []
  const shownRelations = relationGroups.filter(([, issues]) => issues.length > 0)
  const cycle = issue.cycle
  return (
    <div className="grid gap-10 lg:grid-cols-[1fr_16rem]">
      <article className="min-w-0 space-y-8">
        <header className="space-y-3">
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <span className="font-mono">{issue.identifier}</span>
            <StateBadge state={issue.state} />
            {issue.parent && (
              <span className="flex min-w-0 items-center gap-1">
                · Sub-issue of <IssueLink workspace={workspace} issue={issue.parent} className="inline-flex" />
              </span>
            )}
          </div>
          <h1 className="text-2xl font-semibold tracking-tight break-words">{issue.title}</h1>
        </header>

        {issue.description ? (
          <Markdown>{issue.description}</Markdown>
        ) : (
          <p className="text-sm text-muted-foreground">No description.</p>
        )}

        {issue.children && issue.children.length > 0 && (
          <Section title={`Sub-issues (${issue.children.length})`}>
            <ul className="divide-y rounded-lg border">
              {issue.children.map((child) => (
                <li key={child.id} className="px-3 py-2">
                  <IssueLink workspace={workspace} issue={child} />
                </li>
              ))}
            </ul>
          </Section>
        )}

        {shownRelations.length > 0 && (
          <Section title="Relations">
            <div className="space-y-4">
              {shownRelations.map(([label, issues]) => (
                <div key={label} className="space-y-1">
                  <h3 className="text-xs font-medium text-muted-foreground">{label}</h3>
                  <ul className="divide-y rounded-lg border">
                    {issues.map((related) => (
                      <li key={related.id} className="px-3 py-2">
                        <IssueLink workspace={workspace} issue={related} />
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </Section>
        )}

        {issue.attachments && issue.attachments.length > 0 && (
          <Section title="Links">
            <ul className="space-y-2">
              {issue.attachments.map((attachment) => (
                <li key={attachment.id}>
                  <a
                    href={attachment.url}
                    target="_blank"
                    rel="noreferrer noopener"
                    className="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm hover:bg-muted"
                  >
                    <PaperclipIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                    <span className="min-w-0">
                      <span className="block truncate font-medium">{attachment.title}</span>
                      {attachment.subtitle && (
                        <span className="block truncate text-xs text-muted-foreground">{attachment.subtitle}</span>
                      )}
                    </span>
                  </a>
                </li>
              ))}
            </ul>
          </Section>
        )}

        <Section title="Comments">
          {issue.comments && issue.comments.length > 0 ? (
            <div className="space-y-4">
              {issue.comments.map((comment) => (
                <Thread key={comment.id} comment={comment} />
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">No comments.</p>
          )}
        </Section>
      </article>

      <aside className="h-fit space-y-6 rounded-lg border p-4 lg:sticky lg:top-6">
        <Properties>
          <Property label="Status">
            <StateBadge state={issue.state} />
          </Property>
          <Property label="Priority">{issue.priorityLabel}</Property>
          <Property label="Assignee">
            <UserName user={issue.assignee} empty="Unassigned" />
          </Property>
          <Property label="Labels">
            {issue.labels.length > 0 ? (
              <div className="flex flex-wrap gap-1">
                {issue.labels.map((label) => (
                  <Badge key={label.id} variant="outline">
                    <span className="size-2 rounded-full" style={{ backgroundColor: label.color }} aria-hidden />
                    {label.parent ? `${label.parent}/${label.name}` : label.name}
                  </Badge>
                ))}
              </div>
            ) : null}
          </Property>
          <Property label="Project">
            {issue.project ? <ProjectLink workspace={workspace} project={issue.project} /> : null}
          </Property>
          <Property label="Milestone">{issue.projectMilestone?.name ?? null}</Property>
          <Property label="Cycle">
            {cycle ? (
              <span title={`${formatDate(cycle.startsAt)} – ${formatDate(cycle.endsAt)}`}>
                {cycle.name || `Cycle ${cycle.number}`}
              </span>
            ) : null}
          </Property>
          <Property label="Estimate">{issue.estimate ?? null}</Property>
          <Property label="Due date">
            <DateValue date={issue.dueDate} />
          </Property>
          <Property label="Team">{issue.team.name}</Property>
          <Property label="Created by">
            <UserName user={issue.creator} />
          </Property>
          <Property label="Created">
            {<span title={formatDateTime(issue.createdAt)}>{formatDate(issue.createdAt)}</span>}
          </Property>
          <Property label="Updated">
            <Time at={issue.updatedAt} />
          </Property>
        </Properties>
      </aside>
    </div>
  )
}

function Thread({ comment }: { comment: Comment }) {
  const replies = comment.replies ?? comment.children ?? []
  return (
    <div className="rounded-lg border">
      <CommentView comment={comment} />
      {replies.length > 0 && (
        <div className="space-y-0 border-t bg-muted/30 pl-6">
          {replies.map((reply) => (
            <CommentView key={reply.id} comment={reply} />
          ))}
        </div>
      )}
    </div>
  )
}

function CommentView({ comment }: { comment: Comment }) {
  return (
    <div className="space-y-2 p-4">
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="font-medium">
          <UserName user={comment.user} empty="Someone" />
        </span>
        <span className="text-muted-foreground">
          <Time at={comment.createdAt} />
        </span>
        {comment.resolvedAt && <Badge variant="secondary">Resolved</Badge>}
      </div>
      <Markdown>{comment.body}</Markdown>
    </div>
  )
}
