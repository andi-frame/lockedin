import "server-only";
import { cache } from "react";
import { ApiError } from "../api/errors";
import { serverApi } from "../api/server";
import type { components } from "../api/schema";
import { unwrap } from "../api/unwrap";

export type User = components["schemas"]["User"];

/** The signed-in user, or null when there is no live session. Other failures throw. */
export const getCurrentUser = cache(async (): Promise<User | null> => {
  const api = await serverApi();
  try {
    return await unwrap(api.GET("/me"));
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return null;
    throw err;
  }
});

/** Unread notification count for the bell. A failure here must never take the page down. */
export const getUnreadCount = cache(async (): Promise<number> => {
  try {
    const api = await serverApi();
    const page = await unwrap(api.GET("/notifications", { params: { query: { limit: 1 } } }));
    return page.unread_count;
  } catch {
    return 0;
  }
});
