import { useQuery } from "@tanstack/react-query";
import { useDebounceValue } from "usehooks-ts";

import { useWorkspace } from "@/components/workspace/workspace-provider";
import { searchQuery } from "@/lib/queries/search";

const SEARCH_DEBOUNCE_MS = 150;

/** Searches as you type, keeping the last results until the new ones arrive. */
export function useSearch(query: string) {
  const { workspace } = useWorkspace();
  const trimmed = query.trim();
  const [debounced] = useDebounceValue(trimmed, SEARCH_DEBOUNCE_MS);
  const { data, isFetching, error } = useQuery(searchQuery(workspace.id, debounced));
  return {
    hits: trimmed && debounced ? (data ?? []) : [],
    isPending: Boolean(trimmed) && (isFetching || debounced !== trimmed),
    error: trimmed ? error : null,
  };
}
