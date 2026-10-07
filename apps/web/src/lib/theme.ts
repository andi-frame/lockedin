export const THEME_COOKIE = "tepati_theme";
export const themes = ["system", "light", "dark"] as const;
export type Theme = (typeof themes)[number];

export function parseTheme(v: string | undefined): Theme {
  return themes.find((t) => t === v) ?? "system";
}
