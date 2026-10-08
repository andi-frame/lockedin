"use client";

import { Moon } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import { unwrap } from "@/lib/api/unwrap";

/** Declares today a rest day. It is confirmed in a dialog because it spends one of a few and cannot be undone. */
export function RestButton({ checkInId, left }: { checkInId: string; left: number }) {
  const t = useTranslations("Today");
  const tErr = useTranslations("Errors");
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();

  async function declare() {
    if (pending) return;
    setPending(true);
    setError(undefined);
    try {
      await unwrap(api.POST("/check-ins/{checkInId}/rest", { params: { path: { checkInId } } }));
    } catch (err) {
      setError(tErr(errorMessageKey(err instanceof ApiError ? err.code : "unknown")));
      setPending(false);
      return;
    }
    setOpen(false);
    setPending(false);
    toast({ title: t("restDone"), tone: "success" });
    router.refresh();
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (pending ? undefined : setOpen(o))}>
      <DialogTrigger asChild>
        <Button variant="secondary">
          <Moon aria-hidden weight="bold" className="size-4" />
          {t("restOpen")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("restTitle")}</DialogTitle>
          <DialogDescription>{t("restBody", { left, after: left - 1 })}</DialogDescription>
        </DialogHeader>
        {error ? <FormError>{error}</FormError> : null}
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="secondary" disabled={pending}>
              {t("restCancel")}
            </Button>
          </DialogClose>
          <Button onClick={declare} loading={pending}>
            {pending ? t("restWorking") : t("restConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
