"use client";

import { motion } from "motion/react";
import { AudioLines, Film } from "lucide-react";
import { cn } from "cn";
import type { MediaType } from "@/lib/schemas";

interface ModeSwitchProps {
  value: MediaType;
  onChange: (mode: MediaType) => void;
  layoutGroupId: string;
  size?: "sm" | "default";
  className?: string;
}

const OPTIONS: Array<{ value: MediaType; label: string }> = [
  { value: "video", label: "Video" },
  { value: "audio", label: "Audio" },
];

export function ModeSwitch({
  value,
  onChange,
  layoutGroupId,
  size = "default",
  className,
}: ModeSwitchProps) {
  return (
    <div
      role="radiogroup"
      aria-label="Output format type"
      className={cn(
        "inline-flex shrink-0 items-center rounded-[4px] border border-border bg-muted p-[3px]",
        className
      )}
    >
      {OPTIONS.map((opt) => {
        const active = value === opt.value;
        const Icon = opt.value === "video" ? Film : AudioLines;

        return (
          <button
            key={opt.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(opt.value)}
            className={cn(
              "relative inline-flex cursor-pointer items-center justify-center gap-1.5 rounded-[2px] font-medium transition-colors select-none focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring",
              size === "sm"
                ? "min-h-9 sm:min-h-7 px-2.5 text-xs"
                : "min-h-10 sm:min-h-8 px-3 text-xs",
              active
                ? "text-foreground"
                : "text-ink-secondary hover:text-foreground"
            )}
          >
            {active && (
              <motion.span
                layoutId={layoutGroupId}
                transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                className="absolute inset-0 rounded-[2px] border border-border bg-card shadow-[0_1px_2px_rgba(22,21,19,0.06)]"
              />
            )}
            <Icon className="relative z-10 size-3.5 stroke-[1.6]" />
            <span className="relative z-10">{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
