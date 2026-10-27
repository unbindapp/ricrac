import { type ReactNode, useEffect, useState } from "react";

import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  CircleXIcon,
  EllipsisIcon,
  LinkIcon,
  Trash2Icon,
  UndoIcon,
} from "@/components/icons";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useCopyToClipboard } from "@/lib/hooks/use-copy";

type THeaderActionProps = {
  label: string;
  onClick?: () => void;
  isPending?: boolean;
  children: ReactNode;
};

export function HeaderAction({ label, onClick, isPending, children }: THeaderActionProps) {
  return (
    <SimpleTooltip content={label}>
      <Button
        variant="ghost"
        size="icon"
        aria-label={label}
        isPending={isPending}
        onClick={onClick}
      >
        {children}
      </Button>
    </SimpleTooltip>
  );
}

export function CopyLinkAction({ url, label = "Copy link" }: { url: string; label?: string }) {
  const { copyToClipboard } = useCopyToClipboard();
  return (
    <HeaderAction
      label={label}
      onClick={() => {
        copyToClipboard(url);
        toast.add({ type: "success", title: "Link copied" });
      }}
    >
      <LinkIcon className="size-4" />
    </HeaderAction>
  );
}

type TToggleProps = {
  isOn: boolean;
  isPending?: boolean;
  onToggle: () => void;
};

export function ArchiveAction({ isOn, isPending, onToggle }: TToggleProps) {
  return (
    <HeaderAction label={isOn ? "Unarchive" : "Archive"} isPending={isPending} onClick={onToggle}>
      {isOn ? <ArchiveRestoreIcon className="size-4" /> : <ArchiveIcon className="size-4" />}
    </HeaderAction>
  );
}

export function TrashAction({ isOn, isPending, onToggle }: TToggleProps) {
  return (
    <HeaderAction
      label={isOn ? "Restore from trash" : "Move to trash"}
      isPending={isPending}
      onClick={onToggle}
    >
      {isOn ? <UndoIcon className="size-4" /> : <Trash2Icon className="size-4" />}
    </HeaderAction>
  );
}

export function DeleteMenuAction({ label, onSelect }: { label: string; onSelect: () => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="ghost" size="icon" aria-label="More actions">
            <EllipsisIcon className="size-4" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuItem className="text-destructive" onClick={onSelect}>
          <CircleXIcon className="size-4" />
          {label}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

const CONFIRM_WINDOW_MS = 3000;

/** Deletes on a second click within a few seconds, so it can't be hit by accident. */
export function ConfirmDeleteAction({ label, onDelete }: { label: string; onDelete: () => void }) {
  const [isConfirming, setIsConfirming] = useState(false);
  useEffect(() => {
    if (!isConfirming) return;
    const timer = setTimeout(() => setIsConfirming(false), CONFIRM_WINDOW_MS);
    return () => clearTimeout(timer);
  }, [isConfirming]);
  return (
    <SimpleTooltip content={isConfirming ? "Click again to delete" : label}>
      <Button
        variant="ghost"
        size="icon"
        aria-label={isConfirming ? `${label}: click again to confirm` : label}
        data-confirming={isConfirming || undefined}
        onClick={() => {
          if (!isConfirming) return setIsConfirming(true);
          setIsConfirming(false);
          onDelete();
        }}
      >
        <Trash2Icon className="size-4" />
      </Button>
    </SimpleTooltip>
  );
}

export function HeaderActions({ children }: { children: ReactNode }) {
  return <div className="flex shrink-0 items-center gap-1">{children}</div>;
}
