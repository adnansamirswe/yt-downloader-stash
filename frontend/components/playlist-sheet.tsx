"use client";

import { useMemo, useRef, useState } from "react";
import Image from "next/image";
import { useVirtualizer } from "@tanstack/react-virtual";
import { motion } from "motion/react";
import {
  AlertCircle,
  Archive,
  ArrowDownToLine,
  CheckCircle2,
  Loader2,
  RotateCcw,
  Search,
  Square,
  Tag,
  X,
} from "lucide-react";
import { cn } from "cn";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ModeSwitch } from "@/components/mode-switch";
import { TelemetryRuler } from "@/components/telemetry-ruler";
import {
  formatBytes,
  formatDuration,
  formatSpeed,
  formatTotalDuration,
} from "@/lib/format";
import {
  getJobDownloadUrl,
  getTrackDownloadUrl,
  triggerBrowserDownload,
  type StartPlaylistJobInput,
  type StartSingleJobInput,
} from "@/lib/api";
import type {
  AudioBitrate,
  InspectResponse,
  JobSnapshot,
  MediaType,
  PlaylistEntry,
  TrackState,
  VideoContainer,
} from "@/lib/schemas";

interface PlaylistSheetProps {
  inspect: InspectResponse;
  initialMediaType: MediaType;
  maxConcurrency: number;
  activeJob: JobSnapshot | null;
  isSubmitting: boolean;
  onStartPlaylistJob: (payload: StartPlaylistJobInput) => void;
  onStartSingleTrack: (payload: StartSingleJobInput) => void;
  onCancelJob: (jobId: string) => void;
  onRetryTrack: (jobId: string, videoId: string) => void;
}

const VIDEO_QUALITY_OPTIONS = [
  { value: "best", label: "Best Available" },
  { value: "2160p", label: "2160p · 4K" },
  { value: "1440p", label: "1440p · 2K" },
  { value: "1080p", label: "1080p · Full HD" },
  { value: "720p", label: "720p · HD" },
  { value: "480p", label: "480p" },
  { value: "360p", label: "360p" },
];

const AUDIO_FORMAT_OPTIONS: Array<{
  value: string;
  label: string;
  container: string;
  bitrate?: AudioBitrate;
}> = [
  { value: "m4a", label: "M4A · Native Fast", container: "m4a" },
  { value: "opus", label: "Opus · Native Fast", container: "opus" },
  { value: "mp3-320k", label: "MP3 · 320 kbps", container: "mp3", bitrate: "320k" },
  { value: "mp3-192k", label: "MP3 · 192 kbps", container: "mp3", bitrate: "192k" },
  { value: "flac", label: "FLAC · Lossless", container: "flac" },
  { value: "wav", label: "WAV · Uncompressed", container: "wav" },
];

export function PlaylistSheet({
  inspect,
  initialMediaType,
  maxConcurrency,
  activeJob,
  isSubmitting,
  onStartPlaylistJob,
  onStartSingleTrack,
  onCancelJob,
  onRetryTrack,
}: PlaylistSheetProps) {
  const playlist = inspect.playlist;
  const availableIds = useMemo(
    () => (playlist?.entries ?? []).filter((e) => e.available).map((e) => e.videoId),
    [playlist]
  );

  const [selectedIds, setSelectedIds] = useState<Set<string>>(
    () => new Set(availableIds)
  );
  const [filterQuery, setFilterQuery] = useState("");
  const [mediaType, setMediaType] = useState<MediaType>(initialMediaType);
  const [videoQuality, setVideoQuality] = useState("1080p");
  const [videoContainer, setVideoContainer] = useState<VideoContainer>("mp4");
  const [audioPresetKey, setAudioPresetKey] = useState("m4a");
  const [concurrency, setConcurrency] = useState(() =>
    Math.min(Math.max(1, maxConcurrency), 4)
  );
  const [embedMetadata, setEmbedMetadata] = useState(true);

  const lastClickedIdxRef = useRef<number | null>(null);
  const listParentRef = useRef<HTMLDivElement>(null);

  const filteredEntries = useMemo(() => {
    const entries = playlist?.entries ?? [];
    const q = filterQuery.trim().toLowerCase();
    if (!q) return entries;
    return entries.filter(
      (e) =>
        e.title.toLowerCase().includes(q) ||
        e.channel.toLowerCase().includes(q) ||
        String(e.index).includes(q)
    );
  }, [playlist, filterQuery]);

  const trackStateById = useMemo(() => {
    const map = new Map<string, TrackState>();
    for (const tr of activeJob?.tracks ?? []) {
      map.set(tr.videoId, tr);
    }
    return map;
  }, [activeJob]);

  const selectedEntries = useMemo(
    () =>
      (playlist?.entries ?? []).filter(
        (e) => e.available && selectedIds.has(e.videoId)
      ),
    [playlist, selectedIds]
  );

  const selectedDurationSeconds = useMemo(
    () => selectedEntries.reduce((acc, e) => acc + e.durationSeconds, 0),
    [selectedEntries]
  );

  const rowVirtualizer = useVirtualizer({
    count: filteredEntries.length,
    getScrollElement: () => listParentRef.current,
    estimateSize: () => 74,
    overscan: 8,
  });

  if (!playlist) return null;

  const isRunning =
    activeJob?.status === "queued" || activeJob?.status === "running";
  const canDownloadZip = (activeJob?.completedTracks ?? 0) > 0;
  const allFilteredAvailable = filteredEntries.filter((e) => e.available);

  function toggleRowSelection(entry: PlaylistEntry, filteredIdx: number, shiftKey: boolean) {
    if (!entry.available) return;

    setSelectedIds((prev) => {
      const next = new Set(prev);
      const willSelect = !next.has(entry.videoId);

      if (shiftKey && lastClickedIdxRef.current !== null) {
        const start = Math.min(lastClickedIdxRef.current, filteredIdx);
        const end = Math.max(lastClickedIdxRef.current, filteredIdx);
        for (let i = start; i <= end; i++) {
          const item = filteredEntries[i];
          if (item && item.available) {
            if (willSelect) next.add(item.videoId);
            else next.delete(item.videoId);
          }
        }
      } else {
        if (willSelect) next.add(entry.videoId);
        else next.delete(entry.videoId);
      }

      return next;
    });
    lastClickedIdxRef.current = filteredIdx;
  }

  function selectAllFiltered() {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      for (const e of allFilteredAvailable) next.add(e.videoId);
      return next;
    });
  }

  function clearSelection() {
    setSelectedIds(new Set());
  }

  function invertSelection() {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      for (const e of allFilteredAvailable) {
        if (next.has(e.videoId)) next.delete(e.videoId);
        else next.add(e.videoId);
      }
      return next;
    });
  }

  function resolveSelectedAudioFormat(): {
    container: string;
    audioBitrate?: AudioBitrate;
  } {
    const found =
      AUDIO_FORMAT_OPTIONS.find((a) => a.value === audioPresetKey) ??
      AUDIO_FORMAT_OPTIONS[0];
    return {
      container: found.container,
      audioBitrate: found.bitrate,
    };
  }

  function handleDispatchBatch() {
    if (!playlist || selectedEntries.length === 0) return;

    const audioFmt = resolveSelectedAudioFormat();
    onStartPlaylistJob({
      playlistUrl: inspect.sourceUrl,
      playlistTitle: playlist.title,
      items: selectedEntries.map((e) => ({
        videoId: e.videoId,
        title: e.title,
        index: e.index,
      })),
      mediaType,
      quality: mediaType === "video" ? videoQuality : undefined,
      container: mediaType === "video" ? videoContainer : audioFmt.container,
      audioBitrate: mediaType === "audio" ? audioFmt.audioBitrate : undefined,
      concurrency,
      embedMetadata,
    });
  }

  function handleSingleQuickStash(entry: PlaylistEntry) {
    const audioFmt = resolveSelectedAudioFormat();
    onStartSingleTrack({
      url: `https://www.youtube.com/watch?v=${entry.videoId}`,
      title: entry.title,
      mediaType,
      quality: mediaType === "video" ? videoQuality : undefined,
      container: mediaType === "video" ? videoContainer : audioFmt.container,
      audioBitrate: mediaType === "audio" ? audioFmt.audioBitrate : undefined,
      embedMetadata,
    });
  }

  const concurrencyChoices = Array.from(
    { length: Math.min(Math.max(1, maxConcurrency), 6) },
    (_, i) => i + 1
  );

  return (
    <motion.section
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
      aria-label="Playlist catalog and selective downloader"
      className="border border-border bg-card"
    >
      {/* Playlist Catalog Header */}
      <div className="flex flex-col gap-4 border-b border-border p-5 sm:flex-row sm:items-center">
        {playlist.thumbnailUrl && (
          <div className="relative aspect-video w-full shrink-0 overflow-hidden rounded-[2px] border border-border bg-muted sm:w-44">
            <Image
              src={playlist.thumbnailUrl}
              alt={playlist.title}
              fill
              unoptimized
              sizes="176px"
              className="object-cover"
            />
          </div>
        )}
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="signal">PLAYLIST CATALOG</Badge>
            <Badge variant="secondary">
              {playlist.entryCount} TRACKS
            </Badge>
            {playlist.totalDurationSeconds > 0 && (
              <Badge variant="outline">
                {formatTotalDuration(playlist.totalDurationSeconds)}
              </Badge>
            )}
            {inspect.cached && <Badge variant="ready">CACHED</Badge>}
          </div>

          <h1 className="font-display text-[1.75rem] leading-[1.12] tracking-[-0.01em] text-foreground">
            {playlist.title}
          </h1>

          {playlist.channel && (
            <p className="text-xs text-ink-secondary">
              Curated by{" "}
              <span className="font-medium text-foreground">
                {playlist.channel.replace(/^by\s+/i, "")}
              </span>
            </p>
          )}
        </div>
      </div>

      {/* Sticky Selection & Batch Mastering Toolbar (sticky on sm+ so mobile tracklists have full scroll space) */}
      <div className="sm:sticky sm:top-14 z-30 border-b border-border bg-card/98">
        {/* Active Batch Telemetry Banner */}
        {activeJob && (
          <div className="border-b border-border bg-muted/45 px-4 py-3">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0 flex-1">
                <TelemetryRuler
                  percent={
                    activeJob.status === "completed"
                      ? 100
                      : activeJob.overallPercent
                  }
                  tone={
                    activeJob.status === "completed"
                      ? "ready"
                      : activeJob.status === "failed"
                        ? "error"
                        : "signal"
                  }
                  leftLabel={
                    activeJob.status === "completed"
                      ? `ARCHIVE READY · ${activeJob.completedTracks} OF ${activeJob.totalTracks} TRACKS`
                      : activeJob.status === "partial"
                        ? `PARTIAL ARCHIVE · ${activeJob.completedTracks} READY · ${activeJob.failedTracks} FAILED`
                        : activeJob.status === "cancelled"
                          ? "BATCH CANCELLED"
                          : `PARALLEL BATCH · ${activeJob.completedTracks}/${activeJob.totalTracks} DONE · ${formatSpeed(activeJob.aggregateSpeedBytes)}`
                  }
                  rightLabel={`${activeJob.overallPercent.toFixed(0)}%`}
                />
              </div>

              <div className="flex items-center gap-2">
                {isRunning && (
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => onCancelJob(activeJob.jobId)}
                  >
                    <Square className="size-3 fill-current" />
                    <span>Cancel</span>
                  </Button>
                )}

                {canDownloadZip && (
                  <Button
                    variant={activeJob.status === "completed" ? "signal" : "default"}
                    size="sm"
                    onClick={() =>
                      triggerBrowserDownload(getJobDownloadUrl(activeJob.jobId))
                    }
                  >
                    <Archive className="size-3.5 stroke-[1.6]" />
                    <span>
                      Save .zip ({activeJob.completedTracks}
                      {activeJob.outputSizeBytes
                        ? ` · ${formatBytes(activeJob.outputSizeBytes)}`
                        : ""}
                      )
                    </span>
                  </Button>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Row 1: Search & Selection Helpers */}
        <div className="flex flex-wrap items-center justify-between gap-2.5 border-b border-border px-4 py-2.5">
          <div className="relative flex w-full items-center sm:max-w-xs">
            <Search className="pointer-events-none absolute left-2.5 size-3.5 text-ink-muted stroke-[1.6]" />
            <label htmlFor="playlist-filter" className="sr-only">
              Filter playlist tracks
            </label>
            <input
              id="playlist-filter"
              type="text"
              value={filterQuery}
              onChange={(e) => setFilterQuery(e.target.value)}
              placeholder="Filter tracks by title or channel..."
              className="h-10 sm:h-8 w-full rounded-[4px] border border-border bg-background pl-8 pr-8 text-xs text-foreground placeholder:text-ink-muted focus:border-signal focus:outline-none"
            />
            {filterQuery && (
              <button
                type="button"
                onClick={() => setFilterQuery("")}
                aria-label="Clear filter"
                className="absolute right-1.5 inline-flex size-6 cursor-pointer items-center justify-center rounded-[2px] text-ink-muted hover:bg-muted hover:text-foreground"
              >
                <X className="size-3.5" />
              </button>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-1.5">
            <Button variant="outline" size="xs" onClick={selectAllFiltered}>
              Select All
            </Button>
            <Button variant="outline" size="xs" onClick={invertSelection}>
              Invert
            </Button>
            <Button variant="ghost" size="xs" onClick={clearSelection}>
              Clear
            </Button>
            <span className="ml-1 font-mono-tabular text-[11px] text-ink-secondary">
              <strong className="font-semibold text-foreground">
                {selectedEntries.length}
              </strong>{" "}
              of {availableIds.length} selected
              {selectedDurationSeconds > 0
                ? ` · ${formatTotalDuration(selectedDurationSeconds)}`
                : ""}
            </span>
          </div>
        </div>

        {/* Row 2: Format Controls & Dispatch CTA */}
        <div className="flex flex-wrap items-center justify-between gap-3 bg-muted/30 px-4 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <ModeSwitch
              value={mediaType}
              onChange={setMediaType}
              layoutGroupId="playlist-mode-switch"
              size="sm"
            />

            {mediaType === "video" ? (
              <>
                <Select
                  value={videoQuality}
                  onValueChange={(v) => {
                    if (v) setVideoQuality(v);
                  }}
                >
                  <SelectTrigger size="sm" aria-label="Playlist video quality">
                    <SelectValue>
                      {VIDEO_QUALITY_OPTIONS.find((o) => o.value === videoQuality)
                        ?.label ?? videoQuality}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_QUALITY_OPTIONS.map((opt) => (
                      <SelectItem key={opt.value} value={opt.value}>
                        {opt.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Select
                  value={videoContainer}
                  onValueChange={(v) => {
                    if (v === "mp4" || v === "mkv" || v === "webm") {
                      setVideoContainer(v);
                    }
                  }}
                >
                  <SelectTrigger size="sm" aria-label="Playlist video container">
                    <SelectValue>
                      {videoContainer.toUpperCase()} · Remux
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="mp4">MP4 · Remux</SelectItem>
                    <SelectItem value="mkv">MKV · Remux</SelectItem>
                    <SelectItem value="webm">WebM · Remux</SelectItem>
                  </SelectContent>
                </Select>
              </>
            ) : (
              <Select
                value={audioPresetKey}
                onValueChange={(v) => {
                  if (v) setAudioPresetKey(v);
                }}
              >
                <SelectTrigger size="sm" aria-label="Playlist audio format">
                  <SelectValue>
                    {AUDIO_FORMAT_OPTIONS.find((o) => o.value === audioPresetKey)
                      ?.label ?? audioPresetKey}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {AUDIO_FORMAT_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}

            <Select
              value={String(concurrency)}
              onValueChange={(v) => {
                if (v) setConcurrency(Number(v));
              }}
            >
              <SelectTrigger size="sm" aria-label="Parallel worker threads">
                <SelectValue>{concurrency}× Parallel</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {concurrencyChoices.map((n) => (
                  <SelectItem key={n} value={String(n)}>
                    {n}× Parallel
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            <label className="inline-flex cursor-pointer items-center gap-1.5 pl-1 text-xs text-ink-secondary select-none">
              <Checkbox
                checked={embedMetadata}
                onCheckedChange={(checked) => setEmbedMetadata(Boolean(checked))}
              />
              <Tag className="size-3 text-ink-muted" />
              <span className="hidden md:inline">Tags</span>
            </label>
          </div>

          <Button
            variant="signal"
            size="default"
            disabled={isSubmitting || isRunning || selectedEntries.length === 0}
            onClick={handleDispatchBatch}
            className="w-full sm:w-auto"
          >
            {isSubmitting || isRunning ? (
              <>
                <Loader2 className="size-4 animate-spin" />
                <span>
                  Downloading {activeJob?.completedTracks ?? 0}/
                  {activeJob?.totalTracks ?? selectedEntries.length}
                </span>
              </>
            ) : (
              <>
                <Archive className="size-4 stroke-[1.6]" />
                <span>
                  Stash {selectedEntries.length}{" "}
                  {selectedEntries.length === 1 ? "Track" : "Tracks"} (.zip)
                </span>
              </>
            )}
          </Button>
        </div>
      </div>

      {/* Virtualized Tracklist Catalog */}
      {filteredEntries.length === 0 ? (
        <div className="px-6 py-12 text-center">
          <p className="text-sm text-ink-secondary">
            No tracks match &ldquo;{filterQuery}&rdquo;.
          </p>
          <Button
            variant="link"
            size="sm"
            onClick={() => setFilterQuery("")}
            className="mt-1"
          >
            Reset filter
          </Button>
        </div>
      ) : (
        <div
          ref={listParentRef}
          className="max-h-[600px] overflow-y-auto"
          role="region"
          aria-label="Playlist tracks"
        >
          <div
            style={{
              height: `${rowVirtualizer.getTotalSize()}px`,
              width: "100%",
              position: "relative",
            }}
          >
            {rowVirtualizer.getVirtualItems().map((virtualRow) => {
              const entry = filteredEntries[virtualRow.index];
              if (!entry) return null;

              const isSelected = selectedIds.has(entry.videoId);
              const liveTrack = trackStateById.get(entry.videoId);

              return (
                <div
                  key={entry.videoId || `row-${entry.index}`}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    height: `${virtualRow.size}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                  className={cn(
                    "relative flex items-center gap-3 border-b border-border px-4 transition-colors",
                    !entry.available && "opacity-50",
                    isSelected && entry.available
                      ? "bg-signal-tint/55"
                      : "bg-card hover:bg-muted/40"
                  )}
                >
                  {/* Checkbox with Shift+Click range selection */}
                  <div
                    className="flex shrink-0 items-center"
                    onClick={(e) => {
                      e.stopPropagation();
                      toggleRowSelection(entry, virtualRow.index, e.shiftKey);
                    }}
                  >
                    <Checkbox
                      checked={isSelected}
                      disabled={!entry.available}
                      aria-label={`Select track ${entry.index}: ${entry.title}`}
                    />
                  </div>

                  {/* Catalog index */}
                  <span className="hidden w-7 shrink-0 text-right font-mono-tabular text-[11px] text-ink-muted sm:inline-block">
                    {String(entry.index).padStart(2, "0")}
                  </span>

                  {/* Micro thumbnail */}
                  <div className="relative h-9 w-16 shrink-0 overflow-hidden rounded-[2px] border border-border bg-muted">
                    {entry.thumbnailUrl && entry.available ? (
                      <Image
                        src={entry.thumbnailUrl}
                        alt=""
                        fill
                        unoptimized
                        sizes="64px"
                        className="object-cover"
                      />
                    ) : null}
                  </div>

                  {/* Title + Channel (clickable to toggle selection) */}
                  <button
                    type="button"
                    disabled={!entry.available}
                    onClick={(e) =>
                      toggleRowSelection(entry, virtualRow.index, e.shiftKey)
                    }
                    className="min-w-0 flex-1 cursor-pointer text-left focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed"
                  >
                    <p className="truncate text-sm font-medium text-foreground">
                      {entry.title}
                    </p>
                    <div className="mt-0.5 flex items-center gap-2 text-xs text-ink-muted">
                      <span className="truncate">{entry.channel}</span>
                      {!entry.available && (
                        <Badge variant="outline">Unavailable</Badge>
                      )}
                    </div>
                  </button>

                  {/* Duration */}
                  <span className="hidden shrink-0 font-mono-tabular text-xs text-ink-secondary md:inline-block">
                    {entry.durationSeconds > 0
                      ? formatDuration(entry.durationSeconds)
                      : "—"}
                  </span>

                  {/* Live Track Status / Individual Actions */}
                  <div className="flex shrink-0 items-center gap-2">
                    {liveTrack ? (
                      <>
                        {liveTrack.status === "completed" && activeJob ? (
                          <Button
                            variant="outline"
                            size="xs"
                            onClick={() =>
                              triggerBrowserDownload(
                                getTrackDownloadUrl(
                                  activeJob.jobId,
                                  entry.videoId
                                )
                              )
                            }
                            aria-label={`Download ${entry.title}`}
                          >
                            <CheckCircle2 className="size-3.5 text-status-ready" />
                            <span className="hidden sm:inline">
                              {formatBytes(liveTrack.outputSizeBytes ?? 0)}
                            </span>
                            <ArrowDownToLine className="size-3" />
                          </Button>
                        ) : liveTrack.status === "failed" && activeJob ? (
                          <div className="flex items-center gap-1.5">
                            <span
                              title={liveTrack.error}
                              className="inline-flex items-center gap-1 font-mono-tabular text-[11px] text-status-error"
                            >
                              <AlertCircle className="size-3.5" />
                              <span className="hidden sm:inline">Failed</span>
                            </span>
                            <Button
                              variant="outline"
                              size="xs"
                              onClick={() =>
                                onRetryTrack(activeJob.jobId, entry.videoId)
                              }
                            >
                              <RotateCcw className="size-3" />
                              <span>Retry</span>
                            </Button>
                          </div>
                        ) : liveTrack.status === "muxing" ? (
                          <Badge variant="warn">MUXING</Badge>
                        ) : liveTrack.status === "downloading" ? (
                          <span className="font-mono-tabular text-[11px] text-foreground">
                            {liveTrack.percent.toFixed(0)}%
                            {liveTrack.speedBytes > 0
                              ? ` · ${formatSpeed(liveTrack.speedBytes)}`
                              : ""}
                          </span>
                        ) : liveTrack.status === "cancelled" ? (
                          <Badge variant="outline">CANCELLED</Badge>
                        ) : (
                          <Badge variant="secondary">QUEUED</Badge>
                        )}
                      </>
                    ) : entry.available ? (
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => handleSingleQuickStash(entry)}
                        aria-label={`Download only ${entry.title}`}
                        title="Download this track individually"
                      >
                        <ArrowDownToLine className="size-3.5 stroke-[1.6]" />
                      </Button>
                    ) : null}
                  </div>

                  {/* Per-row Signature 2px Hairline Telemetry Ruler */}
                  {liveTrack &&
                    (liveTrack.status === "downloading" ||
                      liveTrack.status === "muxing" ||
                      liveTrack.status === "completed" ||
                      liveTrack.status === "failed") && (
                      <div className="pointer-events-none absolute inset-x-0 bottom-0">
                        <TelemetryRuler
                          compact
                          percent={
                            liveTrack.status === "completed"
                              ? 100
                              : liveTrack.percent
                          }
                          tone={
                            liveTrack.status === "completed"
                              ? "ready"
                              : liveTrack.status === "failed"
                                ? "error"
                                : "signal"
                          }
                        />
                      </div>
                    )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </motion.section>
  );
}
