"use client";

import { useQueryClient } from "@tanstack/react-query";
import { SignOut } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button, type ButtonProps } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import { unwrap } from "@/lib/api/unwrap";

export function LogoutButton({ variant = "secondary", size, block, className }: Pick<ButtonProps, "variant" | "size" | "block" | "className">) {
  const t = useTranslations();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [pending, setPending] = useState(false);

  async function logout() {
    if (pending) return;
    setPending(true);
    try {
      await unwrap(api.POST("/auth/logout"));
    } catch (err) {
      // 401 means the session was already gone, which is the state we wanted.
      if (!(err instanceof ApiError && err.status === 401)) {
        setPending(false);
        toast({ title: t(`Errors.${errorMessageKey(err instanceof ApiError ? err.code : "unknown")}`), tone: "error" });
        return;
      }
    }
    // Drop whatever the previous account had cached before anyone else signs in on this device.
    queryClient.clear();
    router.replace("/login");
    router.refresh();
  }

  return (
    <Button variant={variant} size={size} block={block} className={className} loading={pending} onClick={logout}>
      <SignOut aria-hidden weight="bold" className="size-[18px]" />
      {pending ? t("Nav.loggingOut") : t("Nav.logout")}
    </Button>
  );
}
