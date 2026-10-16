import { useQuery } from "@tanstack/react-query"
import { Link, createFileRoute } from "@tanstack/react-router"

import { viewIssuesQuery, viewQuery } from "@/api/queries"
import { LoadError, Loading, Time } from "@/components/common"
import { ReadOnlyPage, StateBadge, UserName } from "@/components/read-only"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

export const Route = createFileRoute("/$workspace/view/$slug")({ component: ViewPage })

function ViewPage() {
  const { workspace, slug } = Route.useParams()
  const view = useQuery(viewQuery(workspace, slug))
  const issues = useQuery(viewIssuesQuery(workspace, slug))
  return (
    <ReadOnlyPage workspace={workspace}>
      {view.isPending ? (
        <Loading />
      ) : view.isError ? (
        <LoadError error={view.error} what="View" />
      ) : (
        <div className="space-y-6">
          <header className="space-y-2">
            <p className="text-sm text-muted-foreground">Custom view</p>
            <h1 className="text-2xl font-semibold tracking-tight break-words">{view.data.name}</h1>
            {view.data.description && <p className="text-muted-foreground">{view.data.description}</p>}
          </header>
          {issues.isPending ? (
            <Loading />
          ) : issues.isError ? (
            <LoadError error={issues.error} what="Issues" />
          ) : issues.data.nodes.length === 0 ? (
            <p className="rounded-lg border p-8 text-center text-sm text-muted-foreground">
              No issues match this view.
            </p>
          ) : (
            <div className="rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="pl-4">Issue</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Priority</TableHead>
                    <TableHead>Assignee</TableHead>
                    <TableHead className="pr-4">Updated</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {issues.data.nodes.map((issue) => (
                    <TableRow key={issue.id}>
                      <TableCell className="max-w-md pl-4">
                        <Link
                          to="/$workspace/issue/$identifier"
                          params={{ workspace, identifier: issue.identifier }}
                          className="flex min-w-0 gap-2 hover:underline"
                        >
                          <span className="shrink-0 font-mono text-xs text-muted-foreground">{issue.identifier}</span>
                          <span className="truncate">{issue.title}</span>
                        </Link>
                      </TableCell>
                      <TableCell>
                        <StateBadge state={issue.state} />
                      </TableCell>
                      <TableCell>{issue.priorityLabel}</TableCell>
                      <TableCell>
                        <UserName user={issue.assignee} empty="Unassigned" />
                      </TableCell>
                      <TableCell className="pr-4">
                        <Time at={issue.updatedAt} />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              {issues.data.pageInfo.hasNextPage && (
                <p className="border-t px-4 py-2 text-xs text-muted-foreground">Showing the first 100 issues.</p>
              )}
            </div>
          )}
        </div>
      )}
    </ReadOnlyPage>
  )
}
