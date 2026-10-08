/**
 * The headers a server-side API call takes from the visitor's own request. Only the client address
 * (`x-forwarded-for`, set by Caddy): the API rate-limits per client IP and trusts that header from
 * private addresses, so a call without it counts against the web container, and every visitor
 * would share one budget. Cookies are added separately; nothing else is copied.
 */
export function forwardedHeaders(incoming: Headers): Record<string, string> {
  const forwarded = incoming.get("x-forwarded-for");
  return forwarded ? { "x-forwarded-for": forwarded } : {};
}
