import { redirect } from "next/navigation";
import { HOME } from "@/lib/auth/guard";

// src/proxy.ts normally answers `/` first; this is the fallback if the proxy is ever bypassed.
export default function Home() {
  redirect(HOME);
}
