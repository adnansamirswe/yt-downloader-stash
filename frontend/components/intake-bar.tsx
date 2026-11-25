"use client";

import { useEffect, useRef } from "react";
import {
  ArrowRight,
  ClipboardPaste,
  ListMusic,
  Loader2,
  PlaySquare,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { cn } from "cn";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ModeSwitch } from "@/components/mode-switch";
import { TelemetryRuler } from "@/components/telemetry-ruler";
import {
  detectYouTubeInput,
  type DetectedSourceKind,
  type MediaType,
} from "@/lib/schemas";

interface IntakeBarProps {
  url: string;
  onUrlChange: (value: string) => void;
  defaultMediaType: MediaType;
  onDefaultMediaTypeChange: (mode: MediaType) => void;
  showModeSwitch?: boolean;
  isInspecting: boolean;
  onInspect: (overrideUrl?: string) => void;
  onClear: () => void;
  activeTelemetry?: {
    percent: number;
    leftLabel: string;
    rightLabel: string;
  } | null;
}

const SOURCE_LABELS: Record<
  Exclude<DetectedSourceKind, "empty">,
  { text: string; variant: "signal" | "secondary" | "destructive" }
> = {
  video: { text: "VIDEO", variant: "secondary" },
  playlist: { text: "PLAYLIST", variant: "signal" },
  hybrid: { text: "VIDEO + PLAYLIST", variant: "signal" },
  invalid: { text: "YOUTUBE URL REQUIRED", variant: "destructive" },
};

export function IntakeBar({
  url,
  onUrlChange,
  defaultMediaType,
  onDefaultMediaTypeChange,
  showModeSwitch = true,
  isInspecting,
  onInspect,
  onClear,
  activeTelemetry,
}: IntakeBarProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const detected = detectYouTubeInput(url);

  // Global Cmd/Ctrl+V paste shortcut when user is not already focused in an input
  useEffect(() => {
    const handleGlobalPaste = (event: ClipboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (
        target &&
        (target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.isContentEditable)
      ) {
        return;
      }
      const pasted = event.clipboardData?.getData("text")?.trim();
      if (!pasted) return;
      const kind = detectYouTubeInput(pasted);
      if (kind !== "empty" && kind !== "invalid") {
        event.preventDefault();
        onUrlChange(pasted);
        onInspect(pasted);
      }
    };

    window.addEventListener("paste", handleGlobalPaste);
    return () => window.removeEventListener("paste", handleGlobalPaste);
  }, [onInspect, onUrlChange]);

  async function handleClipboardButton() {
    try {
      const text = (await navigator.clipboard.readText()).trim();
      if (!text) {
        toast.info("Clipboard is empty.");
        return;
      }
      onUrlChange(text);
      const kind = detectYouTubeInput(text);
      if (kind !== "invalid" && kind !== "empty") {
        onInspect(text);
      } else {
        inputRef.current?.focus();
        toast.error("Clipboard does not contain a valid YouTube link.");
      }
    } catch {
      inputRef.current?.focus();
      toast.info("Press Ctrl+V or Cmd+V to paste directly into the bar.");
    }
  }

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!url.trim()) {
      inputRef.current?.focus();
      return;
    }
    onInspect();
  }

  return (
    <section
      aria-label="Media link intake"
      className="border border-border bg-card"
    >
      {/* Archival deck header strip */}
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border bg-muted/45 px-4 py-2">
        <div className="flex items-center gap-2 font-mono-tabular text-[11px] uppercase tracking-[0.08em] text-ink-secondary">
          <span className="inline-block size-1.5 rounded-full bg-signal" />
          <span>Media Intake Console</span>
        </div>

        <div className="flex items-center gap-2">
          {detected !== "empty" ? (
            <Badge variant={SOURCE_LABELS[detected].variant}>
              {detected === "playlist" || detected === "hybrid" ? (
                <ListMusic className="size-3" />
              ) : detected === "video" ? (
                <PlaySquare className="size-3" />
              ) : null}
              {SOURCE_LABELS[detected].text}
            </Badge>
          ) : (
            <span className="font-mono-tabular text-[11px] text-ink-muted">
              PASTE VIDEO OR PLAYLIST LINK
            </span>
          )}
        </div>
      </div>

      {/* Main console form */}
      <form
        onSubmit={handleSubmit}
        className="flex flex-col gap-3 p-3.5 sm:flex-row sm:items-center sm:gap-2.5 sm:p-4"
      >
        <div className="relative flex flex-1 items-center rounded-[4px] border border-border-strong bg-background transition-colors focus-within:border-signal">
          <label htmlFor="stash-url-input" className="sr-only">
            YouTube video or playlist URL
          </label>
          <input
            ref={inputRef}
            id="stash-url-input"
            type="url"
            inputMode="url"
            autoComplete="off"
            spellCheck={false}
            value={url}
            onChange={(e) => onUrlChange(e.target.value)}
            onPaste={(e) => {
              const pasted = e.clipboardData.getData("text").trim();
              const kind = detectYouTubeInput(pasted);
              if (kind !== "empty" && kind !== "invalid") {
                e.preventDefault();
                onUrlChange(pasted);
                onInspect(pasted);
              }
            }}
            placeholder="https://www.youtube.com/watch?v=... or playlist?list=..."
            className={cn(
              "h-11 w-full bg-transparent px-3.5 pr-10 text-sm text-foreground placeholder:text-ink-muted focus:outline-none",
              "font-mono-tabular text-[13px]"
            )}
          />
          {url ? (
            <button
              type="button"
              onClick={onClear}
              aria-label="Clear URL"
              className="absolute right-1.5 inline-flex size-8 cursor-pointer items-center justify-center rounded-[2px] text-ink-muted transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
            >
              <X className="size-4 stroke-[1.6]" />
            </button>
          ) : (
            <button
              type="button"
              onClick={handleClipboardButton}
              aria-label="Paste from clipboard"
              className="absolute right-1.5 inline-flex h-8 cursor-pointer items-center gap-1 rounded-[2px] border border-border bg-card px-2 font-mono-tabular text-[11px] text-ink-secondary transition-colors hover:border-border-strong hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
            >
              <ClipboardPaste className="size-3.5 stroke-[1.5]" />
              <span className="hidden sm:inline">Paste</span>
            </button>
          )}
        </div>

        <div className="flex items-center justify-between gap-2 sm:justify-end">
          {showModeSwitch && (
            <ModeSwitch
              value={defaultMediaType}
              onChange={onDefaultMediaTypeChange}
              layoutGroupId="intake-mode-pill"
            />
          )}

          <Button
            type="submit"
            variant="signal"
            size="lg"
            disabled={isInspecting || detected === "empty" || detected === "invalid"}
            className="flex-1 sm:flex-none"
          >
            {isInspecting ? (
              <>
                <Loader2 className="size-4 animate-spin stroke-[1.75]" />
                <span>Inspecting</span>
              </>
            ) : (
              <>
                <span>Inspect</span>
                <ArrowRight className="size-4 stroke-[1.75]" />
              </>
            )}
          </Button>
        </div>
      </form>

      {/* Signature Move: Precision 2px Hairline Telemetry Ruler */}
      {isInspecting ? (
        <div className="px-4 pb-3">
          <TelemetryRuler
            indeterminate
            leftLabel="QUERYING YOUTUBE METADATA"
            rightLabel="FLAT STREAM INDEX"
            compact
          />
        </div>
      ) : activeTelemetry ? (
        <div className="border-t border-border bg-muted/25 px-4 py-2.5">
          <TelemetryRuler
            percent={activeTelemetry.percent}
            leftLabel={activeTelemetry.leftLabel}
            rightLabel={activeTelemetry.rightLabel}
            compact
          />
        </div>
      ) : (
        <div className="h-[2px] w-full bg-border/60" aria-hidden="true" />
      )}
    </section>
  );
}
