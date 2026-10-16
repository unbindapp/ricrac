import type { StatusUpdate } from "@/api/v1"
import { Time } from "@/components/common"
import { Markdown } from "@/components/markdown"
import { UserName } from "@/components/read-only"
import { Badge } from "@/components/ui/badge"

const healthWords: Record<string, string> = { onTrack: "On track", atRisk: "At risk", offTrack: "Off track" }

export function StatusUpdateView({ update }: { update: StatusUpdate }) {
  return (
    <div className="space-y-2 rounded-lg border p-4">
      <div className="flex flex-wrap items-center gap-2 text-sm">
        {update.health && <Badge variant="secondary">{healthWords[update.health] ?? update.health}</Badge>}
        <span className="font-medium">
          <UserName user={update.user} empty="Someone" />
        </span>
        <span className="text-muted-foreground">
          <Time at={update.createdAt} />
        </span>
      </div>
      <Markdown>{update.body}</Markdown>
    </div>
  )
}
