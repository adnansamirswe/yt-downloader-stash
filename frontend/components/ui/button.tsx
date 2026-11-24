"use client";

import { Button as ButtonPrimitive } from "@base-ui/react/button";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "cn";

const buttonVariants = cva(
  "group/button inline-flex shrink-0 cursor-pointer items-center justify-center rounded-[4px] border border-transparent text-sm font-medium whitespace-nowrap transition-colors duration-150 select-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-45 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        signal:
          "bg-signal text-white border-signal hover:bg-signal-hover hover:border-signal-hover",
        default:
          "bg-primary text-primary-foreground border-primary hover:opacity-90",
        outline:
          "border-border bg-card text-foreground hover:border-border-strong hover:bg-muted/60",
        secondary:
          "border-border bg-muted text-foreground hover:border-border-strong",
        ghost:
          "text-ink-secondary hover:bg-muted hover:text-foreground",
        destructive:
          "border-status-error/30 bg-status-error/10 text-status-error hover:bg-status-error/15",
        link: "text-signal-ink underline-offset-4 hover:underline",
      },
      size: {
        default: "min-h-11 sm:min-h-9 h-11 sm:h-9 gap-2 px-3.5",
        sm: "min-h-10 sm:min-h-8 h-10 sm:h-8 gap-1.5 px-2.5 text-xs",
        xs: "min-h-9 sm:min-h-7 h-9 sm:h-7 gap-1 px-2 text-xs [&_svg:not([class*='size-'])]:size-3.5",
        lg: "min-h-11 h-11 gap-2 px-5 text-sm",
        icon: "size-11 sm:size-9",
        "icon-sm": "size-10 sm:size-8",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

function Button({
  className,
  variant = "default",
  size = "default",
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
