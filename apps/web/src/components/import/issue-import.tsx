import { type DragEvent, useState } from "react";

import type { ImportedIssues } from "@/api/ops.gen";
import { CircleCheckIcon, FileJsonIcon, UploadIcon, XIcon } from "@/components/icons";
import { parseIssuesJson, type TParsedIssues } from "@/components/import/parse-issues";
import { ErrorCard } from "@/components/layout/error-card";
import { SettingsSection } from "@/components/layout/settings-section";
import { Button, LinkButton } from "@/components/ui/button";
import { FileTrigger } from "@/components/ui/file-trigger";
import { Textarea } from "@/components/ui/textarea";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
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

/** Pick a file (or paste JSON), check it, then import. */
export function IssueImport() {
  const { workspace } = useWorkspace();
  const team = useWorkspaceTeam();
  const [source, setSource] = useState<TSource | null>(null);
  const [isPasting, setIsPasting] = useState(false);
  const [pasted, setPasted] = useState("");
  const [readError, setReadError] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [checked, setChecked] = useState<ImportedIssues | null>(null);
  const [imported, setImported] = useState<ImportedIssues | null>(null);
  const run = useImportIssuesMutation(workspace.id);

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
      { team: team.id, issues: source.parsed.issues, dryRun },
      { onSuccess: (result) => (dryRun ? setChecked(result) : setImported(result)) },
    );
  };

  if (imported) {
    return (
      <SettingsSection title="Imported">
        <div className="flex items-start gap-3">
          <CircleCheckIcon className="text-success mt-0.5 size-5 shrink-0" />
          <div className="flex flex-col gap-1 text-sm">
            <p className="font-medium">
              {plural(imported.imported, "issue")} and {plural(imported.comments, "comment")} are
              in.
            </p>
            <p className="text-muted-foreground">
              {rangeText(imported.identifiers)} The next issue is {imported.nextIdentifier}.
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
            to="/$workspace/issues"
            params={{ workspace: workspace.urlKey }}
            variant="ghost"
            size="sm"
          >
            Open issues
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
          className="data-dragging:border-primary data-dragging:bg-accent dank:rounded-md dank:border-m3-outline flex flex-col items-center gap-2 rounded-lg border border-dashed p-6 text-center"
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
      {checked && (
        <div className="bg-success/1-10 border-success/5-10 flex items-start gap-3 rounded-lg border p-3">
          <CircleCheckIcon className="text-success mt-0.5 size-4 shrink-0" />
          <div className="flex flex-col gap-1 text-sm">
            <p className="font-medium">
              Ready: {plural(checked.imported, "issue")}, {plural(checked.comments, "comment")}
              {checked.relations ? ` and ${plural(checked.relations, "relation")}` : ""}.
            </p>
            <p className="text-muted-foreground">
              {rangeText(checked.identifiers)} The next issue will be {checked.nextIdentifier}.
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
