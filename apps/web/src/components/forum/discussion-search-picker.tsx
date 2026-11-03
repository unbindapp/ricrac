import { useQuery } from "@tanstack/react-query";
import { type ReactElement, useState } from "react";
import { useDebounceValue } from "usehooks-ts";

import type { Discussion } from "@/api/ops.gen";
import { LockIcon, MessagesSquareIcon } from "@/components/icons";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { searchDiscussionsQuery } from "@/lib/queries/discussions";

/** Finds discussions by words and picks one; without words, the most recently active. */
export function DiscussionSearchPicker({
  label,
  exclude = [],
  onSelect,
  trigger,
}: {
  label: string;
  exclude?: string[];
  onSelect: (discussion: Discussion) => void;
  trigger: ReactElement;
}) {
  const { workspace } = useWorkspace();
  const [query, setQuery] = useState("");
  const [debounced] = useDebounceValue(query, 200);
  const { data, isFetching } = useQuery(searchDiscussionsQuery(workspace.id, debounced));
  const results = (data ?? []).filter((discussion) => !exclude.includes(discussion.id));
  const options: TPickerOption[] = results.map((discussion) => {
    const Icon = discussion.closedAt ? LockIcon : MessagesSquareIcon;
    return {
      value: discussion.id,
      label: discussion.title,
      description: discussion.tags.join(", ") || undefined,
      icon: <Icon className="text-muted-foreground size-4" />,
    };
  });
  return (
    <PropertyPicker
      label={label}
      searchPlaceholder="Search discussions by title…"
      options={options}
      value={[]}
      onSearchChange={setQuery}
      isPending={isFetching}
      emptyText={debounced ? "No matching discussions" : "No discussions yet"}
      wide
      onChange={([id]) => {
        const picked = results.find((discussion) => discussion.id === id);
        if (picked) onSelect(picked);
      }}
      trigger={trigger}
    />
  );
}
