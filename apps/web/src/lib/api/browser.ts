"use client";

import { createApiClient } from "./client";

// Same-origin: Caddy in production, the Next rewrite in dev.
export const api = createApiClient({ baseUrl: "/api/v1" });
