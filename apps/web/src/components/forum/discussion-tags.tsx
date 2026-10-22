import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { HashIcon, PlusIcon, TagIcon } from "@/components/icons/symbols.gen";
import { NONE_KEY } from "@/components/issues/grouping";
import { PickerTrigger, type TPickerAppearance } from "@/components/issues/pickers";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { Badge } from "@/components/ui/badge";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { DISCUSSION_MAX_TAGS, discussionTagsQuery, normalTag } from "@/lib/queries/discussions";

const CREATE_PREFIX = "create:";

type TPickerProps = {
  value: string[];
  onChange: (tags: string[]) => void;
  appearance?: TPickerAppearance;
  disabled?: boolean;
};

/** Picks a discussion's tags from the ones in use, or makes new ones from what you type. */
export function DiscussionTagsPicker({
  value,
  onChange,
  appearance = "compact",
  disabled,
}: TPickerProps) {
  const { workspace } = useWorkspace();
  const { data: tags = [], isPending } = useQuery(discussionTagsQuery(workspace.id));
  const [query, setQuery] = useState("");
  const typed = normalTag(query);
  const counts = new Map(tags.map((tag) => [tag.name, tag.count]));
  const known = [...new Set([...value, ...tags.map((tag) => tag.name)])];
  const options: TPickerOption[] = [
    ...(typed && !known.includes(typed)
      ? [
          {
            value: `${CREATE_PREFIX}${typed}`,
            label: `Create tag “${typed}”`,
            icon: <PlusIcon className="text-muted-foreground size-4" />,
          },
        ]
      : []),
    ...known
      .filter((name) => name.includes(typed))
      .map((name) => ({
        value: name,
        label: name,
        icon: <HashIcon className="text-muted-foreground size-4" />,
        description: counts.has(name) ? String(counts.get(name)) : undefined,
      })),
  ];
  const change = (next: string[]) => {
    const picked = [...new Set(next.map((tag) => tag.replace(CREATE_PREFIX, "")))];
    if (picked.length > DISCUSSION_MAX_TAGS) {
      toast.add({ type: "info", title: `A discussion takes up to ${DISCUSSION_MAX_TAGS} tags` });
      return;
    }
    if (next.some((tag) => tag.startsWith(CREATE_PREFIX))) setQuery("");
    onChange(picked);
  };
  return (
    <PropertyPicker
      label="Tags"
      multiple
      options={options}
      value={value}
      isPending={isPending}
      disabled={disabled}
      searchPlaceholder="Find or create a tag…"
      emptyText="Type to create a tag"
      search={query}
      onSearchChange={setQuery}
      onChange={change}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label="Tags"
          icon={<TagIcon className="text-muted-foreground size-4" />}
          text={value.length > 0 ? value.join(", ") : "Add tags"}
          muted={value.length === 0}
        />
      }
    />
  );
}

/** Filters the forum by one tag; null shows every discussion. */
export function DiscussionTagFilter({
  value,
  onChange,
}: {
  value: string | null;
  onChange: (tag: string | null) => void;
}) {
  const { workspace } = useWorkspace();
  const { data: tags = [], isPending } = useQuery(discussionTagsQuery(workspace.id));
  const options: TPickerOption[] = [
    {
      value: NONE_KEY,
      label: "All tags",
      icon: <TagIcon className="text-muted-foreground size-4" />,
    },
    ...tags.map((tag) => ({
      value: tag.name,
      label: tag.name,
      description: String(tag.count),
      icon: <HashIcon className="text-muted-foreground size-4" />,
    })),
  ];
  return (
    <PropertyPicker
      label="Filter by tag"
      options={options}
      value={[value ?? NONE_KEY]}
      isPending={isPending}
      emptyText="No tags yet"
      onChange={([next]) => onChange(!next || next === NONE_KEY ? null : next)}
      trigger={
        <PickerTrigger
          appearance="compact"
          label="Filter by tag"
          icon={<TagIcon className="text-muted-foreground size-4" />}
          text={value ?? "All tags"}
        />
      }
    />
  );
}

export function DiscussionTagBadges({ tags, max }: { tags: string[]; max?: number }) {
  const shown = max === undefined ? tags : tags.slice(0, max);
  const hidden = tags.length - shown.length;
  return (
    <>
      {shown.map((tag) => (
        <Badge key={tag} variant="muted">
          <HashIcon />
          <span className="truncate">{tag}</span>
        </Badge>
      ))}
      {hidden > 0 && <Badge variant="muted">+{hidden}</Badge>}
    </>
  );
}
