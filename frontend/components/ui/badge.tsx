import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "cn";

const badgeVariants = cva(
  "group/badge inline-flex h-5 w-fit shrink-0 items-center justify-center gap-1 rounded-[2px] border px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.04em] uppercase whitespace-nowrap transition-colors [&>svg]:pointer-events-none [&>svg]:size-3",
  {
    variants: {
      variant: {
        default: "border-primary bg-primary text-primary-foreground",
        signal: "border-signal/30 bg-signal-tint text-signal-ink",
        ready: "border-status-ready/30 bg-status-ready/10 text-status-ready",
        warn: "border-status-warn/30 bg-status-warn/10 text-status-warn",
        destructive: "border-status-error/30 bg-status-error/10 text-status-error",
        secondary: "border-border bg-muted text-ink-secondary",
        outline: "border-border bg-transparent text-ink-secondary",
      },
    },
    defaultVariants: {
      variant: "outline",
    },
  }
);

function Badge({
  className,
  variant = "outline",
  render,
  ...props
}: useRender.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return useRender({
    defaultTagName: "span",
    props: mergeProps<"span">(
      {
        className: cn(badgeVariants({ variant }), className),
      },
      props
    ),
    render,
    state: {
      slot: "badge",
      variant,
    },
  });
}

export { Badge, badgeVariants };
