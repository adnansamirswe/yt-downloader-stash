import {
  ApiErrorSchema,
  HealthResponseSchema,
  InspectResponseSchema,
  JobSnapshotSchema,
  ProgressEventSchema,
  type AudioBitrate,
  type HealthResponse,
  type InspectResponse,
  type JobSnapshot,
  type MediaType,
  type ProgressEvent,
} from "./schemas";

export class StashApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(message: string, code = "api_error", status = 500) {
    super(message);
    this.name = "StashApiError";
    this.code = code;
    this.status = status;
  }
}

export function getApiBaseUrl(): string {
  return (process.env.NEXT_PUBLIC_API_URL ?? "").trim().replace(/\/+$/, "");
}

function apiUrl(path: string): string {
  return `${getApiBaseUrl()}${path}`;
}

async function parseErrorResponse(res: Response): Promise<StashApiError> {
  try {
    const raw: unknown = await res.json();
    const parsed = ApiErrorSchema.safeParse(raw);
    if (parsed.success) {
      return new StashApiError(
        parsed.data.error.message,
        parsed.data.error.code,
        res.status
      );
    }
  } catch {
    // Non-JSON upstream error (e.g. gateway 502)
  }
  return new StashApiError(
    res.status >= 500
      ? "Stash backend is temporarily unavailable."
      : `Request failed with status ${res.status}.`,
    "http_error",
    res.status
  );
}

export async function fetchHealth(signal?: AbortSignal): Promise<HealthResponse> {
  const res = await fetch(apiUrl("/api/v1/health"), {
    method: "GET",
    cache: "no-store",
    signal,
  });
  if (!res.ok) throw await parseErrorResponse(res);
  const raw: unknown = await res.json();
  return HealthResponseSchema.parse(raw);
}

export async function inspectUrl(
  url: string,
  mode: "auto" | "video" | "playlist" = "auto",
  signal?: AbortSignal
): Promise<InspectResponse> {
  const res = await fetch(apiUrl("/api/v1/inspect"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, mode }),
    signal,
  });
  if (!res.ok) throw await parseErrorResponse(res);
  const raw: unknown = await res.json();
  return InspectResponseSchema.parse(raw);
}

export interface StartSingleJobInput {
  url: string;
  title?: string;
  mediaType: MediaType;
  quality?: string;
  container: string;
  audioBitrate?: AudioBitrate;
  embedMetadata?: boolean;
}

export async function startSingleJob(input: StartSingleJobInput): Promise<JobSnapshot> {
  const res = await fetch(apiUrl("/api/v1/jobs/single"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!res.ok) throw await parseErrorResponse(res);
  const raw: unknown = await res.json();
  return JobSnapshotSchema.parse(raw);
}

export interface StartPlaylistJobInput {
  playlistUrl: string;
  playlistTitle?: string;
  items: Array<{ videoId: string; title: string; index: number }>;
  mediaType: MediaType;
  quality?: string;
  container: string;
  audioBitrate?: AudioBitrate;
  concurrency?: number;
  embedMetadata?: boolean;
}

export async function startPlaylistJob(
  input: StartPlaylistJobInput
): Promise<JobSnapshot> {
  const res = await fetch(apiUrl("/api/v1/jobs/playlist"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!res.ok) throw await parseErrorResponse(res);
  const raw: unknown = await res.json();
  return JobSnapshotSchema.parse(raw);
}

export async function fetchJobSnapshot(jobId: string): Promise<JobSnapshot> {
  const res = await fetch(apiUrl(`/api/v1/jobs/${encodeURIComponent(jobId)}`), {
    method: "GET",
    cache: "no-store",
  });
  if (!res.ok) throw await parseErrorResponse(res);
  const raw: unknown = await res.json();
  return JobSnapshotSchema.parse(raw);
}

export async function cancelJob(jobId: string): Promise<JobSnapshot> {
  const res = await fetch(
    apiUrl(`/api/v1/jobs/${encodeURIComponent(jobId)}/cancel`),
    { method: "POST" }
  );
  if (!res.ok) throw await parseErrorResponse(res);
  const raw: unknown = await res.json();
  return JobSnapshotSchema.parse(raw);
}

export async function retryJobTrack(
  jobId: string,
  videoId: string
): Promise<JobSnapshot> {
  const res = await fetch(
    apiUrl(
      `/api/v1/jobs/${encodeURIComponent(jobId)}/tracks/${encodeURIComponent(videoId)}/retry`
    ),
    { method: "POST" }
  );
  if (!res.ok) throw await parseErrorResponse(res);
  const raw: unknown = await res.json();
  return JobSnapshotSchema.parse(raw);
}

export function getJobDownloadUrl(jobId: string): string {
  return apiUrl(`/api/v1/jobs/${encodeURIComponent(jobId)}/download`);
}

export function getTrackDownloadUrl(jobId: string, videoId: string): string {
  return apiUrl(
    `/api/v1/jobs/${encodeURIComponent(jobId)}/tracks/${encodeURIComponent(videoId)}/download`
  );
}

export function triggerBrowserDownload(downloadUrl: string): void {
  const anchor = document.createElement("a");
  anchor.href = downloadUrl;
  anchor.rel = "noopener noreferrer";
  anchor.style.display = "none";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}

export function subscribeToJobEvents(
  jobId: string,
  onEvent: (event: ProgressEvent) => void,
  onConnectionError?: () => void
): () => void {
  const es = new EventSource(
    apiUrl(`/api/v1/jobs/${encodeURIComponent(jobId)}/events`)
  );

  const handleMessage = (ev: Event) => {
    if (!(ev instanceof MessageEvent) || typeof ev.data !== "string") {
      return;
    }
    try {
      const raw: unknown = JSON.parse(ev.data);
      const parsed = ProgressEventSchema.safeParse(raw);
      if (parsed.success) {
        onEvent(parsed.data);
        if (
          parsed.data.type === "done" &&
          (parsed.data.job.status === "completed" ||
            parsed.data.job.status === "cancelled")
        ) {
          es.close();
        }
      }
    } catch {
      // Ignore malformed heartbeat frames
    }
  };

  es.addEventListener("snapshot", handleMessage);
  es.addEventListener("update", handleMessage);
  es.addEventListener("done", handleMessage);

  es.onerror = () => {
    if (es.readyState === EventSource.CLOSED) {
      onConnectionError?.();
    }
  };

  return () => {
    es.close();
  };
}
