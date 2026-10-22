import { TriangleAlertIcon } from "lucide-react";
import type { CreatedAPIKey } from "@/api/account.gen";
import { CopyField } from "@/components/shared/copy-button";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type TProps = {
  created: CreatedAPIKey | null;
  onClose: () => void;
};

export function ApiKeyCreatedDialog({ created, onClose }: TProps) {
  return (
    <Dialog open={created !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent hideXButton className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>API key created</DialogTitle>
          <DialogDescription>
            {created ? `${created.key.label} is ready. ` : ""}Send it as a bearer token to /v1 or
            /mcp.
          </DialogDescription>
        </DialogHeader>
        <CopyField value={created?.secret ?? ""} label="API key" />
        <div className="bg-warning/3-10 text-warning flex items-start gap-1.5 rounded-md px-3 py-2 text-sm">
          <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" />
          <p>Copy the key now. Ricrac only stores a hash of it and can't show it again.</p>
        </div>
        <DialogFooter>
          <DialogClose render={<Button size="sm">Done</Button>} />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
