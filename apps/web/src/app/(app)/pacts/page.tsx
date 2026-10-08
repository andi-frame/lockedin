import { Plus } from "@phosphor-icons/react/dist/ssr";
import type { Metadata } from "next";
import { getFormatter, getTranslations } from "next-intl/server";
import Link from "next/link";
import { PactStatusBadge } from "@/components/pact/status-badge";
import { Button } from "@/components/ui/button";
import { serverApi } from "@/lib/api/server";
import { unwrap } from "@/lib/api/unwrap";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Pacts");
  return { title: t("title") };
}

export default async function PactsPage() {
  const [t, format, api] = await Promise.all([getTranslations("Pacts"), getFormatter(), serverApi()]);
  const page = await unwrap(api.GET("/pacts", { params: { query: { limit: 50 } } }));
  const date = (iso: string) => format.dateTime(new Date(`${iso}T00:00:00Z`), { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-3xl font-semibold tracking-tight">{t("title")}</h1>
        <Button asChild>
          <Link href="/pacts/new">
            <Plus aria-hidden weight="bold" className="size-4" />
            {t("new")}
          </Link>
        </Button>
      </div>

      {page.items.length === 0 ? (
        <p className="mt-8 max-w-prose text-muted">{t("empty")}</p>
      ) : (
        <ul className="mt-8 divide-y divide-rule border-y border-rule">
          {page.items.map((p) => (
            <li key={p.id}>
              <Link
                href={`/pacts/${p.id}`}
                className="flex min-h-16 flex-wrap items-center justify-between gap-x-4 gap-y-1 py-3 outline-offset-4 hover:bg-sunken focus-visible:outline-2 focus-visible:outline-ring sm:px-2"
              >
                <span className="min-w-0">
                  <span className="block truncate font-medium">{p.title}</span>
                  <span className="block font-mono text-sm tabular-nums text-muted">
                    {date(p.starts_on)} – {date(p.ends_on)}
                  </span>
                </span>
                <PactStatusBadge status={p.status} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
