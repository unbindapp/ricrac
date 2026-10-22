import { ArchiveIcon } from "@/components/icons/symbols.gen";
import { Button } from "@/components/ui/button";

type TProps = {
  /** project, initiative */
  noun: string;
  isPending?: boolean;
  onUnarchive?: () => void;
};

export function ArchivedBanner({ noun, isPending, onUnarchive }: TProps) {
  return (
    <div role="status" className="bg-muted flex items-center gap-2 rounded-lg border p-4 text-sm">
      <ArchiveIcon className="text-muted-foreground size-4 shrink-0" />
      <p className="min-w-0 flex-1">This {noun} is archived.</p>
      {onUnarchive && (
        <Button variant="outline" size="sm" isPending={isPending} onClick={onUnarchive}>
          Unarchive
        </Button>
      )}
    </div>
  );
}
