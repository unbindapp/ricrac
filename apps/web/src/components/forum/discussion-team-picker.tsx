import { GlobeIcon } from "@/components/icons/symbols.gen";
import { NONE_KEY } from "@/components/issues/grouping";
import { PickerTrigger, type TPickerAppearance } from "@/components/issues/pickers";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { TeamIcon } from "@/components/shared/team-icon";
import { useTeams } from "@/components/workspace/teams-provider";

type TProps = {
  /** A team id, or null for no team. */
  value: string | null;
  onChange: (teamId: string | null) => void;
  /** What no team means here: "Whole workspace" on a discussion, "All teams" in a filter. */
  noneLabel: string;
  label: string;
  appearance?: TPickerAppearance;
  disabled?: boolean;
};

export function DiscussionTeamPicker({
  value,
  onChange,
  noneLabel,
  label,
  appearance = "compact",
  disabled,
}: TProps) {
  const { teams, isPending } = useTeams();
  const team = teams.find((item) => item.id === value);
  const options: TPickerOption[] = [
    {
      value: NONE_KEY,
      label: noneLabel,
      icon: <GlobeIcon className="text-muted-foreground size-4" />,
    },
    ...teams.map((item) => ({
      value: item.id,
      label: item.name,
      description: item.key,
      keywords: [item.key],
      icon: <TeamIcon teamKey={item.key} icon={item.icon} color={item.color} size="sm" />,
    })),
  ];
  return (
    <PropertyPicker
      label={label}
      options={options}
      value={[value ?? NONE_KEY]}
      isPending={isPending}
      disabled={disabled}
      onChange={([next]) => onChange(!next || next === NONE_KEY ? null : next)}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label={label}
          icon={
            team ? (
              <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />
            ) : (
              <GlobeIcon className="text-muted-foreground size-4" />
            )
          }
          text={team?.name ?? noneLabel}
        />
      }
    />
  );
}
