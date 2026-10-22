import type { OAuthAppSecret } from "@/api/account.gen";
import { TriangleAlertIcon } from "@/components/icons";
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

export function OAuthAppSecretDialog({
  secret,
  onClose,
}: {
  secret: OAuthAppSecret | null;
  onClose: () => void;
}) {
  return (
    <Dialog open={secret !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent hideXButton className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{secret ? `${secret.app.name} credentials` : "Credentials"}</DialogTitle>
          <DialogDescription>
            The app sends these to {window.location.origin}/oauth/token.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-medium">Client ID</p>
          <CopyField value={secret?.app.id ?? ""} label="Client ID" />
        </div>
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-medium">Client secret</p>
          <CopyField value={secret?.clientSecret ?? ""} label="Client secret" />
        </div>
        <div className="bg-warning/3-10 text-warning flex items-start gap-1.5 rounded-md px-3 py-2 text-sm">
          <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" />
          <p>Copy the secret now. Ricrac only stores a hash of it and can't show it again.</p>
        </div>
        <DialogFooter>
          <DialogClose render={<Button size="sm">Done</Button>} />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
