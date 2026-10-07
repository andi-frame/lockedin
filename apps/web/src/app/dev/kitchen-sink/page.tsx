import { ArrowRight, CameraPlus, Check, Plus } from "@phosphor-icons/react/dist/ssr";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import { Amount } from "@/components/amount";
import { Countdown } from "@/components/countdown";
import { MemberLine } from "@/components/member-line";
import { checkInStatuses, StatusChip } from "@/components/status-chip";
import { ThemeToggle } from "@/components/theme-toggle";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { parseTheme, THEME_COOKIE } from "@/lib/theme";
import { FormDemo, OverlayDemo, TabsDemo } from "./demos";

export const metadata: Metadata = { title: "Kitchen sink", robots: { index: false } };

// The demo uses a fixed server clock so the countdown is the same on every visit (and in
// screenshots). All names and numbers below are synthetic demo data.
const SERVER_NOW = "2026-10-08T18:18:48Z";
const CUTOFF = "2026-10-08T22:00:00Z";

const swatches: { name: string; cls: string; note?: string }[] = [
  { name: "ground", cls: "bg-ground" },
  { name: "surface", cls: "bg-surface" },
  { name: "sunken", cls: "bg-sunken" },
  { name: "rule", cls: "bg-rule" },
  { name: "ink", cls: "bg-ink" },
  { name: "muted", cls: "bg-muted" },
  { name: "cover", cls: "bg-cover", note: "shell, rail, header band" },
  { name: "primary", cls: "bg-primary" },
  { name: "debit", cls: "bg-debit" },
  { name: "credit", cls: "bg-credit" },
  { name: "stamp", cls: "bg-stamp", note: "human decisions only" },
  { name: "today", cls: "bg-today", note: "today only" },
  { name: "member-a", cls: "bg-member-a" },
  { name: "member-b", cls: "bg-member-b" },
];

const ledger = [
  { date: "05 Okt", note: "Pot dibuka oleh Dimas", coins: 1000, dir: "credit" as const, saldo: 1000 },
  { date: "06 Okt", note: "Terlewat, tidak ada bukti", coins: 40, dir: "debit" as const, saldo: 960 },
  { date: "07 Okt", note: "Disetujui, tanpa potongan", coins: 0, dir: "balance" as const, saldo: 960 },
  { date: "08 Okt", note: "Penyokong terlewat, tambah pot", coins: 25, dir: "credit" as const, saldo: 985 },
];

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="border-t border-rule py-8 first:border-t-0">
      <h2 className="mb-5 text-lg font-semibold tracking-tight">{title}</h2>
      {children}
    </section>
  );
}

export default async function KitchenSink() {
  // Dev builds only (PLAN 5.2): a production build answers 404.
  if (process.env.NODE_ENV === "production") notFound();
  const theme = parseTheme((await cookies()).get(THEME_COOKIE)?.value);

  return (
    <main className="mx-auto max-w-5xl pb-24">
      <header className="guilloche bg-cover px-4 pb-6 pt-5 text-cover-ink sm:rounded-b-panel sm:px-8">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-sm text-cover-muted">Kontrak contoh (data sintetis)</p>
            <h1 className="mt-1 text-2xl font-semibold tracking-tight">Sprint UTBK Agustus</h1>
          </div>
          <ThemeToggle initial={theme} />
        </div>
        <div className="mt-6">
          <Countdown until={CUTOFF} serverNow={SERVER_NOW} onCover size="lg" />
        </div>
      </header>

      <div className="px-4 sm:px-8">
        <Section title="Warna">
          <ul className="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-4 lg:grid-cols-7">
            {swatches.map((s) => (
              <li key={s.name}>
                <div className={`${s.cls} h-12 rounded-control border border-rule-strong`} />
                <p className="mt-1.5 font-mono text-[13px]">{s.name}</p>
                {s.note ? <p className="text-[13px] leading-tight text-muted">{s.note}</p> : null}
              </li>
            ))}
          </ul>
        </Section>

        <Section title="Tipografi">
          <div className="grid gap-6 md:grid-cols-2">
            <div className="space-y-2">
              <p className="text-3xl font-semibold tracking-tight">Geist untuk antarmuka</p>
              <p className="text-lg font-medium">Hari ini, Kontrak, Tinjau, Pengaturan</p>
              <p className="max-w-[60ch]">
                Bukti belajar dikirim sebelum batas waktu. Kalau terlewat, pot berkurang sesuai kesepakatan, dan setiap
                koin yang bergerak punya baris di buku koin.
              </p>
              <p className="text-sm text-muted">Teks sekunder dipakai untuk keterangan dan bantuan isian.</p>
            </div>
            <div className="space-y-2 font-mono">
              <p className="text-3xl font-medium">1.234.567,00</p>
              <p className="text-lg">03:41:12 &nbsp; 08 Okt 2026</p>
              <p>0123456789 sama lebar</p>
              <p className="text-sm text-muted">Geist Mono, angka tabular untuk jumlah, tanggal, dan hitung mundur.</p>
            </div>
          </div>
        </Section>

        <Section title="Tombol">
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-3">
              <Button>
                <CameraPlus aria-hidden weight="bold" className="size-[18px]" />
                Kirim bukti
              </Button>
              <Button variant="secondary">Simpan draf</Button>
              <Button variant="ghost">Lewati</Button>
              <Button variant="decision">Setujui</Button>
              <Button variant="decision-quiet">Tolak</Button>
              <Button size="icon" aria-label="Tambah">
                <Plus aria-hidden weight="bold" className="size-5" />
              </Button>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <Button size="sm">Kecil</Button>
              <Button size="sm" variant="secondary">
                Kecil
              </Button>
              <Button disabled>Nonaktif</Button>
              <Button variant="secondary" disabled>
                Nonaktif
              </Button>
              <Button loading>Mengirim</Button>
              <Button variant="decision" loading>
                Menyimpan
              </Button>
            </div>
            <Button block>
              Lanjut ke tinjau ketentuan
              <ArrowRight aria-hidden weight="bold" className="size-[18px]" />
            </Button>
          </div>
        </Section>

        <Section title="Formulir">
          <FormDemo />
        </Section>

        <Section title="Tab">
          <TabsDemo />
        </Section>

        <Section title="Status check-in">
          <ul className="flex flex-wrap gap-2">
            {checkInStatuses.map((s) => (
              <li key={s}>
                <StatusChip status={s} />
              </li>
            ))}
          </ul>
          <p className="mt-4 text-sm text-muted">
            Ungu stempel hanya untuk hasil keputusan manusia (disetujui, ditolak). Setiap status punya ikon dan label sendiri.
          </p>
        </Section>

        <Section title="Lencana">
          <div className="flex flex-wrap items-center gap-2">
            <Badge>Netral</Badge>
            <Badge tone="quiet">Pelan</Badge>
            <Badge tone="cover">
              <Check aria-hidden weight="bold" />
              Penyokong
            </Badge>
            <Badge tone="debit">Debit</Badge>
            <Badge tone="credit">Kredit</Badge>
            <Badge tone="stamp">Stempel</Badge>
            <Badge tone="today">Hari ini</Badge>
          </div>
        </Section>

        <Section title="Jumlah koin">
          <div className="grid gap-8 md:grid-cols-[1fr_1.1fr]">
            <div className="space-y-5">
              <Amount coins={985} direction="balance" size="xl" rate={1000} />
              <div className="flex flex-wrap items-end gap-x-8 gap-y-4">
                <Amount coins={40} direction="debit" size="lg" />
                <Amount coins={25} direction="credit" size="lg" />
                <Amount coins={1500} direction="debit" rate={1000} />
                <Amount coins={120} direction="credit" size="sm" />
              </div>
            </div>
            <ul className="divide-y divide-rule border-y border-rule">
              {ledger.map((row) => (
                <li key={row.date} className="grid grid-cols-[3.5rem_1fr_auto] items-baseline gap-x-3 gap-y-1 py-2.5 sm:grid-cols-[3.5rem_1fr_auto_auto]">
                  <span className="font-mono text-sm text-muted">{row.date}</span>
                  <span className="order-last col-span-3 min-w-0 text-sm text-muted sm:order-none sm:col-span-1 sm:truncate sm:text-ink">{row.note}</span>
                  {row.dir === "balance" ? (
                    <span className="justify-self-end font-mono text-sm text-muted sm:w-24 sm:text-right">-</span>
                  ) : (
                    <Amount coins={row.coins} direction={row.dir} size="sm" className="justify-self-end sm:w-24 sm:items-end" />
                  )}
                  <Amount coins={row.saldo} direction="balance" size="sm" className="justify-self-end sm:w-20 sm:items-end" />
                </li>
              ))}
            </ul>
          </div>
        </Section>

        <Section title="Anggota dan hari ini">
          <div className="flex flex-wrap items-center gap-x-10 gap-y-4">
            <MemberLine name="Dimas Pratama" slot={0} role="backer" />
            <MemberLine name="Sari Wulandari" slot={1} role="doer" />
            <ul className="flex gap-1.5 font-mono text-sm" aria-label="Pekan ini">
              {[6, 7, 8, 9, 10].map((d) => (
                <li
                  key={d}
                  aria-current={d === 8 ? "date" : undefined}
                  className={
                    d === 8
                      ? "flex size-10 items-center justify-center rounded-control bg-today font-semibold text-today-ink"
                      : "flex size-10 items-center justify-center rounded-control border border-rule bg-surface text-muted"
                  }
                >
                  {d}
                </li>
              ))}
            </ul>
          </div>
        </Section>

        <Section title="Hitung mundur">
          <div className="space-y-4">
            <div>
              <Countdown until={CUTOFF} serverNow={SERVER_NOW} />
            </div>
            <div>
              <Countdown until="2026-10-08T18:20:00Z" serverNow={SERVER_NOW} />
            </div>
            <div>
              <Countdown until="2026-10-08T18:00:00Z" serverNow={SERVER_NOW} />
            </div>
          </div>
        </Section>

        <Section title="Dialog, sheet, tooltip, toast">
          <OverlayDemo />
        </Section>
      </div>
    </main>
  );
}
