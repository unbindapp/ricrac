import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import { PageContainer } from "@/components/layout/page-container";
import { ProjectHeaderBar } from "@/components/projects/project-header";
import { ProjectIssues } from "@/components/projects/project-issues";
import { ProjectLoader } from "@/components/projects/project-page";

const searchSchema = z.object({ milestone: z.string().optional() });

export const Route = createFileRoute("/$workspace/_workspace/project/$slug/issues")({
  validateSearch: zodValidator(searchSchema),
  component: ProjectSlugIssuesPage,
});

function ProjectSlugIssuesPage() {
  const { slug } = Route.useParams();
  const { milestone } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <ProjectLoader slug={slug}>
      {(project) => (
        <PageContainer>
          <div className="flex w-full flex-col gap-4">
            <ProjectHeaderBar project={project} tab="issues" />
            <ProjectIssues
              project={project}
              milestone={milestone}
              onMilestoneChange={(next) => void navigate({ search: { milestone: next } })}
            />
          </div>
        </PageContainer>
      )}
    </ProjectLoader>
  );
}
