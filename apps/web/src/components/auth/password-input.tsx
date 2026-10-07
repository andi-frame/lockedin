"use client";

import { Eye, EyeSlash } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { useState, type ComponentProps } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/cn";

/** A password field with a show/hide switch; typing a long passphrase on a phone blind is how typos happen. */
export function PasswordInput({ className, ...props }: Omit<ComponentProps<typeof Input>, "type">) {
  const t = useTranslations("Auth");
  const [shown, setShown] = useState(false);
  const Icon = shown ? EyeSlash : Eye;
  return (
    <div className="relative">
      <Input {...props} type={shown ? "text" : "password"} className={cn("pr-12", className)} />
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={shown ? t("hidePassword") : t("showPassword")}
        aria-pressed={shown}
        onClick={() => setShown((v) => !v)}
        className="absolute right-1 top-1/2 -translate-y-1/2"
      >
        <Icon aria-hidden weight="bold" className="size-[18px]" />
      </Button>
    </div>
  );
}
