// Pre-checks for a file before it is sent (SPEC §8). The server sniffs the real bytes and enforces
// the limits; this only saves a person from waiting on an upload that is bound to be refused.

export type AttachmentKind = "image" | "video" | "file";

export const MAX_ATTACHMENTS = 10;
const MB = 1024 * 1024;
const limitMb: Record<AttachmentKind, number> = { image: 15, video: 200, file: 20 };

const byMime: Record<string, AttachmentKind> = {
  "image/jpeg": "image",
  "image/png": "image",
  "image/webp": "image",
  "image/heic": "image",
  "image/heif": "image",
  "image/avif": "image",
  "image/gif": "image",
  "video/mp4": "video",
  "video/quicktime": "video",
  "video/webm": "video",
  "application/pdf": "file",
};

// Some phones hand over HEIC or MOV with an empty type, so the extension is the second chance.
const byExtension: Record<string, AttachmentKind> = {
  jpg: "image", jpeg: "image", png: "image", webp: "image", heic: "image", heif: "image", avif: "image", gif: "image",
  mp4: "video", mov: "video", webm: "video",
  pdf: "file",
};

export type FileCheck =
  | { ok: true; kind: AttachmentKind }
  | { ok: false; reason: "unsupported" | "empty" }
  | { ok: false; reason: "too_large"; limitMb: number };

export function checkFile(file: { name: string; type: string; size: number }): FileCheck {
  const ext = file.name.includes(".") ? file.name.split(".").pop()?.toLowerCase() : undefined;
  const kind = byMime[file.type.toLowerCase()] ?? (file.type === "" && ext ? byExtension[ext] : undefined);
  if (!kind) return { ok: false, reason: "unsupported" };
  if (file.size <= 0) return { ok: false, reason: "empty" };
  if (file.size > limitMb[kind] * MB) return { ok: false, reason: "too_large", limitMb: limitMb[kind] };
  return { ok: true, kind };
}

export const roomLeft = (current: number) => Math.max(0, MAX_ATTACHMENTS - current);
