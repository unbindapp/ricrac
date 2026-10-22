import type { IssueRef } from "@/api/ops.gen";
import { CircleDotIcon, XIcon } from "@/components/icons";
import { NONE_KEY } from "@/components/issues/grouping";
import {
  IssueSearchPicker,
  PickerTrigger,
  type TPickerAppearance,
} from "@/components/issues/pickers";
import { StateIcon } from "@/components/shared/state-icon";

type TProps = {
  issue: IssueRef | null;
  onChange: (issue: IssueRef | null) => void;
  appearance?: TPickerAppearance;
  disabled?: boolean;
};

export function IssueLinkPicker({ issue, onChange, appearance = "compact", disabled }: TProps) {
  return (
    <IssueSearchPicker
      label="Link an issue"
      disabled={disabled}
      extraOptions={
        issue
          ? [
              {
                value: NONE_KEY,
                label: "Unlink issue",
                icon: <XIcon className="text-muted-foreground size-4" />,
              },
            ]
          : []
      }
      onSelect={onChange}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label={issue ? `Tracked by ${issue.identifier}` : "Link an issue"}
          muted={!issue}
          icon={
            issue ? (
              <StateIcon
                type={issue.state.type}
                color={issue.state.color}
                name={issue.state.name}
              />
            ) : (
              <CircleDotIcon className="text-muted-foreground size-4" />
            )
          }
          text={
            issue ? (
              <>
                <span className="text-muted-foreground font-mono text-xs">{issue.identifier}</span>{" "}
                {issue.title}
              </>
            ) : (
              "Link an issue"
            )
          }
        />
      }
    />
  );
}
