import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { Template, TemplateDetail } from "@/api/ops.gen";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TTemplate = Template;
export type TTemplateDetail = TemplateDetail;
export type TTemplateType = Template["type"];

export const queryKeyTemplates = {
  all: (workspaceId: string) => [workspaceId, "templates"] as const,
  list: (workspaceId: string, team: string | null) =>
    [workspaceId, "templates", "list", team] as const,
  detail: (workspaceId: string, id: string) => [workspaceId, "templates", "detail", id] as const,
};

/** Every template you can see; with team, that team's and the workspace-wide ones. */
export const templatesQuery = (workspaceId: string, team: string | null = null) =>
  queryOptions({
    queryKey: queryKeyTemplates.list(workspaceId, team),
    queryFn: () => callAll(workspaceId, "list_templates", { team }),
  });

/** A template with the data it fills in. */
export const templateQuery = (workspaceId: string, id: string) =>
  queryOptions({
    queryKey: queryKeyTemplates.detail(workspaceId, id),
    queryFn: () => call(workspaceId, "get_template", { id }),
  });

function useInvalidateTemplates(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyTemplates.all(workspaceId) });
}

export function useSaveTemplateMutation(workspaceId: string) {
  const invalidate = useInvalidateTemplates(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_template">) => call(workspaceId, "save_template", args),
    meta: { errorTitle: "Could not save template" },
    onSuccess: invalidate,
  });
}

export function useDeleteTemplateMutation(workspaceId: string) {
  const invalidate = useInvalidateTemplates(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_template", { id }),
    meta: { errorTitle: "Could not delete template" },
    onSuccess: invalidate,
  });
}
