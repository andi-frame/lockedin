import { renderToReactElement } from "@tiptap/static-renderer/pm/react";
import type { ProofDoc } from "@/lib/proof/doc";
import { proofExtensions } from "@/lib/proof/extensions";

/**
 * The proof, read-only, drawn from the same schema the editor writes with (so a node the editor
 * cannot make is not drawn here either). Links open in a new tab without handing over the opener.
 */
export function ProofView({ doc }: { doc: ProofDoc }) {
  return <div className="proof-prose max-w-prose text-[15px] leading-7">{renderToReactElement({ extensions: proofExtensions(), content: doc })}</div>;
}
