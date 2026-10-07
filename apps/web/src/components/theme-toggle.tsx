"use client";

import { Desktop, Moon, Sun } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { useState, type ComponentType } from "react";
import { cn } from "@/lib/cn";
import { THEME_COOKIE, themes, type Theme } from "@/lib/theme";

const icons: Record<Theme, ComponentType<{ "aria-hidden"?: boolean; weight?: "bold"; className?: string }>> = {
  system: Desktop,
  light: Sun,
  dark: Moon,
};

function apply(theme: Theme) {
  const root = document.documentElement;
  if (theme === "system") delete root.dataset.theme;
  else root.dataset.theme = theme;
  // A year; read on the server so the first paint already has the right theme.
  document.cookie = `${THEME_COOKIE}=${theme}; path=/; max-age=31536000; samesite=lax`;
}

/** Match system (default), light, or dark. Three buttons rather than a menu: it is one tap. */
export function ThemeToggle({ initial = "system", className }: { initial?: Theme; className?: string }) {
  const t = useTranslations("Theme");
  const [theme, setTheme] = useState<Theme>(initial);
  return (
    <div role="group" aria-label={t("label")} className={cn("inline-flex rounded-control border border-rule-strong bg-surface p-0.5", className)}>
      {themes.map((value) => {
        const Icon = icons[value];
        const active = theme === value;
        return (
          <button
            key={value}
            type="button"
            aria-pressed={active}
            aria-label={t(value)}
            title={t(value)}
            onClick={() => {
              setTheme(value);
              apply(value);
            }}
            className={cn(
              "inline-flex size-10 items-center justify-center rounded-control transition-colors duration-150",
              active ? "bg-primary text-primary-foreground" : "text-muted hover:bg-sunken hover:text-ink",
            )}
          >
            <Icon aria-hidden weight="bold" className="size-[18px]" />
          </button>
        );
      })}
    </div>
  );
}
