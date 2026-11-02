import { useState } from "react";
import { z } from "zod";

import { ArrowDownIcon, ArrowUpIcon, PlusIcon, XIcon } from "@/components/icons";
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
import { Input } from "@/components/ui/input";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { type TBoard, useSaveBoardMutation } from "@/lib/queries/boards";

const STARTING_COLUMNS = ["Ideas", "Backlog", "Doing", "Done"];

const boardFormSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(80, "Keep the name under 80 characters."),
  description: z.string().trim().max(500, "Keep the description under 500 characters."),
});

type TColumnDraft = { key: string; id?: string; name: string };

export type TBoardDialogTarget = { mode: "create" } | { mode: "edit"; board: TBoard };

type TProps = {
  target: TBoardDialogTarget | null;
  onClose: () => void;
  onSaved?: (board: TBoard) => void;
};

export function BoardDialog({ target, onClose, onSaved }: TProps) {
  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{target?.mode === "edit" ? "Edit board" : "New board"}</DialogTitle>
          {target?.mode === "create" && (
            <DialogDescription>
              A kanban board with columns of your own. Cards are issues; their status stays as it
              is.
            </DialogDescription>
          )}
        </DialogHeader>
        {target && (
          <BoardForm
            board={target.mode === "edit" ? target.board : null}
            onDone={(board) => {
              onClose();
              onSaved?.(board);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function BoardForm({ board, onDone }: { board: TBoard | null; onDone: (board: TBoard) => void }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveBoard } = useSaveBoardMutation(workspace.id);
  const [columns, setColumns] = useState<TColumnDraft[]>(() =>
    board
      ? board.columns.map((column) => ({ key: column.id, id: column.id, name: column.name }))
      : STARTING_COLUMNS.map((name) => ({ key: crypto.randomUUID(), name })),
  );
  const [columnsError, setColumnsError] = useState<string>();

  const form = useAppForm({
    defaultValues: { name: board?.name ?? "", description: board?.description ?? "" },
    validators: { onSubmit: boardFormSchema },
    onSubmit: async ({ value }) => {
      const parsed = boardFormSchema.parse(value);
      const named = columns
        .map((column) => ({ ...column, name: column.name.trim() }))
        .filter((column) => column.name);
      if (named.length === 0) {
        setColumnsError("Add at least one column.");
        return;
      }
      setColumnsError(undefined);
      onDone(
        await saveBoard({
          id: board?.id,
          name: parsed.name,
          description: parsed.description || null,
          columns: named.map(({ id, name }) => ({ id, name })),
        }),
      );
    },
  });

  const update = (key: string, name: string) =>
    setColumns((current) =>
      current.map((column) => (column.key === key ? { ...column, name } : column)),
    );
  const move = (index: number, by: number) =>
    setColumns((current) => {
      const next = [...current];
      const [column] = next.splice(index, 1);
      next.splice(index + by, 0, column);
      return next;
    });
  const remove = (key: string) =>
    setColumns((current) => current.filter((column) => column.key !== key));

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="name"
        children={(field) => (
          <field.TextField label="Name" autoFocus autoComplete="off" placeholder="Ideas" />
        )}
      />
      <form.AppField
        name="description"
        children={(field) => (
          <field.TextareaField label="Description" rows={2} placeholder="Optional" />
        )}
      />
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1.5 text-sm font-medium">Columns</legend>
        {columns.map((column, index) => (
          <div key={column.key} className="flex items-center gap-2">
            <Input
              aria-label={`Column ${index + 1}`}
              value={column.name}
              placeholder="Column name"
              onChange={(event) => update(column.key, event.target.value)}
            />
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`Move ${column.name || "column"} up`}
              disabled={index === 0}
              onClick={() => move(index, -1)}
            >
              <ArrowUpIcon className="size-4" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`Move ${column.name || "column"} down`}
              disabled={index === columns.length - 1}
              onClick={() => move(index, 1)}
            >
              <ArrowDownIcon className="size-4" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`Remove ${column.name || "column"}`}
              onClick={() => remove(column.key)}
            >
              <XIcon className="size-4" />
            </Button>
          </div>
        ))}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="self-start"
          onClick={() =>
            setColumns((current) => [...current, { key: crypto.randomUUID(), name: "" }])
          }
        >
          <PlusIcon className="size-4" />
          Add column
        </Button>
        {columnsError && <p className="text-destructive text-sm">{columnsError}</p>}
        {board && (
          <p className="text-muted-foreground text-xs">
            Removing a column takes its cards off the board. The issues stay.
          </p>
        )}
      </fieldset>
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm">{board ? "Save" : "Create board"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}
