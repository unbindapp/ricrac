import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useTheme } from "next-themes";
import { type ReactNode } from "react";
import { z } from "zod";

import type { WorkspaceUserDetail } from "@/api/ops.gen";
import { MonitorIcon, MoonIcon, SunIcon } from "@/components/icons";
import { GithubAccountSection } from "@/components/integrations/github-account-section";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { DEFAULT_ACCENT_HUE, defaultTheme, type TTheme } from "@/components/providers/themes";
import { Avatar } from "@/components/shared/avatar";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { timezoneOptions } from "@/lib/helpers/timezones";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useUpdateUserMutation, workspaceMeQuery } from "@/lib/queries/users";
import { useAccentHueStore } from "@/lib/stores/accent-hue";
import { useUiTheme } from "@/themes/ui-theme-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/account/")({
  component: AccountProfileSettingsPage,
});

const timezones = timezoneOptions();

const themeOptions: { value: TTheme; label: string; icon: ReactNode }[] = [
  { value: "system", label: "System", icon: <MonitorIcon className="size-4" /> },
  { value: "light", label: "Light", icon: <SunIcon className="size-4" /> },
  { value: "dark", label: "Dark", icon: <MoonIcon className="size-4" /> },
];

const accentPresets = [
  { hue: DEFAULT_ACCENT_HUE, name: "Green" },
  { hue: 170, name: "Teal" },
  { hue: 210, name: "Blue" },
  { hue: 260, name: "Violet" },
  { hue: 320, name: "Pink" },
  { hue: 0, name: "Red" },
  { hue: 25, name: "Orange" },
  { hue: 50, name: "Yellow" },
];

const accentSwatch = (hue: number) => `hsl(${hue} 60% 50%)`;

const HUE_STOPS = 12;
const hueGradient = `linear-gradient(to right, ${Array.from({ length: HUE_STOPS + 1 }, (_, index) =>
  accentSwatch((index * 360) / HUE_STOPS),
).join(", ")})`;

const formSchema = z.object({
  name: z.string().trim().min(1, "Name is required."),
  displayName: z
    .string()
    .trim()
    .min(1, "Display name is required.")
    .regex(/^@?\S+$/, "Use one word, like ann."),
  timezone: z.string(),
});

function AccountProfileSettingsPage() {
  const { workspace } = useWorkspace();
  const { data: me, isPending, error, refetch } = useQuery(workspaceMeQuery(workspace.id));
  return (
    <SettingsPage title="Profile" description="How you appear to the people you work with.">
      {isPending && <ListSkeleton rows={3} />}
      {!isPending && error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {me && (
        <>
          <ProfileSection me={me} />
          <NotificationsSection me={me} />
          <GithubAccountSection />
          <AppearanceSection />
        </>
      )}
    </SettingsPage>
  );
}

function ProfileSection({ me }: { me: WorkspaceUserDetail }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: update } = useUpdateUserMutation(workspace.id);
  const form = useAppForm({
    defaultValues: { name: me.name, displayName: me.displayName, timezone: me.timezone ?? "" },
    validators: { onChange: formSchema },
    onSubmit: async ({ value, formApi }) => {
      const parsed = formSchema.parse(value);
      const user = await update({
        id: "me",
        name: parsed.name,
        displayName: parsed.displayName,
        timezone: parsed.timezone || null,
      });
      formApi.reset({
        name: user.name,
        displayName: user.displayName,
        timezone: user.timezone ?? "",
      });
    },
  });
  return (
    <SettingsSection
      title="Details"
      description="Your picture comes from your Google account, if you signed in with Google."
      onSubmit={submitHandler(form)}
      footer={
        <form.AppForm>
          <form.SubmitButton size="sm" requireDirty>
            Save
          </form.SubmitButton>
        </form.AppForm>
      }
    >
      <div className="flex items-center gap-3">
        <Avatar id={me.id} name={me.name} src={me.avatarUrl} size="xl" />
        <div className="flex min-w-0 flex-col">
          <p className="truncate text-sm font-medium">{me.name}</p>
          <p className="text-muted-foreground truncate text-sm">{me.email}</p>
        </div>
      </div>
      <form.AppField
        name="name"
        children={(field) => <field.TextField label="Full name" autoComplete="name" />}
      />
      <form.AppField
        name="displayName"
        children={(field) => (
          <field.TextField
            label="Display name"
            description="One word. People mention you with @ followed by it."
          />
        )}
      />
      <form.AppField
        name="timezone"
        children={(field) => (
          <field.SelectField
            label="Timezone"
            description="Shown to the people you work with, so they know your local time."
            placeholder="Not set"
            items={timezones}
          />
        )}
      />
    </SettingsSection>
  );
}

function NotificationsSection({ me }: { me: WorkspaceUserDetail }) {
  const { workspace } = useWorkspace();
  const { mutate: update, isPending } = useUpdateUserMutation(workspace.id);
  const isOn = me.emailNotifications ?? false;
  return (
    <SettingsSection
      title="Email notifications"
      description={`Unread notifications are emailed to ${me.email}.`}
      actions={
        <Switch
          aria-label="Email notifications"
          checked={isOn}
          disabled={isPending}
          onCheckedChange={(checked) => update({ id: "me", emailNotifications: checked })}
        />
      }
    />
  );
}

function AppearanceSection() {
  const { theme, setTheme } = useTheme();
  return (
    <SettingsSection title="Appearance" description="Saved in this browser.">
      <ThemeField key={theme} theme={theme ?? defaultTheme} onChange={setTheme} />
      <AccentField />
    </SettingsSection>
  );
}

function ThemeField({ theme, onChange }: { theme: string; onChange: (theme: string) => void }) {
  const form = useAppForm({ defaultValues: { theme } });
  return (
    <form.AppField
      name="theme"
      listeners={{ onChange: ({ value }) => onChange(value) }}
      children={(field) => <field.SelectField label="Theme" items={themeOptions} />}
    />
  );
}

function AccentField() {
  const { defaultAccentHue } = useUiTheme();
  const chosenHue = useAccentHueStore((state) => state.hue);
  const setHue = useAccentHueStore((state) => state.setHue);
  const hue = chosenHue ?? defaultAccentHue;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex min-h-8 items-center gap-2">
        <span className="flex-1 text-sm font-medium">Accent color</span>
        <span className="text-muted-foreground text-xs tabular-nums">{hue}°</span>
        {chosenHue !== null && (
          <Button variant="ghost" size="sm" onClick={() => setHue(null)}>
            Reset
          </Button>
        )}
      </div>
      <Slider
        min={0}
        max={359}
        value={hue}
        thumbLabel="Accent hue"
        trackStyle={{ background: hueGradient }}
        onValueChange={(value) => setHue(value)}
      />
      <div className="flex flex-wrap items-center gap-2">
        {accentPresets.map((preset) => (
          <Button
            key={preset.hue}
            variant={preset.hue === hue ? "outline" : "ghost"}
            size="icon"
            aria-label={preset.name}
            aria-pressed={preset.hue === hue}
            onClick={() => setHue(preset.hue)}
          >
            <span
              className="size-4 rounded-full"
              style={{ background: accentSwatch(preset.hue) }}
            />
          </Button>
        ))}
      </div>
      <div className="grid grid-cols-3 gap-2">
        <span className="bg-background flex h-9 items-center justify-center rounded-md border text-sm font-medium">
          Background
        </span>
        <span className="bg-muted flex h-9 items-center justify-center rounded-md text-sm font-medium">
          Muted
        </span>
        <span className="bg-top-loader text-primary-foreground flex h-9 items-center justify-center rounded-md text-sm font-medium">
          Accent
        </span>
      </div>
    </div>
  );
}
