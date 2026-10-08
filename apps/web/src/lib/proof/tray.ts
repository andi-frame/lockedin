import type { AttachmentKind } from "./files";

// The attachment tray's state. Each file moves preparing -> uploading -> processing -> ready, or
// ends rejected (the server said no, with a reason) or failed (something broke on the way and the
// person can retry). Only ready files go out with the proof.

export type TrayItem = {
  /** Identifies the file before the server has given it an id. */
  localId: string;
  name: string;
  kind: AttachmentKind;
  bytes: number;
  attachmentId?: string;
  thumb?: string | null;
} & (
  | { state: "working"; step: "preparing" | "uploading" | "processing"; progress: number }
  | { state: "ready" }
  | { state: "rejected"; reason: string }
  | { state: "failed"; messageKey: string }
);

type NewItem = { localId: string; name: string; kind: AttachmentKind; bytes: number };

export type TrayAction =
  | { type: "add"; item: NewItem }
  | { type: "existing"; item: NewItem & { attachmentId: string; thumb: string | null } }
  | { type: "progress"; localId: string; progress: number }
  | { type: "uploaded"; localId: string; attachmentId: string }
  | { type: "ready"; localId: string; thumb: string | null }
  | { type: "rejected"; localId: string; reason: string }
  | { type: "failed"; localId: string; messageKey: string }
  | { type: "retry"; localId: string }
  | { type: "remove"; localId: string };

const clamp01 = (n: number) => Math.min(1, Math.max(0, n));

export function trayReducer(state: TrayItem[], action: TrayAction): TrayItem[] {
  switch (action.type) {
    case "add":
      return [...state, { ...action.item, state: "working", step: "preparing", progress: 0 }];
    case "existing":
      return [...state, { ...action.item, state: "ready" }];
    case "remove":
      return state.filter((i) => i.localId !== action.localId);
    default: {
      const a = action;
      return state.map((i): TrayItem => {
        if (i.localId !== a.localId) return i;
        const { localId, name, kind, bytes, attachmentId, thumb } = i;
        const base = { localId, name, kind, bytes, attachmentId, thumb };
        switch (a.type) {
          case "progress":
            return { ...base, state: "working", step: "uploading", progress: clamp01(a.progress) };
          case "uploaded":
            return { ...base, attachmentId: a.attachmentId, state: "working", step: "processing", progress: 1 };
          case "ready":
            return { ...base, thumb: a.thumb, state: "ready" };
          case "rejected":
            return { ...base, state: "rejected", reason: a.reason };
          case "failed":
            return { ...base, state: "failed", messageKey: a.messageKey };
          case "retry":
            return { localId, name, kind, bytes, state: "working", step: "preparing", progress: 0 };
        }
      });
    }
  }
}

export function trayCounts(items: TrayItem[]): { ready: number; pending: number; rejected: number; failed: number } {
  const c = { ready: 0, pending: 0, rejected: 0, failed: 0 };
  for (const i of items) {
    if (i.state === "ready") c.ready++;
    else if (i.state === "working") c.pending++;
    else if (i.state === "rejected") c.rejected++;
    else c.failed++;
  }
  return c;
}

/** The attachments that go out with the proof, in tray order. */
export function readyIds(items: TrayItem[]): string[] {
  return items.flatMap((i) => (i.state === "ready" && i.attachmentId ? [i.attachmentId] : []));
}
