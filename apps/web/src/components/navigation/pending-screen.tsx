import { LoaderIcon } from "@/components/icons";
import { uiThemeOf } from "@/themes/registry";

export default function PendingScreen() {
  const { LoadingIndicator } = uiThemeOf(document.documentElement.dataset.uiTheme);
  if (LoadingIndicator) {
    return (
      <div className="flex w-full flex-1 items-center justify-center">
        <LoadingIndicator />
      </div>
    );
  }
  return (
    <div
      aria-busy
      aria-label="Loading"
      className="text-muted-foreground flex w-full flex-1 items-center justify-center"
    >
      <LoaderIcon className="size-5 animate-spin" />
    </div>
  );
}
