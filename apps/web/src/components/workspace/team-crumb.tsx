import type { Team } from "@/api/ops.gen";
import {
  BoxIcon,
  CircleDotIcon,
  ListTodoIcon,
  RefreshCwIcon,
  TriangleIcon,
} from "@/components/icons/symbols.gen";
import type { TBreadcrumbItem, TBreadcrumbMenuItem } from "@/components/layout/breadcrumb";
import { TeamIcon } from "@/components/shared/team-icon";

type TCrumbTeam = Pick<Team, "key" | "name" | "icon" | "color"> &
  Partial<Pick<Team, "triageEnabled" | "cyclesEnabled">>;

/** The pages a team has, in the order the navigation lists them. */
export function teamPages(team: TCrumbTeam, workspaceUrlKey: string): TBreadcrumbMenuItem[] {
  const params = { workspace: workspaceUrlKey, key: team.key };
  const pages: (TBreadcrumbMenuItem & { isShown: boolean })[] = [
    {
      label: "Issues",
      icon: CircleDotIcon,
      link: { to: "/$workspace/team/$key/issues", params },
      isShown: true,
    },
    {
      label: "Backlog",
      icon: ListTodoIcon,
      link: { to: "/$workspace/team/$key/backlog", params },
      isShown: true,
    },
    {
      label: "Triage",
      icon: TriangleIcon,
      link: { to: "/$workspace/team/$key/triage", params },
      isShown: Boolean(team.triageEnabled),
    },
    {
      label: "Cycles",
      icon: RefreshCwIcon,
      link: { to: "/$workspace/team/$key/cycles", params },
      isShown: Boolean(team.cyclesEnabled),
    },
    {
      label: "Projects",
      icon: BoxIcon,
      link: { to: "/$workspace/team/$key/projects", params },
      isShown: true,
    },
  ];
  return pages.filter((page) => page.isShown);
}

export function teamCrumb(team: TCrumbTeam, workspaceUrlKey: string): TBreadcrumbItem {
  return {
    label: team.name,
    icon: <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />,
    link: {
      to: "/$workspace/team/$key/issues",
      params: { workspace: workspaceUrlKey, key: team.key },
    },
    menu: teamPages(team, workspaceUrlKey),
  };
}
