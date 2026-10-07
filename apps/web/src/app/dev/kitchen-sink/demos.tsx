"use client";

// Dev-only demos for the interactive primitives. The copy here is demo data (synthetic) and is not
// user-facing, so it is not in messages/*.json.
import { Info, PaperPlaneTilt, SealCheck } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "@/components/ui/toast";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export function FormDemo() {
  return (
    <div className="grid gap-5 md:grid-cols-2">
      <Field label="Nama kontrak" hint="Dilihat oleh kamu dan teman belajarmu.">
        <Input defaultValue="Sprint UTBK Agustus" />
      </Field>
      <Field label="Email" error="Format email belum benar. Contoh: nama@kampus.ac.id">
        <Input defaultValue="sari@kampus" />
      </Field>
      <Field label="Dikunci sampai kontrak aktif">
        <Input disabled defaultValue="Asia/Jakarta" />
      </Field>
      <Field label="Zona waktu">
        <Select defaultValue="jkt">
          <SelectTrigger>
            <SelectValue placeholder="Pilih zona waktu" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="jkt">Asia/Jakarta (WIB)</SelectItem>
            <SelectItem value="mks">Asia/Makassar (WITA)</SelectItem>
            <SelectItem value="jyp">Asia/Jayapura (WIT)</SelectItem>
          </SelectContent>
        </Select>
      </Field>
      <Field label="Catatan belajar" hint="Minimal 30 kata untuk hari biasa." className="md:col-span-2">
        <Textarea defaultValue="Latihan 40 soal Penalaran Matematika, salah 6, semuanya di bab peluang. Besok ulang bab itu dulu." />
      </Field>
    </div>
  );
}

export function TabsDemo() {
  return (
    <Tabs defaultValue="today">
      <TabsList>
        <TabsTrigger value="today">Hari ini</TabsTrigger>
        <TabsTrigger value="pact">Kontrak</TabsTrigger>
        <TabsTrigger value="review">Tinjau</TabsTrigger>
      </TabsList>
      <TabsContent value="today">Panel Hari ini: aksi hari ini dan statusnya.</TabsContent>
      <TabsContent value="pact">Panel Kontrak: syarat yang sudah disepakati berdua.</TabsContent>
      <TabsContent value="review">Panel Tinjau: bukti yang menunggu keputusanmu.</TabsContent>
    </Tabs>
  );
}

export function OverlayDemo() {
  return (
    <div className="flex flex-wrap gap-3">
      <Dialog>
        <DialogTrigger asChild>
          <Button variant="decision">
            <SealCheck aria-hidden weight="bold" className="size-[18px]" />
            Setujui bukti
          </Button>
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Setujui bukti hari ini?</DialogTitle>
            <DialogDescription>
              Keputusanmu tercatat di buku koin dan terlihat oleh Sari. Tidak ada koin yang dipotong.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="secondary">Batal</Button>
            </DialogClose>
            <DialogClose asChild>
              <Button variant="decision">Setujui</Button>
            </DialogClose>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet>
        <SheetTrigger asChild>
          <Button variant="secondary">
            <PaperPlaneTilt aria-hidden weight="bold" className="size-[18px]" />
            Sheet bawah
          </Button>
        </SheetTrigger>
        <SheetContent side="bottom">
          <SheetTitle>Kirim bukti</SheetTitle>
          <SheetDescription>Lampirkan foto catatan atau tangkapan layar latihan soal.</SheetDescription>
          <Button block>Pilih foto</Button>
        </SheetContent>
      </Sheet>

      <Sheet>
        <SheetTrigger asChild>
          <Button variant="secondary">Sheet kanan</Button>
        </SheetTrigger>
        <SheetContent side="right">
          <SheetTitle>Riwayat keputusan</SheetTitle>
          <SheetDescription>Setiap keputusan penyokong tercatat dengan alasannya.</SheetDescription>
        </SheetContent>
      </Sheet>

      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" aria-label="Tentang batas waktu">
            <Info aria-hidden weight="bold" className="size-5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>Batas waktu dihitung oleh server, bukan jam di ponselmu.</TooltipContent>
      </Tooltip>

      <Button variant="secondary" onClick={() => toast({ title: "Bukti terkirim", description: "Menunggu tinjauan Sari." })}>
        Toast netral
      </Button>
      <Button variant="secondary" onClick={() => toast({ title: "Kontrak disimpan", tone: "success" })}>
        Toast sukses
      </Button>
      <Button
        variant="secondary"
        onClick={() => toast({ title: "Gagal mengirim", description: "Periksa koneksimu lalu coba lagi.", tone: "error" })}
      >
        Toast galat
      </Button>
    </div>
  );
}
