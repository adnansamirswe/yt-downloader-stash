"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, RotateCcw } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { EmptyWorkspace } from "@/components/empty-workspace";
import { InspectSkeleton } from "@/components/inspect-skeleton";
import { IntakeBar } from "@/components/intake-bar";
import { Masthead } from "@/components/masthead";
import { PlaylistSheet } from "@/components/playlist-sheet";
import { SingleMediaDeck } from "@/components/single-media-deck";
import { TransfersDrawer } from "@/components/transfers-drawer";
import {
  cancelJob,
  fetchHealth,
  getJobDownloadUrl,
  inspectUrl,
  retryJobTrack,
  startPlaylistJob,
  startSingleJob,
  StashApiError,
  subscribeToJobEvents,
  triggerBrowserDownload,
  type StartPlaylistJobInput,
  type StartSingleJobInput,
} from "@/lib/api";
import { formatSpeed } from "@/lib/format";
import type {
  HealthResponse,
  InspectResponse,
  JobSnapshot,
  MediaType,
} from "@/lib/schemas";

export function StashWorkspace() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [healthError, setHealthError] = useState(false);

  const [urlInput, setUrlInput] = useState("");
  const [defaultMediaType, setDefaultMediaType] = useState<MediaType>("video");
  const [isInspecting, setIsInspecting] = useState(false);
  const [inspectError, setInspectError] = useState<string | null>(null);
  const [inspectData, setInspectData] = useState<InspectResponse | null>(null);

  const [isStartingJob, setIsStartingJob] = useState(false);
  const [jobsById, setJobsById] = useState<Record<string, JobSnapshot>>({});
  const [jobOrder, setJobOrder] = useState<string[]>([]);
  const [stageJobId, setStageJobId] = useState<string | null>(null);
  const [transfersOpen, setTransfersOpen] = useState(false);

  const inspectAbortRef = useRef<AbortController | null>(null);
  const sseUnsubMapRef = useRef<Map<string, () => void>>(new Map());
  const autoSavedJobsRef = useRef<Set<string>>(new Set());

  // Poll backend health cleanly on mount and every 30s
  useEffect(() => {
    let active = true;
    const poll = async () => {
      try {
        const res = await fetchHealth();
        if (!active) return;
        setHealth(res);
        setHealthError(false);
      } catch {
        if (!active) return;
        setHealthError(true);
      }
    };

    void poll();
    const timer = window.setInterval(poll, 30_000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, []);

  // Clean up all active SSE streams and pending inspect calls on unmount
  useEffect(() => {
    const subs = sseUnsubMapRef.current;
    return () => {
      inspectAbortRef.current?.abort();
      for (const unsub of subs.values()) {
        unsub();
      }
      subs.clear();
    };
  }, []);

  const orderedJobs = useMemo(
    () =>
      jobOrder
        .map((id) => jobsById[id])
        .filter((j): j is JobSnapshot => Boolean(j)),
    [jobOrder, jobsById]
  );

  const activeTransfers = useMemo(
    () =>
      orderedJobs.filter(
        (j) => j.status === "queued" || j.status === "running"
      ),
    [orderedJobs]
  );

  const recentCompleted = useMemo(
    () =>
      orderedJobs.filter(
        (j) =>
          j.status === "completed" ||
          (j.kind === "playlist" && j.completedTracks > 0)
      ),
    [orderedJobs]
  );

  const registerAndTrackJob = useCallback(
    (snapshot: JobSnapshot, bindToStage: boolean) => {
      setJobsById((prev) => ({ ...prev, [snapshot.jobId]: snapshot }));
      setJobOrder((prev) =>
        prev.includes(snapshot.jobId) ? prev : [snapshot.jobId, ...prev]
      );
      if (bindToStage) {
        setStageJobId(snapshot.jobId);
      }

      const existingUnsub = sseUnsubMapRef.current.get(snapshot.jobId);
      if (existingUnsub) existingUnsub();

      const unsub = subscribeToJobEvents(
        snapshot.jobId,
        (ev) => {
          const updated = ev.job;
          setJobsById((prev) => ({ ...prev, [updated.jobId]: updated }));

          if (
            updated.status === "completed" &&
            !autoSavedJobsRef.current.has(updated.jobId)
          ) {
            autoSavedJobsRef.current.add(updated.jobId);
            triggerBrowserDownload(getJobDownloadUrl(updated.jobId));
            toast.success(
              updated.kind === "playlist"
                ? `Archive ready — downloading ${updated.completedTracks} tracks (.zip)`
                : `Ready — saving ${updated.outputFilename ?? updated.title}`
            );
          } else if (
            ev.type === "done" &&
            updated.status === "partial"
          ) {
            toast.warning(
              `${updated.completedTracks} of ${updated.totalTracks} tracks completed. Save the .zip or retry failed tracks.`
            );
          } else if (ev.type === "done" && updated.status === "failed") {
            toast.error(updated.error ?? "Transfer failed.");
          }
        },
        () => {
          // Connection closed or dropped; job remains inspectable via drawer
        }
      );

      sseUnsubMapRef.current.set(snapshot.jobId, unsub);
    },
    []
  );

  const handleInspect = useCallback(
    async (overrideUrl?: string, mode: "auto" | "video" | "playlist" = "auto") => {
      const target = (overrideUrl ?? urlInput).trim();
      if (!target) return;

      inspectAbortRef.current?.abort();
      const controller = new AbortController();
      inspectAbortRef.current = controller;

      setIsInspecting(true);
      setInspectError(null);

      try {
        const result = await inspectUrl(target, mode, controller.signal);
        setInspectData(result);
        setStageJobId(null);
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") {
          return;
        }
        const message =
          err instanceof StashApiError
            ? err.message
            : "Could not reach the Stash backend. Verify the server is running.";
        setInspectError(message);
        setInspectData(null);
      } finally {
        setIsInspecting(false);
      }
    },
    [urlInput]
  );

  const handleStartSingle = useCallback(
    async (payload: StartSingleJobInput, bindToStage = true) => {
      setIsStartingJob(true);
      try {
        const snap = await startSingleJob(payload);
        registerAndTrackJob(snap, bindToStage);
        if (!bindToStage) {
          toast.info(`Queued "${payload.title ?? "track"}" in Transfers`);
        }
      } catch (err) {
        const msg =
          err instanceof StashApiError ? err.message : "Could not start transfer.";
        toast.error(msg);
      } finally {
        setIsStartingJob(false);
      }
    },
    [registerAndTrackJob]
  );

  const handleStartPlaylist = useCallback(
    async (payload: StartPlaylistJobInput) => {
      setIsStartingJob(true);
      try {
        const snap = await startPlaylistJob(payload);
        registerAndTrackJob(snap, true);
        toast.info(
          `Parallel transfer started for ${snap.totalTracks} selected ${snap.totalTracks === 1 ? "track" : "tracks"}.`
        );
      } catch (err) {
        const msg =
          err instanceof StashApiError
            ? err.message
            : "Could not start playlist batch.";
        toast.error(msg);
      } finally {
        setIsStartingJob(false);
      }
    },
    [registerAndTrackJob]
  );

  const handleCancelJob = useCallback(async (jobId: string) => {
    try {
      const snap = await cancelJob(jobId);
      setJobsById((prev) => ({ ...prev, [jobId]: snap }));
      const unsub = sseUnsubMapRef.current.get(jobId);
      if (unsub) {
        unsub();
        sseUnsubMapRef.current.delete(jobId);
      }
      toast.info("Transfer cancelled.");
    } catch (err) {
      const msg =
        err instanceof StashApiError ? err.message : "Could not cancel transfer.";
      toast.error(msg);
    }
  }, []);

  const handleRetryTrack = useCallback(
    async (jobId: string, videoId: string) => {
      try {
        const snap = await retryJobTrack(jobId, videoId);
        autoSavedJobsRef.current.delete(jobId);
        registerAndTrackJob(snap, stageJobId === jobId);
        toast.info("Retrying failed track...");
      } catch (err) {
        const msg =
          err instanceof StashApiError ? err.message : "Could not retry track.";
        toast.error(msg);
      }
    },
    [registerAndTrackJob, stageJobId]
  );

  const handleResetWorkspace = useCallback(() => {
    inspectAbortRef.current?.abort();
    setUrlInput("");
    setInspectData(null);
    setInspectError(null);
    setStageJobId(null);
  }, []);

  const handleClearFinished = useCallback(() => {
    setJobsById((prev) => {
      const kept: Record<string, JobSnapshot> = {};
      for (const [id, job] of Object.entries(prev)) {
        if (job.status === "queued" || job.status === "running") {
          kept[id] = job;
        }
      }
      return kept;
    });
    setJobOrder((prev) =>
      prev.filter((id) => {
        const j = jobsById[id];
        return j && (j.status === "queued" || j.status === "running");
      })
    );
  }, [jobsById]);

  const stageJob = stageJobId ? (jobsById[stageJobId] ?? null) : null;

  const singleJobsByVideoId = useMemo(() => {
    const map: Record<string, JobSnapshot> = {};
    for (const job of orderedJobs) {
      if (job.kind === "single" && job.tracks[0]?.videoId) {
        const vid = job.tracks[0].videoId;
        if (!map[vid]) {
          map[vid] = job;
        }
      }
    }
    return map;
  }, [orderedJobs]);

  const intakeTelemetry = useMemo(() => {
    const running = activeTransfers[0];
    if (!running) return null;
    return {
      percent: running.overallPercent,
      leftLabel:
        running.kind === "playlist"
          ? `ACTIVE BATCH · ${running.completedTracks}/${running.totalTracks} TRACKS · ${running.title}`
          : `ACTIVE TRANSFER · ${running.title}`,
      rightLabel: `${running.overallPercent.toFixed(0)}% · ${formatSpeed(running.aggregateSpeedBytes)}`,
    };
  }, [activeTransfers]);

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <Masthead
        health={health}
        healthError={healthError}
        activeTransfersCount={activeTransfers.length}
        totalTransfersCount={orderedJobs.length}
        onOpenTransfers={() => setTransfersOpen(true)}
        onResetWorkspace={handleResetWorkspace}
      />

      <main className="mx-auto flex w-full max-w-[1120px] flex-1 flex-col gap-6 px-4 py-6 sm:px-6 sm:py-8">
        {/* Quiet editorial lead-in when idle */}
        {!inspectData && !isInspecting && (
          <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-end">
            <div>
              <h1 className="font-display text-[2.25rem] leading-[1.06] tracking-[-0.015em] text-foreground sm:text-[2.75rem]">
                Archive YouTube media at full line speed.
              </h1>
              <p className="mt-1.5 text-sm text-ink-secondary">
                Single videos, lossless or native audio, and selective parallel
                playlist batches.
              </p>
            </div>
            <span className="font-mono-tabular text-[11px] uppercase tracking-[0.08em] text-ink-muted">
              REMUX FIRST · ZERO BLOAT
            </span>
          </div>
        )}

        <IntakeBar
          url={urlInput}
          onUrlChange={setUrlInput}
          defaultMediaType={defaultMediaType}
          onDefaultMediaTypeChange={setDefaultMediaType}
          isInspecting={isInspecting}
          onInspect={(override) => void handleInspect(override, "auto")}
          onClear={handleResetWorkspace}
          activeTelemetry={intakeTelemetry}
        />

        {/* Error state */}
        {inspectError && !isInspecting && (
          <div
            role="alert"
            className="flex flex-wrap items-center justify-between gap-3 border border-status-error/35 bg-card p-4"
          >
            <div className="flex items-start gap-3">
              <AlertCircle className="mt-0.5 size-4 shrink-0 text-status-error stroke-[1.75]" />
              <div>
                <p className="text-sm font-medium text-foreground">
                  Could not inspect link
                </p>
                <p className="mt-0.5 text-xs text-ink-secondary">
                  {inspectError}
                </p>
              </div>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void handleInspect(undefined, "auto")}
            >
              <RotateCcw className="size-3.5" />
              <span>Try Again</span>
            </Button>
          </div>
        )}

        {/* Loading skeleton */}
        {isInspecting && <InspectSkeleton />}

        {/* Dynamic Stage: Single Video or Selective Playlist */}
        {!isInspecting && inspectData?.kind === "video" && inspectData.video && (
          <SingleMediaDeck
            key={inspectData.video.videoId}
            inspect={inspectData}
            initialMediaType={defaultMediaType}
            activeJob={stageJob}
            isSubmitting={isStartingJob}
            onStartDownload={(payload) => void handleStartSingle(payload, true)}
            onCancelJob={(id) => void handleCancelJob(id)}
            onLoadFullPlaylist={(plUrl) => {
              setUrlInput(plUrl);
              void handleInspect(plUrl, "playlist");
            }}
          />
        )}

        {!isInspecting &&
          inspectData?.kind === "playlist" &&
          inspectData.playlist && (
            <PlaylistSheet
              key={inspectData.playlist.playlistId}
              inspect={inspectData}
              initialMediaType={defaultMediaType}
              maxConcurrency={health?.maxConcurrentTracks ?? 4}
              activeJob={stageJob}
              singleJobsByVideoId={singleJobsByVideoId}
              isSubmitting={isStartingJob}
              onStartPlaylistJob={(payload) => void handleStartPlaylist(payload)}
              onStartSingleTrack={(payload) =>
                handleStartSingle(payload, false)
              }
              onCancelJob={(id) => void handleCancelJob(id)}
              onRetryTrack={(jobId, vid) => void handleRetryTrack(jobId, vid)}
            />
          )}

        {/* Empty / Idle Workspace */}
        {!isInspecting && !inspectData && (
          <EmptyWorkspace recentCompleted={recentCompleted} />
        )}
      </main>

      <footer className="mt-auto border-t border-border py-4">
        <div className="mx-auto flex max-w-[1120px] flex-wrap items-center justify-between gap-2 px-4 font-mono-tabular text-[11px] text-ink-muted sm:px-6">
          <span>STASH · ARCHIVAL MEDIA CONSOLE</span>
          <span>
            {health
              ? `GO API · UP TO ${health.maxConcurrentTracks}× PARALLEL WORKERS`
              : "STANDALONE NEXT.JS CLIENT"}
          </span>
        </div>
      </footer>

      <TransfersDrawer
        open={transfersOpen}
        onOpenChange={setTransfersOpen}
        jobs={orderedJobs}
        onCancelJob={(id) => void handleCancelJob(id)}
        onRetryTrack={(jobId, vid) => void handleRetryTrack(jobId, vid)}
        onClearFinished={handleClearFinished}
      />
    </div>
  );
}
