const INDONESIA = ["Asia/Jakarta", "Asia/Makassar", "Asia/Jayapura"];

const browserZones = (): string[] => Intl.supportedValuesOf("timeZone");

/**
 * The zones to offer in Settings: what the browser knows, plus the saved one (so a zone the browser
 * does not list is never silently replaced on save). Without `Intl.supportedValuesOf` it falls back
 * to Indonesia's three zones, which is where the product's users are.
 */
export function timezoneChoices(current: string, list: () => string[] = browserZones): string[] {
  let zones: string[] = [];
  try {
    zones = list();
  } catch {
    zones = [];
  }
  if (zones.length === 0) zones = INDONESIA;
  return [...new Set([...zones, current])].sort();
}
