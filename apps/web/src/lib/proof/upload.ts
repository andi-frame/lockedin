import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";
import { unwrap } from "@/lib/api/unwrap";
import type { AttachmentKind } from "./files";
import { pollDelay, putHeaders } from "./net";

type Attachment = components["schemas"]["Attachment"];

/** The PUT to object storage, with progress (fetch cannot report upload progress). */
export function putFile(url: string, headers: Record<string, string>, body: Blob, onProgress: (fraction: number) => void, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    for (const [k, v] of Object.entries(putHeaders(headers))) xhr.setRequestHeader(k, v);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(e.loaded / e.total);
    };
    xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`put ${xhr.status}`)));
    xhr.onerror = () => reject(new Error("put network"));
    xhr.onabort = () => reject(new DOMException("aborted", "AbortError"));
    signal.addEventListener("abort", () => xhr.abort(), { once: true });
    xhr.send(body);
  });
}

export type UploadEvents = {
  onProgress: (fraction: number) => void;
  onUploaded: (attachmentId: string) => void;
};

export type UploadResult =
  | { status: "ready"; attachment: Attachment }
  | { status: "rejected"; reason: string }
  | { status: "failed"; messageKey: string };

const sleep = (ms: number, signal: AbortSignal) =>
  new Promise<void>((resolve, reject) => {
    const id = setTimeout(resolve, ms);
    signal.addEventListener("abort", () => (clearTimeout(id), reject(new DOMException("aborted", "AbortError"))), { once: true });
  });

/**
 * The flow of docs/STATUS.md: ask for a slot, PUT the bytes straight to storage with exactly the
 * headers given, tell the API it finished, then poll until the server has made its verdict.
 * Anything that goes wrong comes back as a message key under `Errors`; the caller can retry.
 */
export async function uploadAttachment(
  input: { pactId: string; file: Blob; kind: AttachmentKind; mime: string },
  events: UploadEvents,
  signal: AbortSignal,
): Promise<UploadResult> {
  try {
    const intent = await unwrap(
      api.POST("/uploads", { body: { pact_id: input.pactId, kind: input.kind, mime: input.mime, bytes: input.file.size } }),
    );
    try {
      await putFile(intent.put_url, intent.headers, input.file, events.onProgress, signal);
    } catch (err) {
      if (err instanceof DOMException) throw err;
      return { status: "failed", messageKey: "Errors.upload_failed" };
    }
    events.onUploaded(intent.attachment_id);
    let att = await unwrap(api.POST("/uploads/{attachmentId}/complete", { params: { path: { attachmentId: intent.attachment_id } } }));
    for (let attempt = 0; att.status !== "ready" && att.status !== "rejected"; attempt++) {
      await sleep(pollDelay(attempt), signal);
      att = await unwrap(api.GET("/attachments/{attachmentId}", { params: { path: { attachmentId: intent.attachment_id } } }));
    }
    return att.status === "ready"
      ? { status: "ready", attachment: att }
      : { status: "rejected", reason: att.reject_reason ?? "" };
  } catch (err) {
    if (err instanceof DOMException) throw err;
    return { status: "failed", messageKey: `Errors.${errorMessageKey(err instanceof ApiError ? err.code : "unknown")}` };
  }
}
