"use client";

import { motion } from "motion/react";
import { cn } from "cn";

interface TelemetryRulerProps {
  /** 0 to 100 for determinate progress; omit or pass indeterminate=true for scanning sweep */
  percent?: number;
  indeterminate?: boolean;
  tone?: "signal" | "ready" | "error";
  leftLabel?: string;
  rightLabel?: string;
  compact?: boolean;
  className?: string;
}

export function TelemetryRuler({
  percent = 0,
  indeterminate = false,
  tone = "signal",
  leftLabel,
  rightLabel,
  compact = false,
  className,
}: TelemetryRulerProps) {
  const clamped = Math.max(0, Math.min(100, percent));

  const barToneClass =
    tone === "ready"
      ? "bg-status-ready"
      : tone === "error"
        ? "bg-status-error"
        : "bg-signal";

  return (
    <div className={cn("w-full", className)}>
      {(leftLabel || rightLabel) && (
        <div
          className={cn(
            "flex items-center justify-between gap-3 font-mono-tabular text-[11px] tracking-[0.03em] text-ink-secondary",
            compact ? "pb-1" : "pb-1.5"
          )}
        >
          <span className="truncate">{leftLabel}</span>
          <span className="shrink-0 text-foreground">{rightLabel}</span>
        </div>
      )}
      <div
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={indeterminate ? undefined : Math.round(clamped)}
        aria-label={leftLabel ?? "Transfer progress"}
        className={cn(
          "relative w-full overflow-hidden bg-border",
          compact ? "h-[2px]" : "h-[3px]"
        )}
      >
        {indeterminate ? (
          <motion.div
            className={cn("absolute inset-y-0 w-1/3", barToneClass)}
            initial={{ x: "-100%" }}
            animate={{ x: "300%" }}
            transition={{
              repeat: Infinity,
              duration: 1.35,
              ease: [0.22, 1, 0.36, 1],
            }}
          />
        ) : (
          <div
            className={cn(
              "h-full transition-[width] duration-150 ease-out",
              barToneClass
            )}
            style={{ width: `${clamped}%` }}
          />
        )}
      </div>
    </div>
  );
}
