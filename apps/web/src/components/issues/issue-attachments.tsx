import { type FormEvent, useState } from "react";

import type { IssueAttachment, IssueDetail } from "@/api/ops.gen";
import {
  BugIcon,
  ChevronDownIcon,
  ExternalLinkIcon,
  FileIcon,
  GitBranchIcon,
  GitCommitHorizontalIcon,
  GitMergeIcon,
  HashIcon,
  ImageIcon,
  LinkIcon,
  type LucideIcon,
  NotebookTextIcon,
  PaperclipIcon,
  PenToolIcon,
  UploadIcon,
  VideoIcon,
  XIcon,
} from "@/components/icons";
import { GithubIcon } from "@/components/icons/github";
import {
  githubLinkOf,
  type TPullRequestState,
  type TPullRequestStats,
} from "@/components/issues/github-link";
import { List, ListRow, ListRowText } from "@/components/layout/list";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { MarkdownView } from "@/components/shared/markdown-view";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { FileTrigger } from "@/components/ui/file-trigger";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { toast } from "@/components/ui/toast";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  useAttachUploadMutation,
  useCreateAttachmentMutation,
  useDeleteAttachmentMutation,
} from "@/lib/queries/issues";
import { fileKind } from "@/lib/helpers/media";
import { uploadFile } from "@/lib/queries/uploads";
import { errorText } from "@/lib/server/errors";

type TSource = { label: string; icon: LucideIcon | typeof GithubIcon; rank: number };

/** How each attachment source reads, in the order the sections show. */
const SOURCES: Record<string, TSource> = {
  github: { label: "GitHub", icon: GithubIcon, rank: 0 },
  github_branch: { label: "GitHub", icon: GithubIcon, rank: 0 },
  github_commit: { label: "GitHub", icon: GithubIcon, rank: 0 },
  gitlab: { label: "GitLab", icon: GitMergeIcon, rank: 1 },
  figma: { label: "Figma", icon: PenToolIcon, rank: 2 },
  sentry: { label: "Sentry", icon: BugIcon, rank: 3 },
  slack: { label: "Slack", icon: HashIcon, rank: 4 },
  notion: { label: "Notion", icon: NotebookTextIcon, rank: 5 },
  loom: { label: "Loom", icon: VideoIcon, rank: 6 },
  upload: { label: "Files", icon: FileIcon, rank: 7 },
  link: { label: "Links", icon: LinkIcon, rank: 8 },
};

const sourceOf = (attachment: IssueAttachment) => SOURCES[attachment.sourceType] ?? SOURCES.link;

function AttachmentIcon({ attachment }: { attachment: IssueAttachment }) {
  const github = githubLinkOf(attachment);
  if (github?.kind === "branch") return <GitBranchIcon className="text-muted-foreground size-4" />;
  if (github?.kind === "commit") {
    return <GitCommitHorizontalIcon className="text-muted-foreground size-4" />;
  }
  const kind = attachment.sourceType === "upload" ? fileKind(attachment.title) : null;
  if (kind === "image") return <ImageIcon className="text-muted-foreground size-4 shrink-0" />;
  if (kind === "video") return <VideoIcon className="text-muted-foreground size-4 shrink-0" />;
  const Icon = sourceOf(attachment).icon;
  return <Icon className="text-muted-foreground size-4 shrink-0" />;
}

/** Upload files or add a link, from one popover. */
export function AttachMenu({ issue, label = "Attach" }: { issue: IssueDetail; label?: string }) {
  const { workspace } = useWorkspace();
  const [isOpen, setIsOpen] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const { mutateAsync: attachUpload } = useAttachUploadMutation(workspace.id);

  const uploadFiles = async (files: File[]) => {
    setIsUploading(true);
    try {
      for (const file of files) {
        const prepared = await uploadFile(workspace.id, file);
        await attachUpload({ fileId: prepared.fileId, issue: issue.id, title: file.name });
      }
    } catch (error) {
      toast.add({ type: "error", title: "Upload failed", description: errorText(error) });
    } finally {
      setIsUploading(false);
    }
  };

  return (
    <Popover open={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger
        render={
          <Button
            variant="ghost"
            size="sm"
            isPending={isUploading}
            data-m3-chip="assist"
            className="text-muted-foreground font-normal"
          >
            <PaperclipIcon className="size-4" />
            {label}
          </Button>
        }
      />
      <PopoverContent align="start" className="flex w-80 flex-col gap-4">
        <FileTrigger
          multiple
          onSelect={(files) => {
            setIsOpen(false);
            void uploadFiles(files);
          }}
        >
          {(pickFiles) => (
            <Button variant="outline" size="sm" onClick={pickFiles}>
              <UploadIcon className="size-4" />
              Upload files
            </Button>
          )}
        </FileTrigger>
        <AddLinkForm issueId={issue.id} onDone={() => setIsOpen(false)} />
      </PopoverContent>
    </Popover>
  );
}

const pullRequestStates: Record<
  TPullRequestState,
  { label: string; variant: "process" | "muted" | "success" | "destructive" }
> = {
  open: { label: "Open", variant: "process" },
  draft: { label: "Draft", variant: "muted" },
  merged: { label: "Merged", variant: "success" },
  closed: { label: "Closed", variant: "destructive" },
};

/** Attachments grouped by where they come from (GitHub, Figma, files, links, ...). */
export function IssueAttachments({ issue }: { issue: IssueDetail }) {
  const { canEdit } = useWorkspace();
  if (issue.attachments.length === 0) return null;
  const sections = new Map<string, IssueAttachment[]>();
  const sorted = [...issue.attachments].sort((a, b) => sourceOf(a).rank - sourceOf(b).rank);
  for (const attachment of sorted) {
    const label = sourceOf(attachment).label;
    sections.set(label, [...(sections.get(label) ?? []), attachment]);
  }
  return (
    <section aria-label="Attachments" className="flex w-full flex-col gap-2">
      <div className="flex min-h-7 items-center gap-2">
        <h2 className="dank:type-title-medium text-sm font-semibold">Attachments</h2>
        <span className="text-muted-foreground text-xs">{issue.attachments.length}</span>
        <span className="flex-1" />
        {canEdit && <AttachMenu issue={issue} label="Add" />}
      </div>
      <List>
        {[...sections.entries()].map(([label, attachments]) => (
          <div key={label} role="group" aria-label={label} className="flex flex-col">
            {sections.size > 1 && (
              <p className="bg-muted text-muted-foreground border-b px-3 py-1 text-xs font-medium">
                {label}
              </p>
            )}
            {attachments.map((attachment) => (
              <AttachmentRow key={attachment.id} attachment={attachment} canEdit={canEdit} />
            ))}
          </div>
        ))}
      </List>
    </section>
  );
}

function AttachmentRow({ attachment, canEdit }: { attachment: IssueAttachment; canEdit: boolean }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: remove, isPending } = useDeleteAttachmentMutation(workspace.id);
  const [isExpanded, setIsExpanded] = useState(false);
  const github = githubLinkOf(attachment);
  const pullRequest = github?.kind === "pull_request" ? github : null;
  const pullRequestState = pullRequest?.state ? pullRequestStates[pullRequest.state] : null;
  const canExpand = Boolean(pullRequest && (pullRequest.summary || pullRequest.stats));
  const title =
    github?.kind === "commit" ? (
      <>
        <span className="text-muted-foreground font-mono text-xs">{github.sha}</span>{" "}
        {github.subject}
      </>
    ) : (
      attachment.title
    );
  const row = (
    <ListRow
      actions={
        <>
          {pullRequest?.stats && <PullRequestSize stats={pullRequest.stats} />}
          {pullRequestState && (
            <Badge variant={pullRequestState.variant}>{pullRequestState.label}</Badge>
          )}
          <span className="text-muted-foreground text-xs">
            <RelativeTime value={attachment.createdAt} />
          </span>
          {canExpand && (
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              className="dank:size-8 dank:[&>svg]:size-5 size-6"
              aria-label={isExpanded ? "Hide summary" : "Show summary"}
              aria-expanded={isExpanded}
              onClick={() => setIsExpanded(!isExpanded)}
            >
              <ChevronDownIcon
                className={cn("size-3.5 transition-transform", isExpanded && "rotate-180")}
              />
            </Button>
          )}
          {canEdit && (
            <ConfirmDialog
              trigger={
                <Button
                  variant="ghost"
                  size="icon"
                  forceMinSize={false}
                  className="dank:size-8 dank:[&>svg]:size-5 size-6"
                  aria-label={`Remove ${attachment.title}`}
                  isPending={isPending}
                >
                  <XIcon className="size-3.5" />
                </Button>
              }
              title="Remove attachment"
              description={`${attachment.title} is removed from the issue.`}
              confirmLabel="Remove attachment"
              onConfirm={() => remove(attachment.id)}
            />
          )}
        </>
      }
    >
      <AttachmentIcon attachment={attachment} />
      <a
        href={attachment.url}
        target="_blank"
        rel="noreferrer noopener"
        className="flex min-w-0 flex-1 outline-hidden focus-visible:underline has-hover:hover:underline"
      >
        <ListRowText
          primary={title}
          secondary={github?.reference || attachment.subtitle || undefined}
        />
      </a>
    </ListRow>
  );
  if (!pullRequest || !isExpanded) return row;
  return (
    <>
      {row}
      <div
        role="region"
        aria-label={`Summary of ${attachment.title}`}
        className="flex flex-col gap-2 border-b py-3 pr-3 pl-9 last:border-b-0"
      >
        {pullRequest.summary ? (
          <div className="max-h-64 overflow-y-auto text-sm">
            <MarkdownView markdown={pullRequest.summary} />
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">No description.</p>
        )}
        <div className="text-muted-foreground flex items-center gap-2 text-xs">
          {pullRequest.base && (
            <span className="flex items-center gap-1.5">
              <GitBranchIcon className="size-3.5" />
              Into <span className="font-mono">{pullRequest.base}</span>
            </span>
          )}
          {pullRequest.base && pullRequest.stats && <span aria-hidden>·</span>}
          {pullRequest.stats && (
            <span>
              {pullRequest.stats.changedFiles}{" "}
              {pullRequest.stats.changedFiles === 1 ? "file" : "files"} changed
            </span>
          )}
          <span className="flex-1" />
          <a
            href={attachment.url}
            target="_blank"
            rel="noreferrer noopener"
            className="flex items-center gap-1.5 has-hover:hover:underline"
          >
            <ExternalLinkIcon className="size-3.5" />
            Open on GitHub
          </a>
        </div>
      </div>
    </>
  );
}

function PullRequestSize({ stats }: { stats: TPullRequestStats }) {
  return (
    <span
      className="flex items-center gap-1 font-mono text-xs"
      aria-label={`${stats.additions} additions, ${stats.deletions} deletions`}
    >
      <span className="text-success">+{stats.additions}</span>
      <span className="text-destructive">−{stats.deletions}</span>
    </span>
  );
}

function AddLinkForm({ issueId, onDone }: { issueId: string; onDone: () => void }) {
  const { workspace } = useWorkspace();
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const { mutateAsync: createAttachment, isPending } = useCreateAttachmentMutation(workspace.id);
  const isValid = /^https?:\/\/\S+$/i.test(url.trim());

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!isValid) return;
    const trimmed = url.trim();
    await createAttachment({ issue: issueId, url: trimmed, title: title.trim() || trimmed });
    setUrl("");
    setTitle("");
    onDone();
  };

  return (
    <form
      className="flex flex-col gap-2 border-t pt-4"
      onSubmit={(event) => void submit(event).catch(() => undefined)}
    >
      <p className="text-muted-foreground text-xs">Or link a pull request, design, doc, …</p>
      <Input
        aria-label="URL"
        placeholder="https://"
        value={url}
        onChange={(event) => setUrl(event.target.value)}
      />
      <Input
        aria-label="Title"
        placeholder="Title (optional)"
        value={title}
        onChange={(event) => setTitle(event.target.value)}
      />
      <Button
        type="submit"
        size="sm"
        isPending={isPending}
        disabled={!isValid}
        className="self-end"
      >
        Add link
      </Button>
    </form>
  );
}
