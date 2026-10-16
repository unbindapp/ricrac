import { useQuery } from "@tanstack/react-query";

import type { ProjectLabelRef } from "@/api/ops.gen";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { projectLabelsQuery } from "@/lib/queries/projects";

/** Project labels as projects reference them, by id. */
export function useProjectLabelsById() {
  const { workspace } = useWorkspace();
  const { data } = useQuery(projectLabelsQuery(workspace.id));
  return new Map<string, ProjectLabelRef>(
    (data ?? []).map((label) => [
      label.id,
      { id: label.id, name: label.name, color: label.color, parent: label.parent?.name ?? null },
    ]),
  );
}
