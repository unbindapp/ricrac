import { useQuery } from "@tanstack/react-query";
import { type ReactElement } from "react";

import type { CustomerStatusRef, CustomerTierRef, UserRef } from "@/api/ops.gen";
import { OptionDot } from "@/components/customers/customer-badges";
import { CircleDashedIcon, CircleUserIcon, LayersIcon } from "@/components/icons/symbols.gen";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { Avatar } from "@/components/shared/avatar";
import { PropertyTrigger } from "@/components/shared/property-trigger";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { customerStatusesQuery, customerTiersQuery } from "@/lib/queries/customers";
import { usersQuery } from "@/lib/queries/users";

const NONE = "none";

type TOption = { id: string; name: string; color: string };

type TOptionPickerProps<T extends TOption> = {
  value: T | null;
  disabled?: boolean;
  onChange: (id: string | null) => void;
  trigger?: ReactElement;
};

function optionItems(options: TOption[], noneLabel: string): TPickerOption[] {
  return [
    {
      value: NONE,
      label: noneLabel,
      icon: <CircleDashedIcon className="text-muted-foreground size-4" />,
    },
    ...options.map((option) => ({
      value: option.id,
      label: option.name,
      icon: <OptionDot color={option.color} className="mx-0.5" />,
    })),
  ];
}

export function CustomerStatusPicker({
  value,
  disabled,
  onChange,
  trigger,
}: TOptionPickerProps<CustomerStatusRef>) {
  const { workspace } = useWorkspace();
  const { data = [], isPending } = useQuery(customerStatusesQuery(workspace.id));
  return (
    <PropertyPicker
      label="Status"
      options={optionItems(data, "No status")}
      value={[value?.id ?? NONE]}
      isPending={isPending}
      disabled={disabled}
      onChange={([id]) => onChange(id === NONE ? null : id)}
      trigger={
        trigger ?? (
          <PropertyTrigger
            label={value ? `Status: ${value.name}` : "Set status"}
            icon={
              value ? (
                <OptionDot color={value.color} className="mx-0.5" />
              ) : (
                <CircleDashedIcon className="size-4" />
              )
            }
            text={value?.name ?? "No status"}
            muted={!value}
          />
        )
      }
    />
  );
}

export function CustomerTierPicker({
  value,
  disabled,
  onChange,
  trigger,
}: TOptionPickerProps<CustomerTierRef>) {
  const { workspace } = useWorkspace();
  const { data = [], isPending } = useQuery(customerTiersQuery(workspace.id));
  return (
    <PropertyPicker
      label="Tier"
      options={optionItems(data, "No tier")}
      value={[value?.id ?? NONE]}
      isPending={isPending}
      disabled={disabled}
      emptyText="No tiers yet. Add them in settings."
      onChange={([id]) => onChange(id === NONE ? null : id)}
      trigger={
        trigger ?? (
          <PropertyTrigger
            label={value ? `Tier: ${value.name}` : "Set tier"}
            icon={
              value ? (
                <OptionDot color={value.color} className="mx-0.5" />
              ) : (
                <LayersIcon className="size-4" />
              )
            }
            text={value?.name ?? "No tier"}
            muted={!value}
          />
        )
      }
    />
  );
}

export function CustomerOwnerPicker({
  value,
  disabled,
  onChange,
}: {
  value: UserRef | null;
  disabled?: boolean;
  onChange: (id: string | null) => void;
}) {
  const { workspace } = useWorkspace();
  const { data: users = [], isPending } = useQuery(usersQuery(workspace.id));
  const options: TPickerOption[] = [
    {
      value: NONE,
      label: "No owner",
      icon: <CircleUserIcon className="text-muted-foreground size-4" />,
    },
    ...users.map((user) => ({
      value: user.id,
      label: user.name,
      keywords: [user.email, user.displayName],
      icon: <Avatar id={user.id} name={user.name} src={user.avatarUrl} size="xs" />,
    })),
  ];
  return (
    <PropertyPicker
      label="Owner"
      options={options}
      value={[value?.id ?? NONE]}
      isPending={isPending}
      disabled={disabled}
      onChange={([id]) => onChange(id === NONE ? null : id)}
      trigger={
        <PropertyTrigger
          label={value ? `Owner: ${value.name}` : "Set owner"}
          icon={
            value ? (
              <Avatar id={value.id} name={value.name} size="xs" />
            ) : (
              <CircleUserIcon className="size-4" />
            )
          }
          text={value?.name ?? "No owner"}
          muted={!value}
        />
      }
    />
  );
}
