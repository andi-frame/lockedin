"use client";

import { useCallback, useEffect, useReducer, useRef } from "react";
import { MAX_ATTACHMENTS, checkFile, declaredMime, roomLeft, type AttachmentKind } from "@/lib/proof/files";
import { compressImage } from "@/lib/proof/compress";
import { trayReducer, type TrayItem } from "@/lib/proof/tray";
import { uploadAttachment } from "@/lib/proof/upload";

export type Refusal = { name: string; reason: "unsupported" | "empty" | "too_large" | "full"; limitMb?: number };

/**
 * Owns the attachment tray: validates what a person drops or picks, compresses photos, runs each
 * upload through `uploadAttachment`, and keeps one abortable job per file so removing a file (or
 * leaving the page) stops its upload.
 */
export function useAttachments(pactId: string, initial: TrayItem[] = []) {
  const [items, dispatch] = useReducer(trayReducer, initial);
  const jobs = useRef(new Map<string, { controller: AbortController; file: File; kind: AttachmentKind }>());
  const count = useRef(initial.length);
  // Mirrors the size of the tray for `add`, which can run twice before React re-renders.
  useEffect(() => {
    count.current = items.length;
  }, [items.length]);

  const run = useCallback(
    async (localId: string) => {
      const job = jobs.current.get(localId);
      if (!job) return;
      const { signal } = job.controller;
      try {
        const file = job.kind === "image" ? await compressImage(job.file) : job.file;
        if (signal.aborted) return;
        const result = await uploadAttachment(
          { pactId, file, kind: job.kind, mime: declaredMime(file) },
          {
            onProgress: (progress) => dispatch({ type: "progress", localId, progress }),
            onUploaded: (attachmentId) => dispatch({ type: "uploaded", localId, attachmentId }),
          },
          signal,
        );
        if (result.status === "ready") dispatch({ type: "ready", localId, thumb: result.attachment.urls?.thumb ?? result.attachment.urls?.poster ?? null });
        else if (result.status === "rejected") dispatch({ type: "rejected", localId, reason: result.reason });
        else dispatch({ type: "failed", localId, messageKey: result.messageKey });
      } catch {
        // Aborted: the file was removed or the page was left, so there is nothing to report.
      }
    },
    [pactId],
  );

  /** Adds files to the tray and starts them. Returns the ones that were turned away, with why. */
  const add = useCallback(
    (files: File[]): Refusal[] => {
      const refused: Refusal[] = [];
      let room = roomLeft(count.current);
      for (const file of files) {
        const checked = checkFile(file);
        if (!checked.ok) {
          refused.push({ name: file.name, reason: checked.reason, limitMb: "limitMb" in checked ? checked.limitMb : undefined });
          continue;
        }
        if (room === 0) {
          refused.push({ name: file.name, reason: "full", limitMb: MAX_ATTACHMENTS });
          continue;
        }
        room--;
        count.current++;
        const localId = crypto.randomUUID();
        jobs.current.set(localId, { controller: new AbortController(), file, kind: checked.kind });
        dispatch({ type: "add", item: { localId, name: file.name, kind: checked.kind, bytes: file.size } });
        void run(localId);
      }
      return refused;
    },
    [run],
  );

  const retry = useCallback(
    (localId: string) => {
      const job = jobs.current.get(localId);
      if (!job) return;
      job.controller = new AbortController();
      dispatch({ type: "retry", localId });
      void run(localId);
    },
    [run],
  );

  const remove = useCallback((localId: string) => {
    jobs.current.get(localId)?.controller.abort();
    jobs.current.delete(localId);
    dispatch({ type: "remove", localId });
  }, []);

  useEffect(() => {
    const current = jobs.current;
    return () => current.forEach((j) => j.controller.abort());
  }, []);

  return { items, add, retry, remove };
}
