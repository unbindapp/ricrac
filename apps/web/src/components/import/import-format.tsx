import { EXAMPLE_IMPORT } from "@/components/import/parse-issues";
import { SettingsSection } from "@/components/layout/settings-section";
import { CopyButton } from "@/components/shared/copy-button";

const FIELDS: [string, string][] = [
  ["title", "Required. Everything else is optional."],
  ["number", "Keeps the identifier: 42 becomes ENG-42. Must be free in the team; gaps are fine."],
  ["description", "Markdown, kept as is."],
  ["state", "A status name or type of the team (In Progress, started). The default otherwise."],
  ["priority", "0 none, 1 urgent, 2 high, 3 medium, 4 low."],
  ["assignee, creator", "Email, name or id of a workspace member. The creator is you otherwise."],
  ["labels", "Label names; Group/Label for a label in a group."],
  ["project, milestone, cycle", "Names (or a cycle number) of things already in Ricrac."],
  ["parent", "The parent's identifier, in the file or already in Ricrac."],
  ["relatedTo, blocks, blockedBy, duplicateOf", "Identifiers of other issues."],
  ["createdAt, updatedAt, startedAt, completedAt, canceledAt, archivedAt", "ISO 8601 dates."],
  ["dueDate", "YYYY-MM-DD."],
  ["estimate, sortOrder", "Numbers on the team's scale and board."],
  ["links", "Attachments: [{ url, title, subtitle }]."],
  ["subscribers", "People to keep in the loop."],
  ["comments", "Oldest first: [{ author, body, createdAt }]."],
];

/** How to shape the file, with an example to copy. */
export function ImportFormat() {
  return (
    <SettingsSection
      title="The format"
      description="A JSON list of issues, or an object with an issues list. Export from your tracker and reshape it with a script, or ask an agent to. Up to 2000 issues at a time; the import is all or nothing and notifies nobody."
    >
      <dl className="grid grid-cols-1 gap-x-4 gap-y-2 text-sm sm:grid-cols-3">
        {FIELDS.map(([name, about]) => (
          <div key={name} className="contents">
            <dt className="font-mono text-xs leading-5">{name}</dt>
            <dd className="text-muted-foreground sm:col-span-2">{about}</dd>
          </div>
        ))}
      </dl>
      <div className="relative">
        <pre className="bg-muted overflow-x-auto rounded-md p-3 font-mono text-xs leading-relaxed">
          {EXAMPLE_IMPORT}
        </pre>
        <div className="absolute top-2 right-2">
          <CopyButton value={EXAMPLE_IMPORT} label="Copy example" variant="outline" />
        </div>
      </div>
    </SettingsSection>
  );
}
