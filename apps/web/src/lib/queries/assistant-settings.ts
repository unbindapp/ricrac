import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { AssistantSettings, AssistantSettingsInput } from "@/api/account.gen";
import { queryKeyMe } from "@/lib/queries/me";
import { account } from "@/lib/server/client";

export const queryKeyAssistantSettings = (workspaceId: string) =>
  [workspaceId, "assistant-settings"] as const;

export const assistantSettingsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyAssistantSettings(workspaceId),
    queryFn: () => account.assistantSettings(workspaceId),
  });

/** Saves at once and shows the change before the server answers; who may ask shows up on me. */
export function useSaveAssistantSettingsMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: AssistantSettingsInput) =>
      account.saveAssistantSettings(workspaceId, input),
    meta: { errorTitle: "Could not save Claude settings" },
    onMutate: (input) =>
      queryClient.setQueryData(
        queryKeyAssistantSettings(workspaceId),
        (settings?: AssistantSettings) =>
          settings && {
            enabled: input.enabled ?? settings.enabled,
            userIds: input.userIds ?? settings.userIds,
          },
      ),
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeyAssistantSettings(workspaceId) }),
        queryClient.invalidateQueries({ queryKey: queryKeyMe }),
      ]),
  });
}
