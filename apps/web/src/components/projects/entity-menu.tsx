import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  EllipsisIcon,
  HistoryIcon,
  Trash2Icon,
  UndoIcon,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type TProps = {
  /** project, initiative, document */
  noun: string;
  isArchived?: boolean;
  isTrashed: boolean;
  onArchive?: (archive: boolean) => void;
  onTrash: (trash: boolean) => void;
  onShowHistory?: () => void;
};

/** The ⋯ menu of a project, initiative or document page. */
export function EntityMenu({
  noun,
  isArchived,
  isTrashed,
  onArchive,
  onTrash,
  onShowHistory,
}: TProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="ghost" size="icon" aria-label={`More ${noun} actions`}>
            <EllipsisIcon className="size-4" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-52">
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
        <DropdownMenuItem
          className={isTrashed ? undefined : "text-destructive"}
          onClick={() => onTrash(!isTrashed)}
        >
          {isTrashed ? <UndoIcon className="size-4" /> : <Trash2Icon className="size-4" />}
          {isTrashed ? "Restore from trash" : `Move ${noun} to trash`}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
