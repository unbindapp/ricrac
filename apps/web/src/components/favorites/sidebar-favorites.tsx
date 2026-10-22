import { closestCenter, DndContext, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { useQuery } from "@tanstack/react-query";
import { type MouseEvent, useRef } from "react";
import { FolderIcon, XIcon } from "lucide-react";

import { favoriteTarget } from "@/components/favorites/favorite-target";
import { reorderedPosition, sortableStyle, useSortSensors } from "@/components/favorites/sortable";
import { Button, LinkButton } from "@/components/ui/button";
import {
  SidebarCollapsible,
  sidebarItemClassName,
  SidebarSection,
} from "@/components/workspace/sidebar-nav";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  favoritesQuery,
  type TFavorite,
  useMoveFavoriteMutation,
  useRemoveFavoriteMutation,
} from "@/lib/queries/favorites";

type TProps = {
  onNavigate?: () => void;
};

/** The Favorites section of the sidebar; left out while there are none. */
export function SidebarFavorites({ onNavigate }: TProps) {
  const { workspace } = useWorkspace();
  const { data, error, refetch } = useQuery(favoritesQuery(workspace.id));
  if (error) {
    return (
      <SidebarSection title="Favorites">
        <Button
          variant="ghost"
          size="sm"
          forceMinSize={false}
          onClick={() => void refetch()}
          className={`${sidebarItemClassName} text-muted-foreground`}
        >
          Couldn't load favorites. Retry
        </Button>
      </SidebarSection>
    );
  }
  const shown = (data ?? []).filter((favorite) => favoriteTarget(workspace.urlKey, favorite));
  if (shown.length === 0) return null;

  const folders = [...new Set(shown.flatMap((favorite) => favorite.folder ?? []))];
  return (
    <SidebarSection title="Favorites">
      <FavoriteList favorites={shown.filter((f) => !f.folder)} onNavigate={onNavigate} />
      {folders.map((folder) => (
        <SidebarCollapsible
          key={folder}
          label={folder}
          icon={<FolderIcon className="text-muted-foreground size-4" />}
        >
          <FavoriteList
            favorites={shown.filter((favorite) => favorite.folder === folder)}
            onNavigate={onNavigate}
          />
        </SidebarCollapsible>
      ))}
    </SidebarSection>
  );
}

function FavoriteList({ favorites, onNavigate }: { favorites: TFavorite[] } & TProps) {
  const { workspace } = useWorkspace();
  const { mutate: move } = useMoveFavoriteMutation(workspace.id);
  const sensors = useSortSensors();
  const justDragged = useRef(false);
  if (favorites.length === 0) return null;

  const onDragEnd = (event: DragEndEvent) => {
    justDragged.current = true;
    const moved = reorderedPosition(favorites, event);
    const favorite = favorites.find((item) => item.id === moved?.id);
    if (!moved || !favorite) return;
    move({ favorite, sortOrder: moved.sortOrder });
  };
  const swallowClickAfterDrag = (event: MouseEvent) => {
    if (!justDragged.current) return;
    justDragged.current = false;
    event.preventDefault();
    event.stopPropagation();
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={() => (justDragged.current = false)}
      onDragEnd={onDragEnd}
    >
      <SortableContext
        items={favorites.map((favorite) => favorite.id)}
        strategy={verticalListSortingStrategy}
      >
        <div
          role="list"
          aria-label="Favorites"
          className="flex flex-col"
          onClickCapture={swallowClickAfterDrag}
        >
          {favorites.map((favorite) => (
            <FavoriteItem key={favorite.id} favorite={favorite} onNavigate={onNavigate} />
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}

function FavoriteItem({ favorite, onNavigate }: { favorite: TFavorite } & TProps) {
  const { workspace } = useWorkspace();
  const { mutate: remove, isPending } = useRemoveFavoriteMutation(workspace.id);
  const sortable = useSortable({ id: favorite.id });
  const target = favoriteTarget(workspace.urlKey, favorite);
  if (!target) return null;
  return (
    <div
      ref={sortable.setNodeRef}
      {...sortable.attributes}
      {...sortable.listeners}
      role="listitem"
      style={sortableStyle(sortable.transform, sortable.transition)}
      data-dragging={sortable.isDragging || undefined}
      className="group/favorite relative data-dragging:z-10 data-dragging:opacity-80"
    >
      <LinkButton
        {...target.link}
        activeOptions={{ exact: true, includeSearch: false }}
        variant="ghost"
        size="sm"
        forceMinSize={false}
        onClick={onNavigate}
        className={`${sidebarItemClassName} data-[status=active]:bg-accent pr-8`}
      >
        <span className="text-muted-foreground flex size-4 shrink-0 items-center justify-center">
          {target.icon}
        </span>
        <span className="min-w-0 flex-1 truncate text-left">{target.label}</span>
      </LinkButton>
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label={`Remove ${target.label} from favorites`}
        disabled={isPending}
        onClick={() => remove(favorite.id)}
        className="text-muted-foreground absolute top-1/2 right-1 size-6 -translate-y-1/2 opacity-0 group-focus-within/favorite:opacity-100 group-hover/favorite:opacity-100"
      >
        <XIcon className="size-3.5" />
      </Button>
    </div>
  );
}
