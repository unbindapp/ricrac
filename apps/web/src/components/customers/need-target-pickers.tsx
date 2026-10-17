import { useQuery } from "@tanstack/react-query";
import { BoxIcon, BuildingIcon, CircleDotIcon } from "lucide-react";
import { type ReactNode, useState } from "react";
import { useDebounceValue } from "usehooks-ts";

import { CustomerLogo } from "@/components/customers/customer-badges";
import { PropertyPicker } from "@/components/issues/property-picker";
import { ProjectIcon } from "@/components/shared/project-icon";
import { StateIcon } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { customersQuery } from "@/lib/queries/customers";
import { issueOptionsQuery, projectOptionsQuery } from "@/lib/queries/entity-options";

const SEARCH_DEBOUNCE_MS = 200;

export type TPicked = { id: string; label: string };

type TProps = {
  id: string;
  value: TPicked | null;
  onChange: (value: TPicked) => void;
  invalid?: boolean;
};

function FieldButton({
  id,
  invalid,
  icon,
  text,
  placeholder,
  ...props
}: {
  id: string;
  invalid?: boolean;
  icon: ReactNode;
  text: string | undefined;
  placeholder: string;
}) {
  return (
    <Button
      id={id}
      type="button"
      variant="outline"
      aria-invalid={invalid || undefined}
      data-muted={!text || undefined}
      className="data-muted:text-muted-foreground aria-invalid:border-destructive w-full justify-start font-normal"
      {...props}
    >
      {icon}
      <span className="min-w-0 truncate">{text ?? placeholder}</span>
    </Button>
  );
}

/** Picks an issue, searching by title or identifier. */
export function IssueTargetPicker({ id, value, onChange, invalid }: TProps) {
  const { workspace } = useWorkspace();
  const [search, setSearch] = useState("");
  const [query] = useDebounceValue(search, SEARCH_DEBOUNCE_MS);
  const { data: issues = [], isFetching } = useQuery(issueOptionsQuery(workspace.id, query));
  return (
    <PropertyPicker
      label="Issue"
      searchPlaceholder="Search issues or type ENG-123…"
      emptyText="No matching issues"
      isPending={isFetching}
      onSearchChange={setSearch}
      value={value ? [value.id] : []}
      onChange={([picked]) => {
        const issue = issues.find((item) => item.id === picked);
        if (issue) onChange({ id: issue.id, label: `${issue.identifier} ${issue.title}` });
      }}
      options={issues.map((issue) => ({
        value: issue.id,
        label: issue.title,
        description: issue.identifier,
        icon: <StateIcon type={issue.state.type} color={issue.state.color} />,
      }))}
      trigger={
        <FieldButton
          id={id}
          invalid={invalid}
          icon={<CircleDotIcon className="size-4" />}
          text={value?.label}
          placeholder="Pick an issue"
        />
      }
    />
  );
}

export function ProjectTargetPicker({ id, value, onChange, invalid }: TProps) {
  const { workspace } = useWorkspace();
  const { data: projects = [], isPending } = useQuery(projectOptionsQuery(workspace.id));
  return (
    <PropertyPicker
      label="Project"
      emptyText="No projects yet"
      isPending={isPending}
      value={value ? [value.id] : []}
      onChange={([picked]) => {
        const project = projects.find((item) => item.id === picked);
        if (project) onChange({ id: project.id, label: project.name });
      }}
      options={projects.map((project) => ({
        value: project.id,
        label: project.name,
        icon: <ProjectIcon icon={project.icon} color={project.color} />,
      }))}
      trigger={
        <FieldButton
          id={id}
          invalid={invalid}
          icon={<BoxIcon className="size-4" />}
          text={value?.label}
          placeholder="Pick a project"
        />
      }
    />
  );
}

export function CustomerTargetPicker({ id, value, onChange, invalid }: TProps) {
  const { workspace } = useWorkspace();
  const { data: customers = [], isPending } = useQuery(customersQuery(workspace.id));
  return (
    <PropertyPicker
      label="Customer"
      emptyText="No customers yet"
      isPending={isPending}
      value={value ? [value.id] : []}
      onChange={([picked]) => {
        const customer = customers.find((item) => item.id === picked);
        if (customer) onChange({ id: customer.id, label: customer.name });
      }}
      options={customers.map((customer) => ({
        value: customer.id,
        label: customer.name,
        keywords: customer.domains,
        icon: (
          <CustomerLogo
            id={customer.id}
            name={customer.name}
            logoUrl={customer.logoUrl}
            size="sm"
          />
        ),
      }))}
      trigger={
        <FieldButton
          id={id}
          invalid={invalid}
          icon={<BuildingIcon className="size-4" />}
          text={value?.label}
          placeholder="Pick a customer"
        />
      }
    />
  );
}
