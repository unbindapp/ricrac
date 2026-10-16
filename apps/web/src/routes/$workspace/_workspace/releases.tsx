import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import { ReleasesView } from "@/components/releases/releases-view";

const searchSchema = z.object({
  pipeline: z.string().optional(),
  tab: z.enum(["releases", "notes"]).optional(),
});

export const Route = createFileRoute("/$workspace/_workspace/releases")({
  validateSearch: zodValidator(searchSchema),
  component: ReleasesPage,
});

function ReleasesPage() {
  const { pipeline, tab } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <ReleasesView
      pipeline={pipeline}
      tab={tab ?? "releases"}
      onChange={(next) =>
        void navigate({ search: (prev) => ({ ...prev, ...next }), replace: true })
      }
    />
  );
}
