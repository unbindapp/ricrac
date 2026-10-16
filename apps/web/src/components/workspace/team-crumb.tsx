import type { Team } from "@/api/ops.gen";
import type { TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { TeamIcon } from "@/components/shared/team-icon";

export function teamCrumb(
  team: Pick<Team, "key" | "name" | "icon" | "color">,
  workspaceUrlKey: string,
): TBreadcrumbItem {
  return {
    label: team.name,
    icon: <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />,
    link: {
      to: "/$workspace/team/$key/issues",
      params: { workspace: workspaceUrlKey, key: team.key },
    },
  };
}
