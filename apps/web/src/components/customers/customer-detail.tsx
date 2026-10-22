import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import type { CustomerDetail as TCustomerDetail } from "@/api/ops.gen";
import { CustomerLogo } from "@/components/customers/customer-badges";
import { CustomerDialog } from "@/components/customers/customer-dialog";
import { CustomerNeedDialog } from "@/components/customers/customer-need-dialog";
import { CustomerNeedList } from "@/components/customers/customer-need-list";
import {
  CustomerOwnerPicker,
  CustomerStatusPicker,
  CustomerTierPicker,
} from "@/components/customers/customer-pickers";
import {
  ArchiveIcon,
  BuildingIcon,
  EllipsisIcon,
  MessageSquareQuoteIcon,
  PencilIcon,
  PlusIcon,
} from "@/components/icons/symbols.gen";
import { PageBar } from "@/components/layout/page-header";
import { Breadcrumb } from "@/components/layout/breadcrumb";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PropertyRow } from "@/components/layout/property-row";
import { SectionTitle } from "@/components/layout/section-title";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { InlineText } from "@/components/shared/inline-text";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatRevenue, formatSize } from "@/lib/helpers/customers";
import {
  customerNameMaxLength,
  customerQuery,
  useDeleteCustomerMutation,
  useSaveCustomerMutation,
} from "@/lib/queries/customers";
import { isNotFoundError } from "@/lib/server/errors";

export function CustomerDetail({ slug }: { slug: string }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(customerQuery(workspace.id, slug));
  if (isPending) {
    return (
      <PageContainer>
        <ListSkeleton />
      </PageContainer>
    );
  }
  if (isNotFoundError(error)) {
    return (
      <PageContainer>
        <EntityNotFound
          Icon={BuildingIcon}
          description="Customer not found. It was deleted, or the link is wrong."
        />
      </PageContainer>
    );
  }
  if (error) {
    return (
      <PageContainer>
        <ErrorCard message={error.message} onRetry={() => void refetch()} />
      </PageContainer>
    );
  }
  return <CustomerPage customer={data} />;
}

function CustomerPage({ customer }: { customer: TCustomerDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save } = useSaveCustomerMutation(workspace.id);
  const scope = { customer: customer.id };
  const params = { workspace: workspace.urlKey };

  return (
    <PageContainer>
      <PageBar>
        <div className="min-w-0 flex-1">
          <Breadcrumb
            items={[
              {
                label: "Customers",
                icon: <BuildingIcon className="size-4" />,
                link: { to: "/$workspace/customers", params },
              },
              { label: customer.name },
            ]}
          />
        </div>
        {canEdit && <CustomerActions customer={customer} />}
      </PageBar>
      <div className="flex w-full flex-col gap-6 lg:flex-row">
        <div className="flex min-w-0 flex-1 flex-col gap-6">
          <div className="flex items-center gap-3">
            <CustomerLogo
              id={customer.id}
              name={customer.name}
              logoUrl={customer.logoUrl}
              size="lg"
            />
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <InlineText
                label="Customer name"
                value={customer.name}
                maxLength={customerNameMaxLength}
                readOnly={!canEdit}
                onSave={(name) => save({ id: customer.id, name })}
              />
              <p className="text-muted-foreground truncate text-sm">
                {customer.domains.length > 0 ? customer.domains.join(", ") : "No domains"}
              </p>
            </div>
            {customer.archivedAt && <Badge variant="muted">Archived</Badge>}
          </div>
          <section aria-label="Requests" className="flex flex-col gap-3">
            <SectionTitle
              title="Requests"
              description="What they asked for, newest first."
              actions={
                canEdit &&
                customer.needs.length > 0 && (
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
            {customer.needs.length === 0 ? (
              <EmptyState
                icon={MessageSquareQuoteIcon}
                title="No requests yet"
                description="Record what this customer asks for and link it to the issue or project that delivers it."
                action={
                  canEdit && (
                    <CustomerNeedDialog
                      scope={scope}
                      trigger={
                        <Button variant="outline" size="sm">
                          <PlusIcon className="size-4" />
                          Add request
                        </Button>
                      }
                    />
                  )
                }
              />
            ) : (
              <CustomerNeedList needs={customer.needs} scope={scope} show="target" />
            )}
          </section>
        </div>
        <aside aria-label="Properties" className="w-full shrink-0 lg:w-72">
          <div className="flex flex-col gap-1 rounded-lg border p-4 lg:sticky lg:top-0">
            <CustomerProperties customer={customer} />
          </div>
        </aside>
      </div>
    </PageContainer>
  );
}

function CustomerProperties({ customer }: { customer: TCustomerDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save } = useSaveCustomerMutation(workspace.id);
  const disabled = !canEdit;
  const muted = (text: string) => <span className="text-muted-foreground px-2.5">{text}</span>;
  return (
    <>
      <PropertyRow label="Status">
        <CustomerStatusPicker
          value={customer.status}
          disabled={disabled}
          onChange={(status) => save({ id: customer.id, status })}
        />
      </PropertyRow>
      <PropertyRow label="Tier">
        <CustomerTierPicker
          value={customer.tier}
          disabled={disabled}
          onChange={(tier) => save({ id: customer.id, tier })}
        />
      </PropertyRow>
      <PropertyRow label="Owner">
        <CustomerOwnerPicker
          value={customer.owner}
          disabled={disabled}
          onChange={(owner) => save({ id: customer.id, owner })}
        />
      </PropertyRow>
      <PropertyRow label="Revenue">
        {customer.revenue === null ? (
          muted("Not set")
        ) : (
          <span className="px-2.5 tabular-nums">{formatRevenue(customer.revenue)} a year</span>
        )}
      </PropertyRow>
      <PropertyRow label="Employees">
        {customer.size === null ? (
          muted("Not set")
        ) : (
          <span className="px-2.5 tabular-nums">{formatSize(customer.size)}</span>
        )}
      </PropertyRow>
      <PropertyRow label="Requests">
        <span className="px-2.5 tabular-nums">{customer.needCount}</span>
      </PropertyRow>
      {customer.externalIds.length > 0 && (
        <PropertyRow label="External ids">
          <span className="truncate px-2.5 font-mono text-xs">
            {customer.externalIds.join(", ")}
          </span>
        </PropertyRow>
      )}
      <PropertyRow label="Created">
        <span className="text-muted-foreground px-2.5">
          <RelativeTime value={customer.createdAt} />
        </span>
      </PropertyRow>
    </>
  );
}

function CustomerActions({ customer }: { customer: TCustomerDetail }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync: archive } = useDeleteCustomerMutation(workspace.id);
  const [isArchiveOpen, setIsArchiveOpen] = useState(false);
  return (
    <>
      <CustomerDialog
        customer={customer}
        trigger={
          <Button variant="outline" size="sm">
            <PencilIcon className="size-4" />
            Edit
          </Button>
        }
      />
      {!customer.archivedAt && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon" aria-label="More actions">
                <EllipsisIcon className="size-4" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem className="text-destructive" onClick={() => setIsArchiveOpen(true)}>
              <ArchiveIcon className="size-4" />
              Archive customer
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <ConfirmDialog
        open={isArchiveOpen}
        onOpenChange={setIsArchiveOpen}
        title={`Archive ${customer.name}`}
        description="They leave the customer list. Their requests stay on the issues and projects they are linked to."
        confirmLabel="Archive customer"
        onConfirm={async () => {
          await archive(customer.id);
          await navigate({ to: "/$workspace/customers", params: { workspace: workspace.urlKey } });
        }}
      />
    </>
  );
}
