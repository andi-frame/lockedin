"use client";

import { ArrowClockwise, Camera, CheckCircle, File as FileIcon, FilmSlate, Image as ImageIcon, Paperclip, UploadSimple, WarningCircle, X } from "@phosphor-icons/react";
import { useFormatter, useTranslations } from "next-intl";
import { useRef, useState, type ChangeEvent, type DragEvent } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import { MAX_ATTACHMENTS } from "@/lib/proof/files";
import type { TrayItem } from "@/lib/proof/tray";
import type { Refusal } from "./use-attachments";

const icons = { image: ImageIcon, video: FilmSlate, file: FileIcon } as const;

/**
 * Where a proof's photos, videos and PDFs go: pick, take a photo, drop, or paste (the editor hands
 * pasted files over too). Each file shows its own stage and nothing blocks the others.
 */
export function AttachmentTray({
  items,
  refusals,
  onAdd,
  onRetry,
  onRemove,
  labelId,
}: {
  items: TrayItem[];
  /** Files turned away by the last attempt, with why. */
  refusals: Refusal[];
  onAdd: (files: File[]) => void;
  onRetry: (localId: string) => void;
  onRemove: (localId: string) => void;
  labelId: string;
}) {
  const t = useTranslations("Proof");
  const f = useFormatter();
  const pick = useRef<HTMLInputElement>(null);
  const camera = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);

  const onPick = (e: ChangeEvent<HTMLInputElement>) => {
    onAdd(Array.from(e.target.files ?? []));
    e.target.value = ""; // so choosing the same file again still fires
  };
  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setOver(false);
    onAdd(Array.from(e.dataTransfer.files));
  };
  const size = (bytes: number) => `${f.number(Math.max(0.1, bytes / 1048576), { maximumFractionDigits: 1 })} MB`;

  return (
    <div className="flex flex-col gap-3">
      <div
        onDragOver={(e) => (e.preventDefault(), setOver(true))}
        onDragLeave={() => setOver(false)}
        onDrop={onDrop}
        className={cn(
          "flex flex-col items-start gap-3 rounded-control border border-dashed border-rule-strong p-3 transition-colors sm:flex-row sm:items-center",
          over && "border-ring bg-sunken",
        )}
      >
        <UploadSimple aria-hidden weight="bold" className="hidden size-5 shrink-0 text-muted sm:block" />
        <p id={labelId} className="min-w-0 flex-1 text-sm text-muted">
          {t("trayHint", { max: MAX_ATTACHMENTS })}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={() => pick.current?.click()}>
            <Paperclip aria-hidden weight="bold" className="size-4" />
            {t("pick")}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => camera.current?.click()}>
            <Camera aria-hidden weight="bold" className="size-4" />
            {t("camera")}
          </Button>
        </div>
        <input ref={pick} type="file" multiple hidden accept="image/*,video/mp4,video/quicktime,video/webm,application/pdf,.heic,.heif,.avif,.mov" onChange={onPick} data-testid="tray-input" />
        <input ref={camera} type="file" hidden accept="image/*,video/*" capture="environment" onChange={onPick} />
      </div>

      {refusals.length > 0 ? (
        <ul role="alert" className="flex flex-col gap-1 text-sm font-medium text-debit">
          {refusals.map((r, i) => (
            <li key={i}>
              {t(`refused_${r.reason}`, { name: r.name, mb: r.limitMb ?? 0 })}
            </li>
          ))}
        </ul>
      ) : null}

      {items.length > 0 ? (
        <ul className="flex flex-col divide-y divide-rule border-y border-rule">
          {items.map((it) => {
            const Icon = icons[it.kind];
            return (
              <li key={it.localId} className="flex items-start gap-3 py-3">
                {it.thumb ? (
                  // Signed URL from the API; a plain img keeps it out of the image optimizer.
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={it.thumb} alt="" className="size-12 shrink-0 rounded-control border border-rule object-cover" />
                ) : (
                  <span className="flex size-12 shrink-0 items-center justify-center rounded-control border border-rule bg-sunken text-muted">
                    <Icon aria-hidden weight="bold" className="size-5" />
                  </span>
                )}
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[15px] font-medium">{it.name}</p>
                  <p className="font-mono text-xs tabular-nums text-muted">{size(it.bytes)}</p>
                  <Status item={it} onRetry={() => onRetry(it.localId)} />
                </div>
                <Button variant="ghost" size="icon-sm" aria-label={t("remove", { name: it.name })} onClick={() => onRemove(it.localId)}>
                  <X aria-hidden weight="bold" className="size-4" />
                </Button>
              </li>
            );
          })}
        </ul>
      ) : null}
    </div>
  );
}

function Status({ item, onRetry }: { item: TrayItem; onRetry: () => void }) {
  const t = useTranslations();
  if (item.state === "ready")
    return (
      <p className="mt-1 inline-flex items-center gap-1.5 text-sm font-medium text-credit">
        <CheckCircle aria-hidden weight="fill" className="size-4" />
        {t("Proof.ready")}
      </p>
    );
  if (item.state === "rejected")
    return (
      <p role="alert" className="mt-1 inline-flex items-start gap-1.5 text-sm font-medium text-debit">
        <WarningCircle aria-hidden weight="bold" className="mt-0.5 size-4 shrink-0" />
        <span>
          {item.reason || t("Proof.rejectedGeneric")} <span className="font-normal">{t("Proof.rejectedNote")}</span>
        </span>
      </p>
    );
  if (item.state === "failed")
    return (
      <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1">
        <p role="alert" className="inline-flex items-center gap-1.5 text-sm font-medium text-debit">
          <WarningCircle aria-hidden weight="bold" className="size-4 shrink-0" />
          {t(item.messageKey)}
        </p>
        <Button variant="secondary" size="sm" onClick={onRetry}>
          <ArrowClockwise aria-hidden weight="bold" className="size-4" />
          {t("Proof.retry")}
        </Button>
      </div>
    );
  const pct = Math.round(item.progress * 100);
  return (
    <div className="mt-1.5">
      <p className="text-sm text-muted" role="status">
        {item.step === "preparing" ? t("Proof.preparing") : item.step === "uploading" ? t("Proof.uploading", { pct }) : t("Proof.processing")}
      </p>
      <div
        role="progressbar"
        aria-label={item.name}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={item.step === "uploading" ? pct : undefined}
        className="mt-1 h-1.5 overflow-hidden rounded-full bg-sunken"
      >
        <div
          className={cn("h-full rounded-full bg-primary", item.step === "processing" || item.step === "preparing" ? "w-1/3 motion-safe:animate-pulse" : "transition-[width] duration-200")}
          style={item.step === "uploading" ? { width: `${pct}%` } : undefined}
        />
      </div>
    </div>
  );
}
