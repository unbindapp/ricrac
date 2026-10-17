import { useQuery } from "@tanstack/react-query";
import { PlusIcon } from "lucide-react";

import { CustomerNeedDialog } from "@/components/customers/customer-need-dialog";
import { CustomerNeedList } from "@/components/customers/customer-need-list";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SectionTitle } from "@/components/layout/section-title";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { customerNeedsQuery } from "@/lib/queries/customers";

type TProps = { issue: string; project?: never } | { project: string; issue?: never };

/**
 * The customer requests of an issue (identifier or id) or a project (slug or id), for their
 * detail pages: <CustomerNeedsSection issue={issue.identifier} />.
 */
export function CustomerNeedsSection(props: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const scope = props.issue ? { issue: props.issue } : { project: props.project ?? "" };
  const {
    data: needs = [],
    isPending,
    error,
    refetch,
  } = useQuery(customerNeedsQuery(workspace.id, scope));
  const important = needs.filter((need) => need.priority === 1).length;
  const summary =
    needs.length === 0
      ? undefined
      : `${needs.length} ${needs.length === 1 ? "request" : "requests"}${important ? `, ${important} important` : ""}`;
  if (!canEdit && !error && needs.length === 0) return null;

  return (
    <section aria-label="Customer requests" className="flex w-full flex-col gap-3">
      <SectionTitle
        title="Customer requests"
        description={summary}
        actions={
          canEdit && (
            <CustomerNeedDialog
              scope={scope}
              trigger={
                <Button variant="ghost" size="sm">
                  <PlusIcon className="size-4" />
                  Add request
                </Button>
              }
            />
          )
        }
      />
      {isPending && <ListSkeleton rows={2} />}
      {!isPending && error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {!isPending && !error && needs.length === 0 && (
        <p className="text-muted-foreground text-sm">
          No customer has asked for this yet. Add a request to keep track of who needs it.
        </p>
      )}
      {needs.length > 0 && <CustomerNeedList needs={needs} scope={scope} show="customer" />}
    </section>
  );
}
