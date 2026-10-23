type TProps = {
  name: string;
  color?: string | null;
};

export function LabelChip({ name, color }: TProps) {
  return (
    <span className="bg-background text-foreground dank:h-8 dank:gap-2 dank:rounded-sm dank:border-m3-outline-variant dank:bg-transparent dank:pr-3 dank:pl-2 dank:type-label-large dank:text-m3-on-surface-variant inline-flex h-5 max-w-40 shrink-0 items-center gap-1.5 rounded-full border px-2 text-xs leading-none">
      <span
        aria-hidden
        className="bg-muted-foreground dank:mx-1 dank:size-2.5 size-2 shrink-0 rounded-full"
        style={color ? { backgroundColor: color } : undefined}
      />
      <span className="truncate">{name}</span>
    </span>
  );
}
