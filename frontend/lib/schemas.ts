import { z } from "zod";

export const MediaTypeSchema = z.enum(["video", "audio"]);
export type MediaType = z.infer<typeof MediaTypeSchema>;

export const VideoContainerSchema = z.enum(["mp4", "mkv", "webm"]);
export type VideoContainer = z.infer<typeof VideoContainerSchema>;

export const AudioContainerSchema = z.enum(["m4a", "opus", "mp3", "flac", "wav"]);
export type AudioContainer = z.infer<typeof AudioContainerSchema>;

export const VideoQualitySchema = z.enum([
  "best",
  "2160p",
  "1440p",
  "1080p",
  "720p",
  "480p",
  "360p",
]);
export type VideoQuality = z.infer<typeof VideoQualitySchema>;

export const AudioBitrateSchema = z.enum(["320k", "192k", "128k"]);
export type AudioBitrate = z.infer<typeof AudioBitrateSchema>;

export const ApiErrorSchema = z.object({
  error: z.object({
    code: z.string(),
    message: z.string(),
  }),
});

export const HealthResponseSchema = z.object({
  status: z.string(),
  ytDlpVersion: z.string(),
  ffmpegAvailable: z.boolean(),
  ffmpegVersion: z.string().optional(),
  jsRuntime: z.string(),
  potProviderEnabled: z.boolean(),
  potProviderHealthy: z.boolean(),
  proxyEnabled: z.boolean().optional(),
  proxyHealthy: z.boolean().optional(),
  cookiesConfigured: z.boolean().optional(),
  cookiesStatus: z.string().optional(),
  maxConcurrentTracks: z.number(),
  activeJobs: z.number(),
});
export type HealthResponse = z.infer<typeof HealthResponseSchema>;

export const VideoPresetSchema = z.object({
  id: z.string(),
  quality: z.string(),
  label: z.string(),
  height: z.number(),
  fps: z.number().optional(),
  container: z.string(),
  videoCodec: z.string(),
  audioCodec: z.string(),
  filesizeApprox: z.number(),
  fastRemux: z.boolean(),
});
export type VideoPreset = z.infer<typeof VideoPresetSchema>;

export const AudioPresetSchema = z.object({
  id: z.string(),
  label: z.string(),
  container: z.string(),
  codec: z.string(),
  bitrateKbps: z.number().optional(),
  bitrateLabel: z.string(),
  filesizeApprox: z.number(),
  fastRemux: z.boolean(),
});
export type AudioPreset = z.infer<typeof AudioPresetSchema>;

export const VideoMetadataSchema = z.object({
  videoId: z.string(),
  title: z.string(),
  channel: z.string(),
  durationSeconds: z.number(),
  thumbnailUrl: z.string(),
  uploadDate: z.string().optional(),
  viewCount: z.number().optional(),
  isLive: z.boolean(),
  videoPresets: z.array(VideoPresetSchema),
  audioPresets: z.array(AudioPresetSchema),
});
export type VideoMetadata = z.infer<typeof VideoMetadataSchema>;

export const PlaylistEntrySchema = z.object({
  index: z.number(),
  videoId: z.string(),
  title: z.string(),
  channel: z.string(),
  durationSeconds: z.number(),
  thumbnailUrl: z.string(),
  available: z.boolean(),
});
export type PlaylistEntry = z.infer<typeof PlaylistEntrySchema>;

export const PlaylistMetadataSchema = z.object({
  playlistId: z.string(),
  title: z.string(),
  channel: z.string(),
  thumbnailUrl: z.string(),
  entryCount: z.number(),
  totalDurationSeconds: z.number(),
  entries: z.array(PlaylistEntrySchema),
});
export type PlaylistMetadata = z.infer<typeof PlaylistMetadataSchema>;

export const InspectResponseSchema = z.object({
  kind: z.enum(["video", "playlist"]),
  sourceUrl: z.string(),
  hasPlaylist: z.boolean(),
  playlistUrl: z.string().optional(),
  cached: z.boolean(),
  video: VideoMetadataSchema.optional(),
  playlist: PlaylistMetadataSchema.optional(),
});
export type InspectResponse = z.infer<typeof InspectResponseSchema>;

export const TrackStatusSchema = z.enum([
  "queued",
  "downloading",
  "muxing",
  "completed",
  "failed",
  "cancelled",
]);
export type TrackStatus = z.infer<typeof TrackStatusSchema>;

export const JobStatusSchema = z.enum([
  "queued",
  "running",
  "completed",
  "partial",
  "failed",
  "cancelled",
]);
export type JobStatus = z.infer<typeof JobStatusSchema>;

export const TrackStateSchema = z.object({
  index: z.number(),
  videoId: z.string(),
  title: z.string(),
  status: TrackStatusSchema,
  percent: z.number(),
  downloadedBytes: z.number(),
  totalBytes: z.number(),
  speedBytes: z.number(),
  etaSeconds: z.number(),
  outputFilename: z.string().optional(),
  outputSizeBytes: z.number().optional(),
  error: z.string().optional(),
});
export type TrackState = z.infer<typeof TrackStateSchema>;

export const JobSnapshotSchema = z.object({
  jobId: z.string(),
  kind: z.enum(["single", "playlist"]),
  title: z.string(),
  mediaType: MediaTypeSchema,
  quality: z.string().optional(),
  container: z.string(),
  audioBitrate: z.string().optional(),
  status: JobStatusSchema,
  createdAt: z.string(),
  completedAt: z.string().optional(),
  totalTracks: z.number(),
  completedTracks: z.number(),
  failedTracks: z.number(),
  overallPercent: z.number(),
  aggregateSpeedBytes: z.number(),
  outputFilename: z.string().optional(),
  outputSizeBytes: z.number().optional(),
  error: z.string().optional(),
  tracks: z.array(TrackStateSchema),
});
export type JobSnapshot = z.infer<typeof JobSnapshotSchema>;

export const ProgressEventSchema = z.object({
  type: z.enum(["snapshot", "update", "done"]),
  job: JobSnapshotSchema,
  track: TrackStateSchema.optional(),
});
export type ProgressEvent = z.infer<typeof ProgressEventSchema>;

export type DetectedSourceKind = "empty" | "video" | "playlist" | "hybrid" | "invalid";

const ALLOWED_YT_HOSTS = new Set([
  "youtube.com",
  "www.youtube.com",
  "m.youtube.com",
  "music.youtube.com",
  "youtu.be",
]);

const VIDEO_ID_RE = /^[A-Za-z0-9_-]{11}$/;
const LIST_ID_RE = /^[A-Za-z0-9_-]{10,128}$/;

export function detectYouTubeInput(raw: string): DetectedSourceKind {
  const trimmed = raw.trim();
  if (!trimmed) return "empty";

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return "invalid";
  }

  if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
    return "invalid";
  }
  const host = parsed.hostname.toLowerCase();
  if (!ALLOWED_YT_HOSTS.has(host)) {
    return "invalid";
  }

  const listParam = parsed.searchParams.get("list")?.trim() ?? "";
  const hasList = LIST_ID_RE.test(listParam);

  let hasVideo = false;
  const segments = parsed.pathname.split("/").filter(Boolean);
  if (host === "youtu.be") {
    hasVideo = segments.length >= 1 && VIDEO_ID_RE.test(segments[0]);
  } else if (segments[0] === "watch") {
    const v = parsed.searchParams.get("v")?.trim() ?? "";
    hasVideo = VIDEO_ID_RE.test(v);
  } else if (
    segments.length >= 2 &&
    (segments[0] === "shorts" || segments[0] === "live" || segments[0] === "embed")
  ) {
    hasVideo = VIDEO_ID_RE.test(segments[1]);
  }

  if (hasVideo && hasList) return "hybrid";
  if (hasList) return "playlist";
  if (hasVideo) return "video";
  return "invalid";
}
