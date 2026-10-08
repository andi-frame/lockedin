"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

export default function AppError({ reset }: { error: Error; reset: () => void }) {
  const t = useTranslations("Boundary");
  return (
    <div role="alert" className="max-w-prose py-10">
      <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
      <p className="mb-6 mt-2 text-muted">{t("body")}</p>
      <Button variant="secondary" onClick={reset}>
        {t("retry")}
      </Button>
    </div>
  );
}
