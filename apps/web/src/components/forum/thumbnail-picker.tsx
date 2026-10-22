import { ImageIcon, ImageUpIcon, XIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { FileTrigger } from "@/components/ui/file-trigger";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { imageEmbeds } from "@/lib/helpers/media";
import { imageUploadAccept, useUploadFileMutation } from "@/lib/queries/uploads";

const MAX_BODY_CHOICES = 6;

type TProps = {
  /** The chosen image; null falls back to the body's first image. */
  value: string | null;
  onChange: (url: string | null) => void;
  body: string;
};

export function ThumbnailPicker({ value, onChange, body }: TProps) {
  const { workspace } = useWorkspace();
  const { mutateAsync: upload, isPending } = useUploadFileMutation(workspace.id);
  const bodyImages = imageEmbeds(body).slice(0, MAX_BODY_CHOICES);
  const fallback = bodyImages[0] ?? null;
  const shown = value ?? fallback;

  return (
    <div className="flex flex-col gap-3 sm:flex-row">
      <div className="bg-muted text-muted-most-foreground flex aspect-video w-full shrink-0 items-center justify-center overflow-hidden rounded-md border sm:w-48">
        {shown ? (
          <img src={shown} alt="Thumbnail" className="size-full object-cover" />
        ) : (
          <ImageIcon className="size-5" />
        )}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <p className="text-muted-foreground text-sm">
          {value
            ? "Shown on the discussion's card in the forum."
            : fallback
              ? "Uses the first image in the body until you pick one."
              : "Upload an image, or add one to the body."}
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <FileTrigger
            accept={imageUploadAccept}
            onSelect={([file]) =>
              void upload(file)
                .then((prepared) => onChange(prepared.assetUrl))
                .catch(() => undefined)
            }
          >
            {(open) => (
              <Button
                type="button"
                variant="outline"
                size="sm"
                isPending={isPending}
                onClick={open}
              >
                <ImageUpIcon className="size-4" />
                Upload image
              </Button>
            )}
          </FileTrigger>
          {value && (
            <Button type="button" variant="ghost" size="sm" onClick={() => onChange(null)}>
              <XIcon className="size-4" />
              Remove
            </Button>
          )}
        </div>
        {bodyImages.length > (value ? 0 : 1) && (
          <div role="radiogroup" aria-label="Images in the body" className="flex flex-wrap gap-2">
            {bodyImages.map((url) => (
              <Button
                key={url}
                type="button"
                variant="outline"
                size="icon"
                role="radio"
                aria-checked={shown === url}
                aria-label="Use this image"
                className="aria-checked:border-primary aria-checked:ring-primary size-auto overflow-hidden p-0 aria-checked:ring-1"
                onClick={() => onChange(url)}
              >
                <img src={url} alt="" className="aspect-video w-16 object-cover" />
              </Button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
