import { Notebook } from "@phosphor-icons/react/dist/ssr";
import { cn } from "@/lib/cn";

// A text wordmark until the logo is designed (surface brief, "Unresolved").
export function Wordmark({ className }: { className?: string }) {
  return (
    <span translate="no" className={cn("inline-flex items-center gap-2 text-xl font-semibold tracking-tight", className)}>
      <Notebook aria-hidden weight="fill" className="size-6" />
      Tepati
    </span>
  );
}
