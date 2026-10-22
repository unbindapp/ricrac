import { useTheme } from "next-themes";
import { lazy, Suspense, useState } from "react";

import type { StatusUpdateReaction } from "@/api/ops.gen";
import { SmilePlusIcon } from "@/components/icons/symbols.gen";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { SimpleTooltip } from "@/components/ui/tooltip";

const EmojiPicker = lazy(() => import("@/components/shared/emoji-picker"));

type TGroup = { emoji: string; names: string[]; reacted: boolean };

function groupReactions(reactions: StatusUpdateReaction[], meId: string | undefined) {
  const groups = new Map<string, TGroup>();
  for (const reaction of reactions) {
    const group = groups.get(reaction.emoji) ?? {
      emoji: reaction.emoji,
      names: [],
      reacted: false,
    };
    if (reaction.user) group.names.push(reaction.user.name);
    if (reaction.user?.id === meId) group.reacted = true;
    groups.set(reaction.emoji, group);
  }
  return [...groups.values()];
}

type TProps = {
  reactions: StatusUpdateReaction[];
  meId: string | undefined;
  disabled?: boolean;
  onToggle: (emoji: string, reacted: boolean) => void;
};

export function UpdateReactions({ reactions, meId, disabled, onToggle }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  const { resolvedTheme } = useTheme();
  const groups = groupReactions(reactions, meId);
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {groups.map((group) => (
        <SimpleTooltip key={group.emoji} content={group.names.join(", ")}>
          <Button
            variant="outline"
            size="sm"
            forceMinSize={false}
            disabled={disabled}
            aria-pressed={group.reacted}
            aria-label={`${group.emoji} ${group.names.length}`}
            data-active={group.reacted || undefined}
            className="data-active:border-primary/6-10 data-active:bg-primary/1-10 h-7 rounded-full px-2"
            onClick={() => onToggle(group.emoji, group.reacted)}
          >
            <span>{group.emoji}</span>
            <span className="text-xs tabular-nums">{group.names.length}</span>
          </Button>
        </SimpleTooltip>
      ))}
      {!disabled && (
        <Popover open={isOpen} onOpenChange={setIsOpen}>
          <PopoverTrigger
            render={
              <Button variant="ghost" size="icon" aria-label="Add reaction">
                <SmilePlusIcon className="text-muted-foreground size-4" />
              </Button>
            }
          />
          <PopoverContent align="start" className="w-80 p-2">
            <Suspense fallback={<Skeleton className="h-80 w-full" />}>
              <EmojiPicker
                theme={resolvedTheme === "light" ? "light" : "dark"}
                onSelect={(emoji) => {
                  setIsOpen(false);
                  onToggle(emoji, groups.find((group) => group.emoji === emoji)?.reacted ?? false);
                }}
              />
            </Suspense>
          </PopoverContent>
        </Popover>
      )}
    </div>
  );
}
