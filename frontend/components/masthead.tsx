"use client";

import { useSyncExternalStore } from "react";
import { useTheme } from "next-themes";
import {
  AlertTriangle,
  ArrowDownToLine,
  CheckCircle2,
  Moon,
  Sun,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import type { HealthResponse } from "@/lib/schemas";

interface MastheadProps {
  health: HealthResponse | null;
  healthError: boolean;
  activeTransfersCount: number;
  totalTransfersCount: number;
  onOpenTransfers: () => void;
  onResetWorkspace: () => void;
}

const emptySubscribe = () => () => {};

export function Masthead({
  health,
  healthError,
  activeTransfersCount,
  totalTransfersCount,
  onOpenTransfers,
  onResetWorkspace,
}: MastheadProps) {
  const { resolvedTheme, setTheme } = useTheme();
  const mounted = useSyncExternalStore(
    emptySubscribe,
    () => true,
    () => false
  );

  const isDark = mounted && resolvedTheme === "dark";

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/95">
      <div className="mx-auto flex h-14 max-w-[1120px] items-center justify-between gap-4 px-4 sm:px-6">
        <div className="flex items-center gap-4 min-w-0">
          <button
            type="button"
            onClick={onResetWorkspace}
            className="group inline-flex cursor-pointer items-baseline gap-0.5 rounded-[2px] py-1 text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            aria-label="Stash home"
          >
            <span className="font-display text-[2rem] leading-none tracking-[-0.02em] text-foreground">
              stash
            </span>
            <span className="font-display text-[2rem] leading-none text-signal">
              .
            </span>
          </button>

          <span
            aria-hidden="true"
            className="hidden h-4 w-px bg-border sm:inline-block"
          />

          <div
            className="hidden items-center gap-2 font-mono-tabular text-[11px] uppercase tracking-[0.05em] text-ink-secondary sm:flex"
            aria-live="polite"
          >
            {healthError ? (
              <>
                <AlertTriangle className="size-3.5 text-status-error stroke-[1.75]" />
                <span className="text-status-error">Engine Offline</span>
              </>
            ) : health ? (
              <>
                <CheckCircle2 className="size-3.5 text-status-ready stroke-[1.75]" />
                <span>
                  Ready
                  {health.ytDlpVersion ? ` · yt-dlp ${health.ytDlpVersion}` : ""}
                  {health.jsRuntime && health.jsRuntime !== "none"
                    ? ` · ${health.jsRuntime}`
                    : ""}
                </span>
              </>
            ) : (
              <>
                <span className="size-1.5 rounded-full bg-ink-muted" />
                <span>Connecting engine</span>
              </>
            )}
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={onOpenTransfers}
            className="relative font-mono-tabular text-xs"
            aria-label={`Open transfers drawer (${activeTransfersCount} active)`}
          >
            <ArrowDownToLine className="size-3.5 stroke-[1.6]" />
            <span>Transfers</span>
            {totalTransfersCount > 0 && (
              <span
                className={
                  activeTransfersCount > 0
                    ? "ml-0.5 inline-flex min-w-5 items-center justify-center rounded-[2px] bg-signal px-1.5 py-0.5 text-[10px] font-semibold text-white"
                    : "ml-0.5 inline-flex min-w-5 items-center justify-center rounded-[2px] bg-muted px-1.5 py-0.5 text-[10px] text-ink-secondary"
                }
              >
                {activeTransfersCount > 0
                  ? activeTransfersCount
                  : totalTransfersCount}
              </span>
            )}
          </Button>

          <Button
            variant="outline"
            size="icon-sm"
            onClick={() => setTheme(isDark ? "light" : "dark")}
            aria-label={
              mounted
                ? isDark
                  ? "Switch to light mode"
                  : "Switch to dark mode"
                : "Toggle color theme"
            }
          >
            {isDark ? (
              <Sun className="size-4 stroke-[1.5]" />
            ) : (
              <Moon className="size-4 stroke-[1.5]" />
            )}
          </Button>
        </div>
      </div>
    </header>
  );
}
