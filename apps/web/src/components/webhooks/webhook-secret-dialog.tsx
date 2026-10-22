import { TriangleAlertIcon } from "lucide-react";
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
  /** The new secret and whether it came with a new webhook; null keeps the dialog closed. */
  secret: { value: string; isNew: boolean } | null;
  onClose: () => void;
};

export function WebhookSecretDialog({ secret, onClose }: TProps) {
  return (
    <Dialog open={secret !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent hideXButton className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{secret?.isNew ? "Webhook created" : "New signing secret"}</DialogTitle>
          <DialogDescription>
            Check deliveries with this secret. It isn't shown again.
          </DialogDescription>
        </DialogHeader>
        <CopyField value={secret?.value ?? ""} label="Signing secret" />
        <p className="text-muted-foreground text-sm">
          Each delivery has a <span className="font-mono text-xs">Ricrac-Signature</span> header:
          the hex HMAC-SHA256 of the raw body, keyed with this secret. Compute it over the exact
          bytes you receive, compare in constant time, and refuse bodies whose{" "}
          <span className="font-mono text-xs">webhookTimestamp</span> is over a minute old.
        </p>
        <div className="bg-warning/3-10 text-warning flex items-start gap-1.5 rounded-md px-3 py-2 text-sm">
          <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" />
          <p>Copy it now. If you lose it, rotate the secret to get a new one.</p>
        </div>
        <DialogFooter>
          <DialogClose render={<Button size="sm">Done</Button>} />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
