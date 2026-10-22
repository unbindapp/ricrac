import { ArchiveIcon, CircleDotIcon, type LucideIcon, Trash2Icon } from "@/components/icons";
import type { TProjectScope } from "@/components/projects/project-groups";
import { EmptyState } from "@/components/layout/empty-state";
import { FilterMenu } from "@/components/shared/filter-menu";

export function ScopeTabs({
  scope,
  onChange,
}: {
  scope: TProjectScope;
  onChange: (scope: TProjectScope) => void;
}) {
  return (
    <FilterMenu
      label="Show"
      value={scope}
      options={[
        { value: "active", label: "Active", icon: CircleDotIcon },
        { value: "archived", label: "Archived", icon: ArchiveIcon },
        { value: "trash", label: "Trash", icon: Trash2Icon },
      ]}
      onChange={onChange}
    />
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
