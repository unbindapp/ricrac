import { type TTextPatch, textPatch } from "@/lib/helpers/text-patch";
import { isApiError } from "@/lib/server/errors";

/** An edit of a description or document body: the server's text it started from, and the new one. */
export type TTextEdit = { before: string; after: string };

/**
 * Saves the edit as a patch, so changes others made elsewhere in the text meanwhile stay; when
 * the patch no longer applies, `save(null)` saves the whole text instead.
 */
export async function savePatchOrText<T>(
  { before, after }: TTextEdit,
  save: (patch: TTextPatch | null) => Promise<T>,
) {
  const patch = textPatch(before, after);
  if (!patch) return save(null);
  try {
    return await save(patch);
  } catch (error) {
    if (!isApiError(error, 400)) throw error;
    return save(null);
  }
}
