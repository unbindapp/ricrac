import { z } from "zod";

export const ALL_TEAMS = "all";

/** What a webhook can receive, grouped as the form shows them (save_webhook's resourceTypes). */
export const RESOURCE_GROUPS: { label: string; types: string[] }[] = [
  {
    label: "Issues",
    types: ["Issue", "Comment", "Reaction", "Attachment", "IssueLabel", "IssueRelation"],
  },
  {
    label: "Projects",
    types: [
      "Project",
      "ProjectLabel",
      "ProjectMilestone",
      "ProjectUpdate",
      "Initiative",
      "InitiativeUpdate",
    ],
  },
  { label: "Teams", types: ["Cycle", "Team", "WorkflowState", "Document"] },
  {
    label: "Customers and releases",
    types: ["Customer", "CustomerNeed", "Release", "ReleasePipeline", "ReleaseNote"],
  },
];

export const RESOURCE_TYPES = RESOURCE_GROUPS.flatMap((group) => group.types);

/** "IssueLabel" as "Issue label". */
export function resourceLabel(type: string) {
  const words = type.replace(/([a-z])([A-Z])/g, "$1 $2").toLowerCase();
  return words.charAt(0).toUpperCase() + words.slice(1);
}

export const webhookFormSchema = z.object({
  url: z
    .string()
    .trim()
    .min(1, "URL is required.")
    .max(2048, "Keep the URL under 2048 characters.")
    .refine(
      (value) =>
        /^https:\/\/[^\s/]+/i.test(value) ||
        /^http:\/\/(localhost|127\.0\.0\.1)(:\d+)?(\/|$)/i.test(value),
      "Use an https URL (http only for localhost).",
    ),
  label: z.string().trim().max(100, "Keep the label under 100 characters."),
  resourceTypes: z
    .array(z.enum(RESOURCE_TYPES as [string, ...string[]]))
    .min(1, "Pick at least one thing to receive."),
  team: z.string(),
  enabled: z.boolean(),
});

export type TWebhookFormValues = z.infer<typeof webhookFormSchema>;

/** The save_webhook arguments for the form's values. */
export function toSaveArgs(values: TWebhookFormValues) {
  return {
    url: values.url.trim(),
    label: values.label.trim() || null,
    resourceTypes: values.resourceTypes,
    team: values.team === ALL_TEAMS ? null : values.team,
    enabled: values.enabled,
  };
}
