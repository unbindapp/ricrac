import { type ReactNode, useState } from "react";

import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  EllipsisIcon,
  HistoryIcon,
  Trash2Icon,
  UndoIcon,
} from "@/components/icons";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";

type TProps = {
  /** project, initiative, document */
  noun: string;
  isArchived?: boolean;
  onArchive?: (archive: boolean) => void;
  onTrash: () => void;
  onShowHistory?: () => void;
  /** Items of the page's own, first. */
  children?: ReactNode;
};

/** Trashed things can't be opened, so their page is left with a toast that brings them back. */
export function toastTrashed(title: string, onUndo: () => void) {
  const id = toast.add({
    type: "success",
    title,
    data: {
      action: (
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            toast.close(id);
            onUndo();
          }}
        >
          <UndoIcon className="size-4" />
          Undo
        </Button>
      ),
    },
  });
}

/** A row in the trash: it can't be opened, only restored. */
export function TrashedRow({
  icon,
  name,
  trashedAt,
  isPending,
  onRestore,
}: {
  icon: ReactNode;
  name: string;
  trashedAt: string | null;
  isPending?: boolean;
  onRestore?: () => void;
}) {
  return (
    <div
      role="listitem"
      className="text-muted-foreground flex h-9 w-full items-center gap-3 border-b px-3 text-sm group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0"
    >
      {icon}
      <span className="min-w-0 flex-1 truncate">{name}</span>
      {trashedAt && (
        <span className="shrink-0 text-xs">
          Trashed <RelativeTime value={trashedAt} />
        </span>
      )}
      {onRestore && (
        <Button variant="outline" size="sm" isPending={isPending} onClick={onRestore}>
          <UndoIcon className="size-4" />
          Restore
        </Button>
      )}
    </div>
  );
}

/** The ⋯ menu of a project, initiative or document page. */
export function EntityMenu({
  noun,
  isArchived,
  onArchive,
  onTrash,
  onShowHistory,
  children,
}: TProps) {
  const [isTrashing, setIsTrashing] = useState(false);
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="ghost" size="icon" aria-label={`More ${noun} actions`}>
              <EllipsisIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-52">
          {children}
          {onShowHistory && (
            <>
              <DropdownMenuItem onClick={onShowHistory}>
                <HistoryIcon className="size-4" />
                Version history
              </DropdownMenuItem>
              <DropdownMenuSeparator />
            </>
          )}
          {onArchive && (
            <DropdownMenuItem onClick={() => onArchive(!isArchived)}>
              {isArchived ? (
                <ArchiveRestoreIcon className="size-4" />
              ) : (
                <ArchiveIcon className="size-4" />
              )}
              {isArchived ? "Unarchive" : "Archive"}
            </DropdownMenuItem>
          )}
          <DropdownMenuItem className="text-destructive" onClick={() => setIsTrashing(true)}>
            <Trash2Icon className="size-4" />
            Move {noun} to trash
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        open={isTrashing}
        onOpenChange={setIsTrashing}
        title={`Move ${noun} to trash`}
        description="It stays in the trash for 30 days, then it is deleted for good."
        confirmLabel="Move to trash"
        onConfirm={async () => onTrash()}
      />
    </>
  );
}
