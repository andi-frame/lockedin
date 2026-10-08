"use client";

import { FilePdf, X } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import type { components } from "@/lib/api/schema";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "@/components/ui/dialog";

type Attachment = components["schemas"]["Attachment"];

/**
 * What the doer attached: photos that open in a lightbox, videos with a player, PDFs as links.
 * Only `ready` attachments have signed URLs, and a proof can only be sent with ready ones.
 */
export function AttachmentGallery({ attachments }: { attachments: Attachment[] }) {
  const t = useTranslations("Detail");
  const images = attachments.filter((a) => a.kind === "image" && a.urls);
  const videos = attachments.filter((a) => a.kind === "video" && a.urls);
  const files = attachments.filter((a) => a.kind === "file" && a.urls);
  if (images.length + videos.length + files.length === 0) return null;

  return (
    <div className="flex flex-col gap-4">
      {images.length > 0 ? (
        <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          {images.map((a, i) => (
            <li key={a.id}>
              <Dialog>
                <DialogTrigger asChild>
                  <button
                    type="button"
                    aria-label={t("openImage", { n: i + 1, total: images.length })}
                    className="block aspect-[4/3] w-full overflow-hidden rounded-control border border-rule bg-sunken outline-offset-2 hover:border-rule-strong focus-visible:outline-2 focus-visible:outline-ring"
                  >
                    {/* The thumbnail is a signed storage URL, which next/image would only proxy. */}
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src={a.urls?.thumb ?? a.urls?.original} alt="" loading="lazy" className="size-full object-cover" />
                  </button>
                </DialogTrigger>
                <DialogContent className="max-w-[min(92vw,64rem)] p-3">
                  <DialogTitle className="sr-only">{t("openImage", { n: i + 1, total: images.length })}</DialogTitle>
                  <DialogDescription className="sr-only">{t("attachments", { n: images.length })}</DialogDescription>
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={a.urls?.original} alt="" className="mx-auto max-h-[78vh] w-auto max-w-full rounded-control" />
                  <DialogClose className="sr-only">
                    <X aria-hidden />
                    {t("closeImage")}
                  </DialogClose>
                </DialogContent>
              </Dialog>
            </li>
          ))}
        </ul>
      ) : null}

      {videos.map((a, i) => (
        <figure key={a.id} className="max-w-xl">
          {/* No captions exist for a study video; the player still has its native controls. */}
          <video controls preload="metadata" poster={a.urls?.poster ?? undefined} src={a.urls?.original} aria-label={t("video", { n: i + 1 })} className="w-full rounded-control bg-sunken" />
        </figure>
      ))}

      {files.length > 0 ? (
        <ul className="flex flex-col gap-2">
          {files.map((a, i) => (
            <li key={a.id}>
              <a
                href={a.urls?.original}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex min-h-11 items-center gap-2 rounded-control border border-rule-strong bg-surface px-3 text-[15px] font-medium hover:bg-sunken"
              >
                <FilePdf aria-hidden weight="bold" className="size-5 text-debit" />
                {t("openPdf", { n: i + 1 })}
              </a>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
