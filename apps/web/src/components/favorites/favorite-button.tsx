import { useQuery } from "@tanstack/react-query";
import { StarIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  favoritesQuery,
  findFavorite,
  type TFavoriteKind,
  useAddFavoriteMutation,
  useRemoveFavoriteMutation,
} from "@/lib/queries/favorites";

type TProps = {
  kind: TFavoriteKind;
  /** The id of what to favorite. */
  entityId: string;
};

export function FavoriteButton({ kind, entityId }: TProps) {
  const { workspace } = useWorkspace();
  const { data } = useQuery(favoritesQuery(workspace.id));
  const add = useAddFavoriteMutation(workspace.id);
  const remove = useRemoveFavoriteMutation(workspace.id);
  const favorite = findFavorite(data, kind, entityId);
  const label = favorite ? "Remove from favorites" : "Add to favorites";

  return (
    <SimpleTooltip content={label}>
      <Button
        variant="ghost"
        size="icon"
        aria-label={label}
        aria-pressed={!!favorite}
        disabled={!data || add.isPending || remove.isPending}
        onClick={() => (favorite ? remove.mutate(favorite.id) : add.mutate({ kind, id: entityId }))}
      >
        <StarIcon
          data-active={favorite ? true : undefined}
          className="data-active:text-warning size-4 data-active:fill-current"
        />
      </Button>
    </SimpleTooltip>
  );
}
