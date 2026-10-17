import { ArchiveIcon, type LucideIcon, Trash2Icon } from "lucide-react";

import type { TProjectScope } from "@/components/projects/project-groups";
import { EmptyState } from "@/components/layout/empty-state";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";

export function ScopeTabs({
  scope,
  onChange,
}: {
  scope: TProjectScope;
  onChange: (scope: TProjectScope) => void;
}) {
  return (
    <Tabs
      value={scope}
      onValueChange={(value) =>
        onChange(value === "archived" || value === "trash" ? value : "active")
      }
    >
      <TabsList className="border-b-0">
        <TabsTab value="active">Active</TabsTab>
        <TabsTab value="archived">
          <ArchiveIcon />
          Archived
        </TabsTab>
        <TabsTab value="trash">
          <Trash2Icon />
          Trash
        </TabsTab>
      </TabsList>
    </Tabs>
  );
}

const scopeEmpty: Record<Exclude<TProjectScope, "active">, { icon: LucideIcon; title: string }> = {
  archived: { icon: ArchiveIcon, title: "No archived {noun}s yet" },
  trash: { icon: Trash2Icon, title: "No trashed {noun}s yet" },
};

/** The empty state of the archive or the trash. */
export function ScopeEmpty({
  scope,
  noun,
}: {
  scope: Exclude<TProjectScope, "active">;
  noun: string;
}) {
  const { icon, title } = scopeEmpty[scope];
  return (
    <EmptyState
      icon={icon}
      title={title.replace("{noun}", noun)}
      description={
        scope === "trash"
          ? `Trashed ${noun}s stay here for 30 days before they are deleted.`
          : `Archived ${noun}s are kept out of the way, with everything in them.`
      }
    />
  );
}
