"use client";

import {
  AlertCircle,
  Archive,
  ArrowDownToLine,
  CheckCircle2,
  RotateCcw,
  Square,
  Trash2,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { TelemetryRuler } from "@/components/telemetry-ruler";
import { formatBytes, formatSpeed } from "@/lib/format";
import { getJobDownloadUrl, triggerBrowserDownload } from "@/lib/api";
import type { JobSnapshot } from "@/lib/schemas";

interface TransfersDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  jobs: JobSnapshot[];
  onCancelJob: (jobId: string) => void;
  onRetryTrack: (jobId: string, videoId: string) => void;
  onClearFinished: () => void;
}

export function TransfersDrawer({
  open,
  onOpenChange,
  jobs,
  onCancelJob,
  onRetryTrack,
  onClearFinished,
}: TransfersDrawerProps) {
  const hasFinished = jobs.some(
    (j) =>
      j.status === "completed" ||
      j.status === "failed" ||
      j.status === "cancelled" ||
      j.status === "partial"
  );

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="p-0">
        <SheetHeader className="pr-12">
          <div className="flex items-center justify-between gap-2">
            <SheetTitle>Transfers</SheetTitle>
            {hasFinished && (
              <Button variant="ghost" size="xs" onClick={onClearFinished}>
                <Trash2 className="size-3.5" />
                <span>Clear Done</span>
              </Button>
            )}
          </div>
          <SheetDescription>
            Live single and parallel playlist transfers from this session.
          </SheetDescription>
        </SheetHeader>

        {jobs.length === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center px-6 py-16 text-center">
            <p className="font-display text-xl text-foreground">
              Queue is quiet
            </p>
            <p className="mt-1 max-w-xs text-xs leading-relaxed text-ink-secondary">
              Inspect a YouTube video or playlist and start a transfer to track
              live throughput here.
            </p>
          </div>
        ) : (
          <ScrollArea className="flex-1">
            <div className="divide-y divide-border">
              {jobs.map((job) => {
                const isRunning =
                  job.status === "queued" || job.status === "running";
                const canSave =
                  job.kind === "single"
                    ? job.status === "completed"
                    : job.completedTracks > 0;
                const failedTracks = job.tracks.filter(
                  (t) => t.status === "failed"
                );

                return (
                  <div key={job.jobId} className="space-y-2.5 p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-1.5">
                          <Badge
                            variant={
                              job.status === "completed"
                                ? "ready"
                                : job.status === "failed"
                                  ? "destructive"
                                  : job.status === "partial"
                                    ? "warn"
                                    : "signal"
                            }
                          >
                            {job.kind === "playlist"
                              ? `ZIP · ${job.completedTracks}/${job.totalTracks}`
                              : job.container.toUpperCase()}
                          </Badge>
                          <span className="font-mono-tabular text-[11px] uppercase text-ink-muted">
                            {job.status}
                          </span>
                        </div>
                        <p className="mt-1.5 truncate text-sm font-medium text-foreground">
                          {job.title}
                        </p>
                      </div>

                      <div className="flex shrink-0 items-center gap-1.5">
                        {isRunning && (
                          <Button
                            variant="destructive"
                            size="xs"
                            onClick={() => onCancelJob(job.jobId)}
                          >
                            <Square className="size-3 fill-current" />
                            <span>Cancel</span>
                          </Button>
                        )}
                        {canSave && (
                          <Button
                            variant={
                              job.status === "completed" ? "signal" : "outline"
                            }
                            size="xs"
                            onClick={() =>
                              triggerBrowserDownload(
                                getJobDownloadUrl(job.jobId)
                              )
                            }
                          >
                            {job.kind === "playlist" ? (
                              <Archive className="size-3.5" />
                            ) : (
                              <ArrowDownToLine className="size-3.5" />
                            )}
                            <span>Save</span>
                          </Button>
                        )}
                      </div>
                    </div>

                    <TelemetryRuler
                      compact
                      percent={
                        job.status === "completed" ? 100 : job.overallPercent
                      }
                      tone={
                        job.status === "completed"
                          ? "ready"
                          : job.status === "failed"
                            ? "error"
                            : "signal"
                      }
                      leftLabel={
                        job.status === "completed"
                          ? job.outputFilename ?? "Ready"
                          : isRunning
                            ? formatSpeed(job.aggregateSpeedBytes)
                            : job.error ?? job.status.toUpperCase()
                      }
                      rightLabel={
                        job.status === "completed" && job.outputSizeBytes
                          ? formatBytes(job.outputSizeBytes)
                          : `${job.overallPercent.toFixed(0)}%`
                      }
                    />

                    {failedTracks.length > 0 && (
                      <div className="space-y-1.5 rounded-[2px] border border-status-error/25 bg-status-error/5 p-2.5">
                        <div className="flex items-center gap-1.5 font-mono-tabular text-[11px] text-status-error">
                          <AlertCircle className="size-3.5" />
                          <span>
                            {failedTracks.length}{" "}
                            {failedTracks.length === 1 ? "track" : "tracks"}{" "}
                            failed
                          </span>
                        </div>
                        {failedTracks.slice(0, 4).map((ft) => (
                          <div
                            key={ft.videoId}
                            className="flex items-center justify-between gap-2 text-xs"
                          >
                            <span className="truncate text-ink-secondary">
                              {ft.title}
                            </span>
                            <Button
                              variant="outline"
                              size="xs"
                              onClick={() =>
                                onRetryTrack(job.jobId, ft.videoId)
                              }
                            >
                              <RotateCcw className="size-3" />
                              <span>Retry</span>
                            </Button>
                          </div>
                        ))}
                      </div>
                    )}

                    {job.status === "completed" && (
                      <div className="flex items-center gap-1.5 font-mono-tabular text-[11px] text-status-ready">
                        <CheckCircle2 className="size-3.5" />
                        <span>Ready for download</span>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </ScrollArea>
        )}
      </SheetContent>
    </Sheet>
  );
}
