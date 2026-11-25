"use client";

import { useState } from "react";
import Image from "next/image";
import { motion } from "motion/react";
import {
  AlertCircle,
  ArrowDownToLine,
  Check,
  ListMusic,
  Loader2,
  RotateCcw,
  Square,
  Tag,
} from "lucide-react";
import { cn } from "cn";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { ModeSwitch } from "@/components/mode-switch";
import { TelemetryRuler } from "@/components/telemetry-ruler";
import {
  formatBytes,
  formatDuration,
  formatETA,
  formatSpeed,
  formatUploadDate,
  formatViews,
} from "@/lib/format";
import {
  getJobDownloadUrl,
  triggerBrowserDownload,
  type StartSingleJobInput,
} from "@/lib/api";
import type {
  AudioBitrate,
  InspectResponse,
  JobSnapshot,
  MediaType,
  VideoContainer,
} from "@/lib/schemas";

interface SingleMediaDeckProps {
  inspect: InspectResponse;
  initialMediaType: MediaType;
  activeJob: JobSnapshot | null;
  isSubmitting: boolean;
  onStartDownload: (payload: StartSingleJobInput) => void;
  onCancelJob: (jobId: string) => void;
  onLoadFullPlaylist: (playlistUrl: string) => void;
}

const VIDEO_CONTAINERS: Array<{ value: VideoContainer; label: string }> = [
  { value: "mp4", label: "MP4" },
  { value: "mkv", label: "MKV" },
  { value: "webm", label: "WebM" },
];

export function SingleMediaDeck({
  inspect,
  initialMediaType,
  activeJob,
  isSubmitting,
  onStartDownload,
  onCancelJob,
  onLoadFullPlaylist,
}: SingleMediaDeckProps) {
  const video = inspect.video;
  const [mediaType, setMediaType] = useState<MediaType>(initialMediaType);
  const [selectedVideoPresetId, setSelectedVideoPresetId] = useState<string>(
    () => video?.videoPresets[0]?.id ?? ""
  );
  const [videoContainer, setVideoContainer] = useState<VideoContainer>(() => {
    const first = video?.videoPresets[0]?.container;
    if (first === "mkv" || first === "webm" || first === "mp4") return first;
    return "mp4";
  });
  const [selectedAudioPresetId, setSelectedAudioPresetId] = useState<string>(() => {
    const mp3High = video?.audioPresets.find((p) => p.id === "audio-mp3-320");
    return mp3High?.id ?? video?.audioPresets[0]?.id ?? "audio-mp3-320";
  });
  const [embedMetadata, setEmbedMetadata] = useState<boolean>(true);

  if (!video) return null;

  const activeVideoPreset =
    video.videoPresets.find((p) => p.id === selectedVideoPresetId) ??
    video.videoPresets[0];
  const activeAudioPreset =
    video.audioPresets.find((p) => p.id === selectedAudioPresetId) ??
    video.audioPresets[0];

  const isRunning =
    activeJob?.status === "queued" || activeJob?.status === "running";
  const isCompleted = activeJob?.status === "completed";
  const isFailed = activeJob?.status === "failed";
  const track = activeJob?.tracks[0];

  function handleSelectVideoPreset(presetId: string) {
    setSelectedVideoPresetId(presetId);
    const found = video?.videoPresets.find((p) => p.id === presetId);
    if (
      found &&
      (found.container === "mp4" ||
        found.container === "mkv" ||
        found.container === "webm")
    ) {
      setVideoContainer(found.container);
    }
  }

  function handleDispatchDownload() {
    if (!video) return;
    if (mediaType === "video") {
      onStartDownload({
        url: inspect.sourceUrl,
        title: video.title,
        mediaType: "video",
        quality: activeVideoPreset?.quality ?? "1080p",
        container: videoContainer,
        embedMetadata,
      });
      return;
    }

    const container = activeAudioPreset?.container ?? "m4a";
    let audioBitrate: AudioBitrate | undefined;
    if (container === "mp3") {
      audioBitrate =
        activeAudioPreset?.bitrateKbps === 192
          ? "192k"
          : activeAudioPreset?.bitrateKbps === 128
            ? "128k"
            : "320k";
    }

    onStartDownload({
      url: inspect.sourceUrl,
      title: video.title,
      mediaType: "audio",
      container,
      audioBitrate,
      embedMetadata,
    });
  }

  const telemetryLeft = track
    ? track.status === "muxing"
      ? "REMUXING STREAMS · FFMPEG"
      : track.status === "completed"
        ? `ARCHIVED · ${track.outputFilename ?? activeJob?.outputFilename ?? "READY"}`
        : track.status === "failed"
          ? "TRANSFER FAILED"
          : `${activeJob?.mediaType.toUpperCase()} · ${activeJob?.container.toUpperCase()} · ${formatSpeed(track.speedBytes)}`
    : undefined;

  const telemetryRight = track
    ? track.status === "completed"
      ? formatBytes(track.outputSizeBytes ?? 0)
      : track.status === "muxing"
        ? "99%"
        : `${track.percent.toFixed(1)}% · ETA ${formatETA(track.etaSeconds)}`
    : undefined;

  return (
    <motion.section
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
      aria-label="Single video mastering deck"
      className="border border-border bg-card"
    >
      {/* Hybrid URL playlist notice */}
      {inspect.hasPlaylist && inspect.playlistUrl && (
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-signal-tint px-4 py-2.5">
          <div className="flex items-center gap-2 text-xs text-foreground">
            <ListMusic className="size-4 shrink-0 text-signal-ink stroke-[1.6]" />
            <span>
              This link is part of a YouTube playlist. Want to pick multiple
              tracks from it?
            </span>
          </div>
          <Button
            variant="outline"
            size="xs"
            onClick={() => {
              if (inspect.playlistUrl) onLoadFullPlaylist(inspect.playlistUrl);
            }}
          >
            Load Full Playlist
          </Button>
        </div>
      )}

      {/* Live Telemetry Ruler across top of deck when transfer is active/completed */}
      {activeJob && (
        <div className="border-b border-border bg-muted/40 px-4 py-2.5">
          <TelemetryRuler
            percent={isCompleted ? 100 : activeJob.overallPercent}
            tone={isCompleted ? "ready" : isFailed ? "error" : "signal"}
            leftLabel={telemetryLeft}
            rightLabel={telemetryRight}
          />
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-12">
        {/* Left Plate: Preview & Editorial Metadata */}
        <div className="border-b border-border p-5 lg:col-span-5 lg:border-r lg:border-b-0">
          <div className="relative aspect-video w-full overflow-hidden rounded-[2px] border border-border bg-muted">
            {video.thumbnailUrl ? (
              <Image
                src={video.thumbnailUrl}
                alt={video.title}
                fill
                unoptimized
                sizes="(max-width: 1024px) 100vw, 420px"
                className="object-cover"
              />
            ) : null}
            {video.durationSeconds > 0 && (
              <span className="absolute right-2 bottom-2 rounded-[2px] bg-[#161513]/90 px-1.5 py-0.5 font-mono-tabular text-[11px] font-medium text-[#F6F4EF]">
                {formatDuration(video.durationSeconds)}
              </span>
            )}
          </div>

          <div className="mt-4 space-y-2.5">
            <div className="flex flex-wrap items-center gap-1.5">
              <Badge variant="secondary">ID {video.videoId}</Badge>
              {inspect.cached && <Badge variant="ready">CACHED · INSTANT</Badge>}
              {video.isLive && <Badge variant="destructive">LIVE</Badge>}
            </div>

            <h1 className="font-display text-[1.75rem] leading-[1.15] tracking-[-0.01em] text-foreground">
              {video.title}
            </h1>

            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-secondary">
              <span className="font-medium text-foreground">{video.channel}</span>
              {video.uploadDate && (
                <>
                  <span aria-hidden="true">·</span>
                  <span className="font-mono-tabular">
                    {formatUploadDate(video.uploadDate)}
                  </span>
                </>
              )}
              {video.viewCount ? (
                <>
                  <span aria-hidden="true">·</span>
                  <span className="font-mono-tabular">
                    {formatViews(video.viewCount)}
                  </span>
                </>
              ) : null}
            </div>
          </div>
        </div>

        {/* Right Plate: Mastering Controls & Format Matrix */}
        <div className="flex flex-col p-5 lg:col-span-7">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4">
            <ModeSwitch
              value={mediaType}
              onChange={setMediaType}
              layoutGroupId="single-deck-mode"
            />

            {mediaType === "video" && (
              <div className="flex items-center gap-1.5">
                <span className="font-mono-tabular text-[11px] uppercase tracking-[0.06em] text-ink-muted">
                  Container
                </span>
                <div
                  role="radiogroup"
                  aria-label="Video container"
                  className="inline-flex rounded-[4px] border border-border bg-muted p-0.5"
                >
                  {VIDEO_CONTAINERS.map((c) => (
                    <button
                      key={c.value}
                      type="button"
                      role="radio"
                      aria-checked={videoContainer === c.value}
                      onClick={() => setVideoContainer(c.value)}
                      className={cn(
                        "min-h-8 sm:min-h-7 cursor-pointer rounded-[2px] px-2.5 font-mono-tabular text-[11px] font-medium transition-colors",
                        videoContainer === c.value
                          ? "bg-card text-foreground shadow-[0_1px_1px_rgba(22,21,19,0.06)]"
                          : "text-ink-secondary hover:text-foreground"
                      )}
                    >
                      {c.label}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Preset List */}
          <div className="mt-4 flex-1">
            <div className="mb-2 flex items-center justify-between font-mono-tabular text-[11px] uppercase tracking-[0.06em] text-ink-muted">
              <span>
                {mediaType === "video"
                  ? "Available Video Streams"
                  : "Audio Mastering Presets"}
              </span>
              <span>Est. Size</span>
            </div>

            {mediaType === "video" ? (
              <div
                role="radiogroup"
                aria-label="Video quality presets"
                className="divide-y divide-border border border-border"
              >
                {video.videoPresets.map((preset) => {
                  const selected = activeVideoPreset?.id === preset.id;
                  return (
                    <button
                      key={preset.id}
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      onClick={() => handleSelectVideoPreset(preset.id)}
                      className={cn(
                        "flex min-h-12 w-full cursor-pointer items-center justify-between gap-3 px-3.5 py-2.5 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring",
                        selected
                          ? "bg-signal-tint/80"
                          : "bg-card hover:bg-muted/50"
                      )}
                    >
                      <div className="flex items-center gap-3 min-w-0">
                        <span
                          className={cn(
                            "flex size-4 shrink-0 items-center justify-center rounded-[2px] border",
                            selected
                              ? "border-signal bg-signal text-white"
                              : "border-border-strong bg-card"
                          )}
                        >
                          {selected && <Check className="size-3 stroke-[2.5]" />}
                        </span>
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-medium text-foreground">
                              {preset.label}
                            </span>
                            <Badge variant="ready">REMUX · FAST</Badge>
                          </div>
                          <p className="mt-0.5 font-mono-tabular text-[11px] text-ink-muted">
                            {preset.videoCodec} + {preset.audioCodec}
                          </p>
                        </div>
                      </div>
                      <span className="shrink-0 font-mono-tabular text-xs text-ink-secondary">
                        {formatBytes(preset.filesizeApprox)}
                      </span>
                    </button>
                  );
                })}
              </div>
            ) : (
              <div
                role="radiogroup"
                aria-label="Audio format presets"
                className="divide-y divide-border border border-border"
              >
                {video.audioPresets.map((preset) => {
                  const selected = activeAudioPreset?.id === preset.id;
                  return (
                    <button
                      key={preset.id}
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      onClick={() => setSelectedAudioPresetId(preset.id)}
                      className={cn(
                        "flex min-h-12 w-full cursor-pointer items-center justify-between gap-3 px-3.5 py-2.5 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring",
                        selected
                          ? "bg-signal-tint/80"
                          : "bg-card hover:bg-muted/50"
                      )}
                    >
                      <div className="flex items-center gap-3 min-w-0">
                        <span
                          className={cn(
                            "flex size-4 shrink-0 items-center justify-center rounded-[2px] border",
                            selected
                              ? "border-signal bg-signal text-white"
                              : "border-border-strong bg-card"
                          )}
                        >
                          {selected && <Check className="size-3 stroke-[2.5]" />}
                        </span>
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-medium text-foreground">
                              {preset.label}
                            </span>
                            {preset.fastRemux ? (
                              <Badge variant="ready">REMUX · FAST</Badge>
                            ) : (
                              <Badge variant="outline">TRANSCODE</Badge>
                            )}
                          </div>
                          <p className="mt-0.5 font-mono-tabular text-[11px] text-ink-muted">
                            {preset.codec} · {preset.bitrateLabel}
                          </p>
                        </div>
                      </div>
                      <span className="shrink-0 font-mono-tabular text-xs text-ink-secondary">
                        {formatBytes(preset.filesizeApprox)}
                      </span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {/* Options & Primary Action */}
          <div className="mt-5 space-y-3 border-t border-border pt-4">
            <label className="inline-flex cursor-pointer items-center gap-2.5 text-xs text-ink-secondary select-none">
              <Checkbox
                checked={embedMetadata}
                onCheckedChange={(checked) => setEmbedMetadata(Boolean(checked))}
              />
              <Tag className="size-3.5 text-ink-muted stroke-[1.5]" />
              <span>Embed title, channel tags, and cover artwork</span>
            </label>

            {isFailed && (
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-[4px] border border-status-error/30 bg-status-error/10 p-3 text-xs text-status-error"
              >
                <AlertCircle className="mt-0.5 size-4 shrink-0 stroke-[1.75]" />
                <span>
                  {activeJob?.error ??
                    track?.error ??
                    "Transfer failed. Try another format or retry."}
                </span>
              </div>
            )}

            <div className="flex flex-wrap items-center gap-2.5">
              {isRunning && activeJob ? (
                <Button
                  variant="destructive"
                  size="lg"
                  onClick={() => onCancelJob(activeJob.jobId)}
                  className="w-full sm:w-auto"
                >
                  <Square className="size-3.5 fill-current" />
                  <span>Cancel Transfer</span>
                </Button>
              ) : (
                <Button
                  variant="signal"
                  size="lg"
                  disabled={isSubmitting || video.isLive}
                  onClick={handleDispatchDownload}
                  className="w-full sm:flex-1"
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 className="size-4 animate-spin" />
                      <span>Starting Transfer</span>
                    </>
                  ) : isFailed ? (
                    <>
                      <RotateCcw className="size-4 stroke-[1.6]" />
                      <span>Retry Transfer</span>
                    </>
                  ) : (
                    <>
                      <ArrowDownToLine className="size-4 stroke-[1.75]" />
                      <span>
                        {mediaType === "video"
                          ? `Stash ${activeVideoPreset?.quality ?? "Video"} (${videoContainer.toUpperCase()})`
                          : `Stash Audio (${(activeAudioPreset?.container ?? "m4a").toUpperCase()})`}
                      </span>
                    </>
                  )}
                </Button>
              )}

              {isCompleted && activeJob && (
                <Button
                  variant="default"
                  size="lg"
                  onClick={() =>
                    triggerBrowserDownload(getJobDownloadUrl(activeJob.jobId))
                  }
                  className="w-full sm:w-auto"
                >
                  <ArrowDownToLine className="size-4 stroke-[1.75]" />
                  <span>
                    Save File ({formatBytes(activeJob.outputSizeBytes ?? 0)})
                  </span>
                </Button>
              )}
            </div>
          </div>
        </div>
      </div>
    </motion.section>
  );
}
