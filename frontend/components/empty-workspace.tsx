"use client";

import { ArrowDownToLine, CornerDownLeft, Layers, Zap } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatBytes } from "@/lib/format";
import { getJobDownloadUrl, triggerBrowserDownload } from "@/lib/api";
import type { JobSnapshot } from "@/lib/schemas";

interface EmptyWorkspaceProps {
  recentCompleted: JobSnapshot[];
}

export function EmptyWorkspace({ recentCompleted }: EmptyWorkspaceProps) {
  return (
    <div className="space-y-8 pt-2">
      {/* Quiet specification sheet — zero marketing bloat or fake metrics */}
      <div className="grid grid-cols-1 border-t border-border md:grid-cols-3">
        <div className="border-b border-border py-5 md:border-r md:pr-6">
          <div className="flex items-center gap-2 font-mono-tabular text-[11px] uppercase tracking-[0.08em] text-ink-muted">
            <Zap className="size-3.5 text-signal-ink stroke-[1.6]" />
            <span>01 · Native Remux Pipeline</span>
          </div>
          <p className="mt-2.5 text-sm leading-relaxed text-ink-secondary">
            Direct stream-copy for H.264/MP4, WebM, MKV, M4A, and Opus skips slow
            re-encoding. Transcodes to MP3 320k, FLAC, or WAV only when requested.
          </p>
        </div>

        <div className="border-b border-border py-5 md:border-r md:px-6">
          <div className="flex items-center gap-2 font-mono-tabular text-[11px] uppercase tracking-[0.08em] text-ink-muted">
            <Layers className="size-3.5 text-signal-ink stroke-[1.6]" />
            <span>02 · Selective Playlist Batches</span>
          </div>
          <p className="mt-2.5 text-sm leading-relaxed text-ink-secondary">
            Flat-indexed playlists load in seconds. Filter tracks, use{" "}
            <kbd className="rounded-[2px] border border-border bg-muted px-1 py-0.5 font-mono-tabular text-[11px] text-foreground">
              Shift+Click
            </kbd>{" "}
            range selection, download in parallel, and stream a zero-compression{" "}
            <span className="font-mono-tabular text-foreground">.zip</span>.
          </p>
        </div>

        <div className="border-b border-border py-5 md:pl-6">
          <div className="flex items-center gap-2 font-mono-tabular text-[11px] uppercase tracking-[0.08em] text-ink-muted">
            <CornerDownLeft className="size-3.5 text-signal-ink stroke-[1.6]" />
            <span>03 · Keyboard Intake</span>
          </div>
          <dl className="mt-2.5 space-y-1.5 text-xs text-ink-secondary">
            <div className="flex items-center justify-between gap-2">
              <dt>Paste YouTube link anywhere</dt>
              <dd className="font-mono-tabular text-foreground">Ctrl / ⌘ + V</dd>
            </div>
            <div className="flex items-center justify-between gap-2">
              <dt>Contiguous playlist range</dt>
              <dd className="font-mono-tabular text-foreground">Shift + Click</dd>
            </div>
            <div className="flex items-center justify-between gap-2">
              <dt>Supported sources</dt>
              <dd className="font-mono-tabular text-foreground">
                Watch · Playlist · Music
              </dd>
            </div>
          </dl>
        </div>
      </div>

      {/* Recent completed items in current session */}
      {recentCompleted.length > 0 && (
        <section aria-label="Recent transfers in this session">
          <div className="mb-2.5 flex items-center justify-between">
            <h2 className="font-mono-tabular text-[11px] uppercase tracking-[0.08em] text-ink-muted">
              Ready in Current Session
            </h2>
            <span className="font-mono-tabular text-[11px] text-ink-muted">
              {recentCompleted.length} archived
            </span>
          </div>
          <div className="divide-y divide-border border border-border bg-card">
            {recentCompleted.slice(0, 5).map((job) => (
              <div
                key={job.jobId}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <Badge variant="secondary">
                      {job.kind === "playlist"
                        ? `ZIP · ${job.completedTracks} TRACKS`
                        : `${job.container.toUpperCase()}`}
                    </Badge>
                    <p className="truncate text-sm font-medium text-foreground">
                      {job.title}
                    </p>
                  </div>
                  <p className="mt-0.5 font-mono-tabular text-[11px] text-ink-muted">
                    {job.outputFilename ?? job.title}
                    {job.outputSizeBytes
                      ? ` · ${formatBytes(job.outputSizeBytes)}`
                      : ""}
                  </p>
                </div>

                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => triggerBrowserDownload(getJobDownloadUrl(job.jobId))}
                >
                  <ArrowDownToLine className="size-3.5 stroke-[1.6]" />
                  <span>Save Again</span>
                </Button>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
