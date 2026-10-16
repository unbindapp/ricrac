import { CircleCheckIcon, FileJsonIcon, UploadIcon, XIcon } from "lucide-react";
import { type DragEvent, useState } from "react";

import type { ImportedIssues } from "@/api/ops.gen";
import { parseIssuesJson, type TParsedIssues } from "@/components/import/parse-issues";
import { ErrorCard } from "@/components/layout/error-card";
import { SettingsSection } from "@/components/layout/settings-section";
import { TeamIcon } from "@/components/shared/team-icon";
import { Field, SimpleSelect } from "@/components/templates/template-fields";
import { Button, LinkButton } from "@/components/ui/button";
import { FileTrigger } from "@/components/ui/file-trigger";
import { Textarea } from "@/components/ui/textarea";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useImportIssuesMutation } from "@/lib/queries/imports";

type TSource = { name: string; parsed: TParsedIssues };

const plural = (count: number, noun: string) => `${count} ${noun}${count === 1 ? "" : "s"}`;

function sourceSummary(parsed: TParsedIssues) {
  const parts = [plural(parsed.issues.length, "issue")];
  if (parsed.comments) parts.push(plural(parsed.comments, "comment"));
  if (parsed.numbered < parsed.issues.length) {
    parts.push(`${parsed.issues.length - parsed.numbered} without a number take the next ones`);
  }
  return parts.join(", ");
}

/** Pick a file (or paste JSON) and a team, check it, then import. */
export function IssueImport() {
  const { workspace } = useWorkspace();
  const { teams, myTeams } = useTeams();
  const [picked, setTeam] = useState<string | null>(null);
  const team = picked ?? myTeams[0]?.id ?? teams[0]?.id ?? "";
  const [source, setSource] = useState<TSource | null>(null);
  const [isPasting, setIsPasting] = useState(false);
  const [pasted, setPasted] = useState("");
  const [readError, setReadError] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [checked, setChecked] = useState<ImportedIssues | null>(null);
  const [imported, setImported] = useState<ImportedIssues | null>(null);
  const run = useImportIssuesMutation(workspace.id);
  const teamOf = teams.find((item) => item.id === team);

  const reset = () => {
    setChecked(null);
    setImported(null);
    run.reset();
  };
  const read = (name: string, text: string) => {
    reset();
    const result = parseIssuesJson(text);
    if (!result.ok) {
      setSource(null);
      setReadError(result.error);
      return;
    }
    setReadError(null);
    setSource({ name, parsed: result.parsed });
    setIsPasting(false);
  };
  const onDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setIsDragging(false);
    const file = event.dataTransfer.files[0];
    if (file) void file.text().then((text) => read(file.name, text));
  };
  const start = (dryRun: boolean) => {
    if (!source || !team) return;
    run.mutate(
      { team, issues: source.parsed.issues, dryRun },
      { onSuccess: (result) => (dryRun ? setChecked(result) : setImported(result)) },
    );
  };

  const teamItems = teams.map((item) => ({
    value: item.id,
    label: item.name,
    icon: <TeamIcon teamKey={item.key} icon={item.icon} color={item.color} size="sm" />,
  }));

  if (imported && teamOf) {
    return (
      <SettingsSection title="Imported">
        <div className="flex items-start gap-3">
          <CircleCheckIcon className="text-success mt-0.5 size-5 shrink-0" />
          <div className="flex flex-col gap-1 text-sm">
            <p className="font-medium">
              {plural(imported.imported, "issue")} and {plural(imported.comments, "comment")} are in{" "}
              {teamOf.name}.
            </p>
            <p className="text-muted-foreground">
              {rangeText(imported.identifiers)} The team's next issue is {imported.nextIdentifier}.
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              reset();
              setSource(null);
            }}
          >
            Import more
          </Button>
          <LinkButton
            to="/$workspace/team/$key/issues"
            params={{ workspace: workspace.urlKey, key: teamOf.key }}
            variant="ghost"
            size="sm"
          >
            Open {teamOf.name} issues
          </LinkButton>
        </div>
      </SettingsSection>
    );
  }

  return (
    <SettingsSection
      title="Import issues"
      description="Issues keep their numbers, dates, people, labels and comments. Check first: nothing changes until you import."
    >
      <Field label="Into team">
        {(id) => (
          <SimpleSelect
            id={id}
            label="Into team"
            items={teamItems}
            value={team}
            onChange={(value) => {
              setTeam(value);
              reset();
            }}
          />
        )}
      </Field>
      {isPasting ? (
        <div className="flex flex-col gap-2">
          <Textarea
            aria-label="Issues as JSON"
            value={pasted}
            onChange={(event) => setPasted(event.target.value)}
            rows={10}
            spellCheck={false}
            placeholder='{"issues": [{"number": 1, "title": "…"}]}'
            className="font-mono text-xs"
            autoFocus
          />
          <div className="flex items-center gap-2">
            <Button size="sm" disabled={!pasted.trim()} onClick={() => read("Pasted JSON", pasted)}>
              Use this JSON
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setIsPasting(false)}>
              Cancel
            </Button>
          </div>
        </div>
      ) : source ? (
        <div className="flex items-center gap-3 rounded-lg border p-3">
          <FileJsonIcon className="text-muted-foreground size-5 shrink-0" />
          <div className="flex min-w-0 flex-1 flex-col">
            <span className="truncate text-sm font-medium">{source.name}</span>
            <span className="text-muted-foreground text-xs">{sourceSummary(source.parsed)}</span>
          </div>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Choose another file"
            onClick={() => {
              setSource(null);
              reset();
            }}
          >
            <XIcon className="size-4" />
          </Button>
        </div>
      ) : (
        <div
          data-dragging={isDragging || undefined}
          className="data-dragging:border-primary data-dragging:bg-accent flex flex-col items-center gap-2 rounded-lg border border-dashed p-6 text-center"
          onDragOver={(event) => {
            event.preventDefault();
            setIsDragging(true);
          }}
          onDragLeave={() => setIsDragging(false)}
          onDrop={onDrop}
        >
          <FileJsonIcon className="text-muted-foreground size-5" />
          <p className="text-sm font-medium">Drop a JSON file here</p>
          <div className="flex items-center gap-2">
            <FileTrigger
              accept="application/json,.json"
              onSelect={(files) => void files[0].text().then((text) => read(files[0].name, text))}
            >
              {(open) => (
                <Button variant="outline" size="sm" onClick={open}>
                  <UploadIcon className="size-4" />
                  Choose file
                </Button>
              )}
            </FileTrigger>
            <Button variant="ghost" size="sm" onClick={() => setIsPasting(true)}>
              Paste JSON
            </Button>
          </div>
        </div>
      )}
      {readError && <ErrorCard title="Can't read these issues" message={readError} />}
      {run.error && <ErrorCard title="Ricrac can't import this" message={run.error.message} />}
      {checked && teamOf && (
        <div className="bg-success/1-10 border-success/5-10 flex items-start gap-3 rounded-lg border p-3">
          <CircleCheckIcon className="text-success mt-0.5 size-4 shrink-0" />
          <div className="flex flex-col gap-1 text-sm">
            <p className="font-medium">
              Ready: {plural(checked.imported, "issue")}, {plural(checked.comments, "comment")}
              {checked.relations ? ` and ${plural(checked.relations, "relation")}` : ""} into{" "}
              {teamOf.name}.
            </p>
            <p className="text-muted-foreground">
              {rangeText(checked.identifiers)} The team's next issue will be{" "}
              {checked.nextIdentifier}.
            </p>
          </div>
        </div>
      )}
      {source && (
        <div className="flex items-center justify-end gap-2">
          {checked ? (
            <Button size="sm" isPending={run.isPending} onClick={() => start(false)}>
              Import {plural(checked.imported, "issue")}
            </Button>
          ) : (
            <Button
              size="sm"
              isPending={run.isPending}
              disabled={!team}
              onClick={() => start(true)}
            >
              Check import
            </Button>
          )}
        </div>
      )}
    </SettingsSection>
  );
}

function rangeText(identifiers: string[]) {
  if (identifiers.length === 0) return "";
  if (identifiers.length === 1) return `${identifiers[0]}.`;
  return `${identifiers[0]} to ${identifiers[identifiers.length - 1]}.`;
}
