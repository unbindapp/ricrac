import { useTheme } from "next-themes";
import { lazy, Suspense, useState } from "react";

import type { UserRef } from "@/api/ops.gen";
import { SmilePlusIcon } from "@/components/icons";
import { userName } from "@/components/issues/grouping";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { SimpleTooltip } from "@/components/ui/tooltip";

const EmojiPicker = lazy(() => import("@/components/shared/emoji-picker"));

export type TReactionGroup = {
  emoji: string;
  count: number;
  reacted: boolean;
  users: UserRef[];
};

/** Reactions as the server lists them (one per person and emoji), grouped by emoji. */
export function groupReactions(
  reactions: { emoji: string; user: UserRef | null }[],
  meId: string | undefined,
): TReactionGroup[] {
  const groups = new Map<string, TReactionGroup>();
  for (const reaction of reactions) {
    const group = groups.get(reaction.emoji) ?? {
      emoji: reaction.emoji,
      count: 0,
      reacted: false,
      users: [],
    };
    group.count += 1;
    if (reaction.user) group.users.push(reaction.user);
    if (reaction.user && reaction.user.id === meId) group.reacted = true;
    groups.set(reaction.emoji, group);
  }
  return [...groups.values()];
}

type TProps = {
  reactions: TReactionGroup[];
  onToggle: (emoji: string, reacted: boolean) => void;
  disabled?: boolean;
  /** The add button shows only while a surrounding group/reactable is hovered. */
  revealOnHover?: boolean;
};

function reactorNames(reaction: TReactionGroup) {
  return reaction.users.map(userName).join(", ");
}

export function ReactionBar({ reactions, onToggle, disabled, revealOnHover }: TProps) {
  if (disabled && reactions.length === 0) return null;
  return (
    <div className="dank:gap-2 flex flex-wrap items-center gap-1.5">
      {reactions.map((reaction) => (
        <SimpleTooltip key={reaction.emoji} content={reactorNames(reaction)}>
          <Button
            variant="outline"
            size="sm"
            forceMinSize={false}
            disabled={disabled}
            aria-pressed={reaction.reacted}
            aria-label={`${reaction.emoji} ${reaction.count}`}
            data-active={reaction.reacted || undefined}
            data-m3-chip="filter"
            className="data-active:border-primary/6-10 data-active:bg-primary/1-10 rounded-full"
            onClick={() => onToggle(reaction.emoji, reaction.reacted)}
          >
            <span>{reaction.emoji}</span>
            <span className="dank:type-label-large text-xs">{reaction.count}</span>
          </Button>
        </SimpleTooltip>
      ))}
      {!disabled && (
        <AddReactionButton
          reactions={reactions}
          onToggle={onToggle}
          revealOnHover={revealOnHover}
        />
      )}
    </div>
  );
}

/** The emoji picker button; with revealOnHover it shows while a group/reactable is hovered. */
export function AddReactionButton({
  reactions,
  onToggle,
  revealOnHover,
}: Omit<TProps, "disabled">) {
  const [isOpen, setIsOpen] = useState(false);
  const { resolvedTheme } = useTheme();
  return (
    <Popover open={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label="Add reaction"
            data-reveal={revealOnHover || undefined}
            className="dank:size-8 dank:[&>svg]:size-5 size-7 data-reveal:opacity-0 data-reveal:group-hover/reactable:opacity-100 data-reveal:focus-visible:opacity-100 data-reveal:data-popup-open:opacity-100"
          >
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
              const existing = reactions.find((reaction) => reaction.emoji === emoji);
              onToggle(emoji, existing?.reacted ?? false);
            }}
          />
        </Suspense>
      </PopoverContent>
    </Popover>
  );
}
