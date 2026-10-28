import { useQuery } from "@tanstack/react-query";

import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { MarkdownView } from "@/components/shared/markdown-view";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatDateTime } from "@/lib/helpers/dates";
import { messageHistoryQuery, type TMessageHistoryTarget } from "@/lib/queries/discussions";

type TProps = {
  target: TMessageHistoryTarget;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/** Every version of a message or post, oldest first. */
export function MessageHistoryDialog({ target, open, onOpenChange }: TProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-xl max-w-full">
        <DialogHeader>
          <DialogTitle>Edit history</DialogTitle>
        </DialogHeader>
        {open && <Versions target={target} />}
      </DialogContent>
    </Dialog>
  );
}

function Versions({ target }: { target: TMessageHistoryTarget }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(messageHistoryQuery(workspace.id, target));
  if (isPending) return <ListSkeleton rows={3} />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  return (
    <ol aria-label="Versions" className="flex max-h-96 flex-col gap-3 overflow-y-auto">
      {data.versions.map((version, index) => (
        <li
          key={`${version.writtenAt}:${index}`}
          data-current={version.current || undefined}
          className="bg-muted/40 data-current:bg-primary/1-10 flex flex-col gap-1 rounded-lg p-3"
        >
          <span className="text-muted-foreground text-xs">
            {version.current ? "Current" : index === 0 ? "Original" : "Edited"} ·{" "}
            {formatDateTime(version.writtenAt)}
          </span>
          {version.body ? (
            <MarkdownView markdown={version.body} />
          ) : (
            <span className="text-muted-foreground text-sm italic">Empty</span>
          )}
        </li>
      ))}
    </ol>
  );
}
