import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { type TWorkspaceRole, workspaceRoleLabels } from "@/lib/queries/workspaces";

type TProps<R extends TWorkspaceRole> = {
  value: R;
  roles: R[];
  onChange: (role: R) => void;
  disabled?: boolean;
  label: string;
};

export function roleItems<R extends TWorkspaceRole>(roles: R[]) {
  return roles.map((role) => ({ value: role, label: workspaceRoleLabels[role] }));
}

export function RoleSelect<R extends TWorkspaceRole>({
  value,
  roles,
  onChange,
  disabled,
  label,
}: TProps<R>) {
  const items = roleItems(roles);
  return (
    <Select
      items={items}
      value={value}
      disabled={disabled}
      onValueChange={(next) => next && onChange(next as R)}
    >
      <SelectTrigger size="sm" aria-label={label} className="w-28">
        <SelectValue />
      </SelectTrigger>
      <SelectContent align="end">
        {items.map((item) => (
          <SelectItem key={item.value} value={item.value}>
            {item.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
