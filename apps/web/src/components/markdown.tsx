import { cn } from "cn"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"

/** Markdown as agents write it (GitHub flavored). Raw HTML is shown as text, never rendered. */
export function Markdown({ children, className }: { children: string; className?: string }) {
  return (
    <div
      className={cn(
        "prose prose-sm prose-neutral max-w-none dark:prose-invert prose-headings:font-semibold prose-a:break-words prose-pre:bg-muted prose-pre:text-foreground prose-code:before:content-none prose-code:after:content-none",
        className,
      )}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        components={{
          a: ({ node: _node, ...props }) => <a {...props} target="_blank" rel="noreferrer noopener" />,
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  )
}
