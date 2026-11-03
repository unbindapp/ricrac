import { useCallback } from "react";

type TInfiniteList = {
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => Promise<unknown>;
};

/** Fetches the next page of an infinite query unless one is on its way or there is none. */
export function useLoadMore({ hasNextPage, isFetchingNextPage, fetchNextPage }: TInfiniteList) {
  return useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);
}
