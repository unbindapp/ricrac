import { GlobalSearch } from "@/components/search/global-search";

export function DankTopBar() {
  return (
    <header className="hidden h-12 shrink-0 items-center justify-center px-4 md:flex">
      <GlobalSearch />
    </header>
  );
}
