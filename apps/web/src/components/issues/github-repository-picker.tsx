import { useQuery } from "@tanstack/react-query";

import { GithubIcon } from "@/components/icons/github";
import { PickerTrigger } from "@/components/issues/pickers";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { issueRepositories } from "@/lib/helpers/github-sync";
import { githubRepoSyncsQuery } from "@/lib/queries/github";

export const NOT_ON_GITHUB = "none";

type TProps = {
  teamId: string;
  projectId: string | undefined;
  /** The team opens new issues on GitHub unless they say otherwise. */
  opensByDefault: boolean;
  /** The repository picked (owner/name or NOT_ON_GITHUB); undefined leaves it to the server. */
  value: string | undefined;
  onChange: (value: string | undefined) => void;
};

/** Which synced repository a new issue opens in on GitHub, if any. */
export function GithubRepositoryPicker({
  teamId,
  projectId,
  opensByDefault,
  value,
  onChange,
}: TProps) {
  const { workspace } = useWorkspace();
  const { data } = useQuery(githubRepoSyncsQuery(workspace.id));
  const repositories = issueRepositories(
    (data ?? []).filter((repoSync) => repoSync.team.id === teamId),
    projectId,
  );
  if (repositories.length === 0) return null;

  const fallback = opensByDefault ? repositories[0].repository.fullName : NOT_ON_GITHUB;
  const current = value ?? fallback;
  const isOff = current === NOT_ON_GITHUB;
  const options: TPickerOption[] = [
    ...repositories.map(({ repository }) => ({
      value: repository.fullName,
      label: repository.fullName,
      icon: <GithubIcon className="text-muted-foreground" />,
    })),
    {
      value: NOT_ON_GITHUB,
      label: "Not on GitHub",
      icon: <GithubIcon className="text-muted-foreground opacity-50" />,
    },
  ];
  return (
    <PropertyPicker
      label="Open on GitHub in"
      options={options}
      value={[current]}
      onChange={([next]) => onChange(next === fallback ? undefined : next)}
      trigger={
        <PickerTrigger
          appearance="compact"
          label={isOff ? "Not on GitHub" : `Opens on GitHub in ${current}`}
          icon={<GithubIcon className="text-muted-foreground" />}
          text={isOff ? "Not on GitHub" : current.split("/").at(-1)}
          muted={isOff}
        />
      }
    />
  );
}
