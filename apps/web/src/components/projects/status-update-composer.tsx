import { useState } from "react";

import { HEALTHS, HealthIcon, healthLabels } from "@/components/projects/health-badge";
import { useMentionUsers } from "@/components/projects/people";
import { FilterChip } from "@/components/shared/filter-chip";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useEditorUploads } from "@/lib/queries/uploads";
import type { THealth } from "@/lib/queries/projects";

type TProps = {
  initialBody?: string;
  initialHealth?: THealth;
  submitLabel?: string;
  onSubmit: (value: { body: string; health: THealth }) => Promise<unknown>;
  onCancel?: () => void;
};

export function StatusUpdateComposer({
  initialBody = "",
  initialHealth = "onTrack",
  submitLabel = "Post update",
  onSubmit,
  onCancel,
}: TProps) {
  const users = useMentionUsers();
  const { workspace } = useWorkspace();
  const uploads = useEditorUploads(workspace.id);
  const [body, setBody] = useState(initialBody);
  const [health, setHealth] = useState<THealth>(initialHealth);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const canSubmit = body.trim().length > 0 && !isSubmitting && !uploads.isUploading;

  const submit = async () => {
    if (!canSubmit) return;
    setIsSubmitting(true);
    try {
      await onSubmit({ body: body.trim(), health });
    } catch {
      return;
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Card className="flex flex-col gap-3 p-4">
      <div role="radiogroup" aria-label="Health" className="flex flex-wrap items-center gap-1">
        {HEALTHS.map((option) => (
          <FilterChip
            key={option}
            role="radio"
            aria-checked={health === option}
            selected={health === option}
            forceMinSize={false}
            onClick={() => setHealth(option)}
          >
            <HealthIcon health={option} />
            {healthLabels[option]}
          </FilterChip>
        ))}
      </div>
      <MarkdownEditor
        value={body}
        onChange={setBody}
        users={users}
        autoFocus
        placeholder="What changed, what's next, and what's in the way?"
        label="Update"
        onSubmit={() => void submit()}
        {...uploads.editorProps}
      />
      <div className="flex items-center justify-end gap-2">
        {onCancel && (
          <Button variant="ghost" size="sm" onClick={onCancel}>
            Cancel
          </Button>
        )}
        <Button
          size="sm"
          disabled={!canSubmit}
          isPending={isSubmitting}
          onClick={() => void submit()}
        >
          {submitLabel}
        </Button>
      </div>
    </Card>
  );
}
