import type { Project, ProjectStatus, ProjectStatusRef } from "@/api/ops.gen";

export type TProjectGroup = { status: ProjectStatusRef; projects: Project[] };

/** One group per status in the workspace's order; archived statuses only while projects use them. */
export function groupProjectsByStatus(statuses: ProjectStatus[], projects: Project[]) {
  const used = new Set(projects.map((project) => project.status.id));
  const shown: ProjectStatusRef[] = statuses.filter(
    (status) => !status.archivedAt || used.has(status.id),
  );
  const known = new Set(shown.map((status) => status.id));
  for (const project of projects) {
    if (known.has(project.status.id)) continue;
    known.add(project.status.id);
    shown.push(project.status);
  }
  return shown.map((status) => ({
    status,
    projects: projects.filter((project) => project.status.id === status.id),
  }));
}

export function matchesQuery(project: Project, query: string) {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  return [project.name, project.summary ?? "", project.lead?.name ?? ""].some((text) =>
    text.toLowerCase().includes(needle),
  );
}
