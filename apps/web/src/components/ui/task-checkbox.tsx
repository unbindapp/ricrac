/** A task list item's checkbox in rendered Markdown, styled by `.rr-prose`; read-only without onToggle. */
export function TaskCheckbox({ checked, onToggle }: { checked: boolean; onToggle?: () => void }) {
  return (
    <input
      type="checkbox"
      checked={checked}
      disabled={!onToggle}
      aria-label={checked ? "Done task" : "Open task"}
      onChange={() => onToggle?.()}
      className="focus-visible:ring-primary/8-10 cursor-pointer outline-none focus-visible:ring-2 disabled:cursor-default"
    />
  );
}
