import { Link } from "@tanstack/react-router";

import type { Document } from "@/api/ops.gen";
import { useDocumentParent } from "@/components/documents/document-parent";
import { FileTextIcon } from "@/components/icons";
import { PersonAvatar } from "@/components/projects/people";
import { EntityIcon } from "@/components/shared/entity-icon";
import { RelativeTime } from "@/components/shared/relative-time";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";

type TProps = {
  document: Document;
  /** Leave out where it belongs, on the page of that parent. */
  showParent?: boolean;
};

export function DocumentRow({ document, showParent = true }: TProps) {
  const { workspace } = useWorkspace();
  const parent = useDocumentParent(document, workspace.urlKey);
  const editor = document.updatedBy ?? document.creator;
  return (
    <Link
      role="listitem"
      to="/$workspace/document/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(document) }}
      data-trashed={document.trashed || undefined}
      className="has-hover:hover:bg-muted focus-visible:bg-muted data-trashed:text-muted-foreground flex h-9 w-full items-center gap-3 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0"
    >
      <span className="dank:*:size-6 flex shrink-0">
        <EntityIcon icon={document.icon} color={document.color} fallback={FileTextIcon} />
      </span>
      <span className="dank:font-normal min-w-0 flex-1 truncate font-medium">{document.title}</span>
      {showParent && parent && (
        <span className="text-muted-foreground dank:type-label-small hidden max-w-56 shrink-0 items-center gap-1.5 text-xs sm:flex">
          {parent.icon}
          <span className="truncate">{parent.label}</span>
        </span>
      )}
      <span className="flex w-6 shrink-0 justify-center">
        {editor && (
          <SimpleTooltip content={`Last edited by ${editor.name}`}>
            <span className="flex">
              <PersonAvatar user={editor} size="xs" />
            </span>
          </SimpleTooltip>
        )}
      </span>
      <span className="text-muted-foreground dank:type-label-small w-20 shrink-0 text-right text-xs">
        <RelativeTime value={document.updatedAt} />
      </span>
    </Link>
  );
}
