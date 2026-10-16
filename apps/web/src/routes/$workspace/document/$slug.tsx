import { useQuery } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"

import { documentQuery } from "@/api/queries"
import { LoadError, Loading, Time } from "@/components/common"
import { Markdown } from "@/components/markdown"
import { ProjectLink, ReadOnlyPage, UserName } from "@/components/read-only"

export const Route = createFileRoute("/$workspace/document/$slug")({ component: DocumentPage })

function DocumentPage() {
  const { workspace, slug } = Route.useParams()
  const document = useQuery(documentQuery(workspace, slug))
  return (
    <ReadOnlyPage workspace={workspace}>
      {document.isPending ? (
        <Loading />
      ) : document.isError ? (
        <LoadError error={document.error} what="Document" />
      ) : (
        <article className="mx-auto max-w-3xl space-y-8">
          <header className="space-y-2">
            <h1 className="text-3xl font-semibold tracking-tight break-words">{document.data.title}</h1>
            <p className="flex flex-wrap gap-x-2 text-sm text-muted-foreground">
              {document.data.creator && (
                <span>
                  By <UserName user={document.data.creator} />
                </span>
              )}
              {document.data.project && (
                <span>
                  in <ProjectLink workspace={workspace} project={document.data.project} />
                </span>
              )}
              {document.data.updatedAt && (
                <span>
                  · updated <Time at={document.data.updatedAt} />
                </span>
              )}
            </p>
          </header>
          {document.data.content ? (
            <Markdown className="prose-base">{document.data.content}</Markdown>
          ) : (
            <p className="text-sm text-muted-foreground">This document is empty.</p>
          )}
        </article>
      )}
    </ReadOnlyPage>
  )
}
