import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { useDebounceValue } from "usehooks-ts";

import type { Customer } from "@/api/ops.gen";
import { CustomerLogo, OptionDot, OptionLabel } from "@/components/customers/customer-badges";
import { CustomerDialog } from "@/components/customers/customer-dialog";
import { BuildingIcon, ListFilterIcon, PlusIcon, SearchXIcon } from "@/components/icons";
import { PropertyPicker } from "@/components/issues/property-picker";
import { CreateButton } from "@/components/layout/create-button";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { Avatar } from "@/components/shared/avatar";
import { Button } from "@/components/ui/button";
import { segmentOf } from "@/components/shared/list-segment";
import { SearchField } from "@/components/ui/search-field";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatRevenue, formatSize } from "@/lib/helpers/customers";
import { customersQuery, customerStatusesQuery, customerTiersQuery } from "@/lib/queries/customers";

const ALL = "all";
const SEARCH_DEBOUNCE_MS = 250;

export function CustomersView() {
  const { workspace, canEdit } = useWorkspace();
  const [search, setSearch] = useState("");
  const [query] = useDebounceValue(search.trim(), SEARCH_DEBOUNCE_MS);
  const [status, setStatus] = useState(ALL);
  const [tier, setTier] = useState(ALL);
  const filters = {
    query: query || undefined,
    status: status === ALL ? undefined : status,
    tier: tier === ALL ? undefined : tier,
  };
  const isFiltered = Boolean(filters.query || filters.status || filters.tier);
  const {
    data: customers = [],
    isPending,
    error,
    refetch,
  } = useQuery(customersQuery(workspace.id, filters));

  const newButton = (placement: "header" | "empty") =>
    canEdit && (
      <CustomerDialog
        trigger={
          placement === "header" ? (
            <CreateButton label="New customer" />
          ) : (
            <Button variant="outline" size="sm">
              <PlusIcon className="size-4" />
              New customer
            </Button>
          )
        }
      />
    );

  return (
    <PageContainer>
      <PageHeader
        title="Customers"
        icon={<BuildingIcon />}
        actions={newButton("header")}
        toolbar={
          <PageToolbar>
            <SearchField
              icon={false}
              aria-label="Search customers"
              placeholder="Search by name or domain…"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
            <StatusFilter value={status} onChange={setStatus} />
            <TierFilter value={tier} onChange={setTier} />
          </PageToolbar>
        }
      />
      {isPending && <ListSkeleton />}
      {!isPending && error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {!isPending && !error && customers.length === 0 && !isFiltered && (
        <EmptyState
          icon={BuildingIcon}
          title="No customers yet"
          description="Customers tie the requests of the people you build for to issues and projects, so you can see who needs what."
          action={newButton("empty")}
        />
      )}
      {!isPending && !error && customers.length === 0 && isFiltered && (
        <EmptyState
          icon={SearchXIcon}
          title="No matching customers"
          description="Try another name, domain or filter."
        />
      )}
      {!isPending && !error && customers.length > 0 && <CustomerTable customers={customers} />}
    </PageContainer>
  );
}

function CustomerTable({ customers }: { customers: Customer[] }) {
  return (
    <div
      role="table"
      aria-label="Customers"
      className="dank:gap-0.5 dank:overflow-visible dank:rounded-none dank:border-0 flex w-full flex-col overflow-hidden rounded-lg border"
    >
      <div
        role="row"
        className="bg-card text-muted-foreground dank:type-label-medium dank:border-0 dank:bg-transparent dank:px-4 flex h-9 items-center gap-3 border-b px-3 text-xs font-medium"
      >
        <span role="columnheader" className="min-w-0 flex-1">
          Name
        </span>
        <span role="columnheader" className="hidden w-32 md:block">
          Status
        </span>
        <span role="columnheader" className="hidden w-28 lg:block">
          Tier
        </span>
        <span role="columnheader" className="hidden w-36 lg:block">
          Owner
        </span>
        <span role="columnheader" className="hidden w-20 text-right sm:block">
          Revenue
        </span>
        <span role="columnheader" className="hidden w-20 text-right md:block">
          Employees
        </span>
        <span role="columnheader" className="w-20 text-right">
          Requests
        </span>
      </div>
      {customers.map((customer, index) => (
        <CustomerRow
          key={customer.id}
          customer={customer}
          segment={segmentOf(index, customers.length)}
        />
      ))}
    </div>
  );
}

function CustomerRow({ customer, segment }: { customer: Customer; segment?: string }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const open = () =>
    void navigate({
      to: "/$workspace/customer/$slug",
      params: { workspace: workspace.urlKey, slug: customer.slugId },
    });
  return (
    <div
      role="row"
      tabIndex={0}
      aria-label={customer.name}
      data-slot="list-item"
      data-segment={segment}
      onClick={open}
      onKeyDown={(event) => {
        if (event.key === "Enter") open();
      }}
      className="has-hover:hover:bg-accent focus-visible:ring-primary/8-10 flex h-9 cursor-default items-center gap-3 border-b px-3 text-sm outline-hidden last:border-b-0 focus-visible:ring-1 focus-visible:ring-inset"
    >
      <span role="cell" className="flex min-w-0 flex-1 items-center gap-2">
        <CustomerLogo id={customer.id} name={customer.name} logoUrl={customer.logoUrl} />
        <span className="dank:font-normal truncate font-medium">{customer.name}</span>
        {customer.domains[0] && (
          <span className="text-muted-foreground dank:text-sm hidden truncate text-xs sm:inline">
            {customer.domains[0]}
          </span>
        )}
        {customer.archivedAt && <span className="text-muted-foreground text-xs">Archived</span>}
      </span>
      <Cell className="hidden w-32 md:flex">
        {customer.status && (
          <OptionLabel name={customer.status.name} color={customer.status.color} />
        )}
      </Cell>
      <Cell className="hidden w-28 lg:flex">
        {customer.tier && <OptionLabel name={customer.tier.name} color={customer.tier.color} />}
      </Cell>
      <Cell className="hidden w-36 lg:flex">
        {customer.owner && (
          <span className="inline-flex min-w-0 items-center gap-1.5">
            <Avatar id={customer.owner.id} name={customer.owner.name} size="xs" />
            <span className="truncate">{customer.owner.name}</span>
          </span>
        )}
      </Cell>
      <Cell className="hidden w-20 justify-end tabular-nums sm:flex">
        {formatRevenue(customer.revenue)}
      </Cell>
      <Cell className="hidden w-20 justify-end tabular-nums md:flex">
        {formatSize(customer.size)}
      </Cell>
      <Cell className="flex w-20 justify-end tabular-nums">
        {customer.needCount > 0 ? customer.needCount : ""}
      </Cell>
    </div>
  );
}

function Cell({ className, children }: { className: string; children: ReactNode }) {
  return (
    <span
      role="cell"
      className={cn("text-muted-foreground dank:text-sm min-w-0 items-center", className)}
    >
      {children || <span aria-hidden>–</span>}
    </span>
  );
}

type TFilterProps = { value: string; onChange: (value: string) => void };

function StatusFilter({ value, onChange }: TFilterProps) {
  const { workspace } = useWorkspace();
  const { data = [], isPending } = useQuery(customerStatusesQuery(workspace.id));
  return (
    <OptionFilter
      label="Status"
      allLabel="All statuses"
      options={data}
      isPending={isPending}
      value={value}
      onChange={onChange}
    />
  );
}

function TierFilter({ value, onChange }: TFilterProps) {
  const { workspace } = useWorkspace();
  const { data = [], isPending } = useQuery(customerTiersQuery(workspace.id));
  if (!isPending && data.length === 0) return null;
  return (
    <OptionFilter
      label="Tier"
      allLabel="All tiers"
      options={data}
      isPending={isPending}
      value={value}
      onChange={onChange}
    />
  );
}

function OptionFilter({
  label,
  allLabel,
  options,
  isPending,
  value,
  onChange,
}: TFilterProps & {
  label: string;
  allLabel: string;
  options: { id: string; name: string; color: string }[];
  isPending: boolean;
}) {
  const selected = options.find((option) => option.id === value);
  return (
    <PropertyPicker
      label={label}
      isPending={isPending}
      value={[value]}
      onChange={([next]) => onChange(next ?? ALL)}
      options={[
        { value: ALL, label: allLabel },
        ...options.map((option) => ({
          value: option.id,
          label: option.name,
          icon: <OptionDot color={option.color} className="mx-0.5" />,
        })),
      ]}
      trigger={
        <Button
          variant="outline"
          size="sm"
          aria-label={`${label}: ${selected?.name ?? allLabel}`}
          data-muted={!selected || undefined}
          className="data-muted:text-muted-foreground font-normal"
        >
          {selected ? <OptionDot color={selected.color} /> : <ListFilterIcon className="size-4" />}
          {selected?.name ?? allLabel}
        </Button>
      }
    />
  );
}
