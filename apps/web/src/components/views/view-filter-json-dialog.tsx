import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { type TView, useSaveViewMutation } from "@/lib/queries/views";

type TProps = {
  view: TView | null;
  onClose: () => void;
};

/** The view's filter in the filter language, for what the builder doesn't cover (text, dates, or). */
export function ViewFilterJsonDialog({ view, onClose }: TProps) {
  return (
    <Dialog open={view !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-xl max-w-full">
        <DialogHeader>
          <DialogTitle>Edit filter as JSON</DialogTitle>
          <DialogDescription>
            The filter language of the API: fields like title, priority, state, team, assignee,
            labels, dueDate, with comparators like eq, in, contains and lt, combined with and / or.
            The server checks it when you save.
          </DialogDescription>
        </DialogHeader>
        {view && <JsonForm key={view.id} view={view} onDone={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

function JsonForm({ view, onDone }: { view: TView; onDone: () => void }) {
  const { workspace } = useWorkspace();
  const save = useSaveViewMutation(workspace.id);
  const [text, setText] = useState(JSON.stringify(view.filter, null, 2));
  const [error, setError] = useState<string | null>(null);

  const submit = () => {
    let filter: unknown;
    try {
      filter = JSON.parse(text);
    } catch {
      setError("This isn't valid JSON.");
      return;
    }
    if (typeof filter !== "object" || filter === null || Array.isArray(filter)) {
      setError("The filter is a JSON object.");
      return;
    }
    setError(null);
    save.mutate({ id: view.id, filter }, { onSuccess: onDone });
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Textarea
          aria-label="Filter"
          value={text}
          onChange={(event) => setText(event.target.value)}
          rows={14}
          spellCheck={false}
          className="font-mono text-xs"
        />
        {error && (
          <p role="alert" className="text-destructive text-xs">
            {error}
          </p>
        )}
      </div>
      <DialogFooter>
        <DialogClose
          render={
            <Button variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <Button size="sm" isPending={save.isPending} onClick={submit}>
          Save filter
        </Button>
      </DialogFooter>
    </div>
  );
}
