import { ReactNode } from "react";

import { TIcon } from "@/components/icons/symbols.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";

type TProps = {
  isPending: boolean;
  error: Error | null;
  isEmpty: boolean;
  onRetry?: () => void;
  empty: {
    icon: TIcon;
    title: string;
    description?: string;
    action?: ReactNode;
  } | null;
  skeletonRows?: number;
  skeleton?: ReactNode;
  children: ReactNode;
};

export function ListStates({
  isPending,
  error,
  isEmpty,
  onRetry,
  empty,
  skeletonRows,
  skeleton,
  children,
}: TProps) {
  if (isPending) return skeleton ?? <ListSkeleton rows={skeletonRows} />;
  if (error) return <ErrorCard message={error.message} onRetry={onRetry} />;
  if (isEmpty) return empty && <EmptyState {...empty} />;
  return children;
}
