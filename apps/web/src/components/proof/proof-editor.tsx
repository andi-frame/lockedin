"use client";

import {
  ArrowClockwise,
  ArrowCounterClockwise,
  Code,
  CodeBlock,
  LinkSimple,
  ListBullets,
  ListChecks,
  ListNumbers,
  Minus,
  Quotes,
  TextB,
  TextHThree,
  TextHTwo,
  TextItalic,
  TextStrikethrough,
} from "@phosphor-icons/react";
import { EditorContent, useEditor, useEditorState, type Editor } from "@tiptap/react";
import { useTranslations } from "next-intl";
import { useState, type ComponentType, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/cn";
import { normalizeLink, type ProofDoc } from "@/lib/proof/doc";
import { MAX_CHARACTERS, proofExtensions } from "@/lib/proof/extensions";

type Tool = {
  key: string;
  Icon: ComponentType<{ "aria-hidden"?: boolean; weight?: "bold" | "regular"; className?: string }>;
  run: (e: Editor) => void;
  active?: (e: Editor) => boolean;
  disabled?: (e: Editor) => boolean;
};

const tools: Tool[][] = [
  [
    { key: "bold", Icon: TextB, run: (e) => e.chain().focus().toggleBold().run(), active: (e) => e.isActive("bold") },
    { key: "italic", Icon: TextItalic, run: (e) => e.chain().focus().toggleItalic().run(), active: (e) => e.isActive("italic") },
    { key: "strike", Icon: TextStrikethrough, run: (e) => e.chain().focus().toggleStrike().run(), active: (e) => e.isActive("strike") },
    { key: "code", Icon: Code, run: (e) => e.chain().focus().toggleCode().run(), active: (e) => e.isActive("code") },
  ],
  [
    { key: "h2", Icon: TextHTwo, run: (e) => e.chain().focus().toggleHeading({ level: 2 }).run(), active: (e) => e.isActive("heading", { level: 2 }) },
    { key: "h3", Icon: TextHThree, run: (e) => e.chain().focus().toggleHeading({ level: 3 }).run(), active: (e) => e.isActive("heading", { level: 3 }) },
    { key: "bullets", Icon: ListBullets, run: (e) => e.chain().focus().toggleBulletList().run(), active: (e) => e.isActive("bulletList") },
    { key: "numbers", Icon: ListNumbers, run: (e) => e.chain().focus().toggleOrderedList().run(), active: (e) => e.isActive("orderedList") },
    { key: "tasks", Icon: ListChecks, run: (e) => e.chain().focus().toggleTaskList().run(), active: (e) => e.isActive("taskList") },
    { key: "quote", Icon: Quotes, run: (e) => e.chain().focus().toggleBlockquote().run(), active: (e) => e.isActive("blockquote") },
    { key: "codeBlock", Icon: CodeBlock, run: (e) => e.chain().focus().toggleCodeBlock().run(), active: (e) => e.isActive("codeBlock") },
    { key: "rule", Icon: Minus, run: (e) => e.chain().focus().setHorizontalRule().run() },
  ],
  [
    { key: "undo", Icon: ArrowCounterClockwise, run: (e) => e.chain().focus().undo().run(), disabled: (e) => !e.can().undo() },
    { key: "redo", Icon: ArrowClockwise, run: (e) => e.chain().focus().redo().run(), disabled: (e) => !e.can().redo() },
  ],
];

/**
 * The proof body: a Tiptap 3 editor limited to the nodes and marks the server accepts. The toolbar
 * wraps onto a second row on a phone instead of hiding controls. Pasted or dropped files are not
 * inserted into the text; they go to the attachment tray through `onFiles`.
 */
export function ProofEditor({
  labelledBy,
  describedBy,
  content,
  onChange,
  onFiles,
}: {
  labelledBy: string;
  describedBy?: string;
  content: ProofDoc | null;
  onChange: (doc: ProofDoc, characters: number) => void;
  onFiles: (files: File[]) => void;
}) {
  const t = useTranslations("Proof");
  const [linkOpen, setLinkOpen] = useState(false);
  const [link, setLink] = useState("");
  const [linkError, setLinkError] = useState(false);

  const editor = useEditor({
    extensions: proofExtensions(),
    content: content ?? undefined,
    // Server render and the first client render agree on an empty shell; the editor mounts after.
    immediatelyRender: false,
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-multiline": "true",
        "aria-labelledby": labelledBy,
        ...(describedBy ? { "aria-describedby": describedBy } : {}),
        class: "proof-prose min-h-48 px-3 py-3 text-[15px] leading-7 outline-none",
      },
      handlePaste: (_view, event) => {
        const files = Array.from(event.clipboardData?.files ?? []);
        if (files.length === 0) return false;
        onFiles(files);
        return true;
      },
      handleDrop: (_view, event) => {
        const files = Array.from(event.dataTransfer?.files ?? []);
        if (files.length === 0) return false;
        event.preventDefault();
        onFiles(files);
        return true;
      },
    },
    onUpdate: ({ editor }) => onChange(editor.getJSON() as ProofDoc, editor.storage.characterCount.characters()),
  });

  // Which buttons are pressed or unavailable follows the selection, so subscribe to that.
  const state = useEditorState({
    editor,
    selector: ({ editor }) =>
      editor ? { active: tools.flat().map((x) => x.active?.(editor) ?? false), disabled: tools.flat().map((x) => x.disabled?.(editor) ?? false), link: editor.isActive("link") } : null,
  });

  function openLink() {
    if (!editor) return;
    setLink((editor.getAttributes("link").href as string | undefined) ?? "");
    setLinkError(false);
    setLinkOpen(true);
  }

  function applyLink(event: FormEvent) {
    event.preventDefault();
    if (!editor) return;
    const href = normalizeLink(link);
    if (!href) return setLinkError(true);
    editor.chain().focus().extendMarkRange("link").setLink({ href }).run();
    setLinkOpen(false);
  }

  function removeLink() {
    editor?.chain().focus().extendMarkRange("link").unsetLink().run();
    setLinkOpen(false);
  }

  let flat = 0;
  return (
    <div className="rounded-control border border-rule-strong bg-surface focus-within:border-ring focus-within:outline-2 focus-within:outline-ring">
      <div role="toolbar" aria-label={t("toolbar")} className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-rule p-1.5">
        {tools.map((group, gi) => (
          <div key={gi} className="flex items-center gap-0.5">
            {group.map((tool) => {
              const i = flat++;
              const pressed = tool.active ? (state?.active[i] ?? false) : undefined;
              return (
                <Button
                  key={tool.key}
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t(`tool_${tool.key}`)}
                  aria-pressed={pressed}
                  disabled={!editor || (state?.disabled[i] ?? false)}
                  onClick={() => editor && tool.run(editor)}
                  className={cn(pressed && "bg-sunken text-teal-text")}
                >
                  <tool.Icon aria-hidden weight={pressed ? "bold" : "regular"} className="size-[18px]" />
                </Button>
              );
            })}
            {gi === 0 ? (
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("tool_link")}
                aria-pressed={state?.link ?? false}
                aria-expanded={linkOpen}
                disabled={!editor}
                onClick={openLink}
                className={cn(state?.link && "bg-sunken text-teal-text")}
              >
                <LinkSimple aria-hidden weight={state?.link ? "bold" : "regular"} className="size-[18px]" />
              </Button>
            ) : null}
          </div>
        ))}
      </div>

      {linkOpen ? (
        <form onSubmit={applyLink} noValidate className="flex flex-col gap-2 border-b border-rule bg-sunken p-2 sm:flex-row sm:items-start">
          <div className="min-w-0 flex-1">
            <Input
              type="url"
              inputMode="url"
              autoFocus
              autoCapitalize="none"
              spellCheck={false}
              aria-label={t("linkLabel")}
              autoComplete="off"
              placeholder="https://"
              value={link}
              aria-invalid={linkError ? true : undefined}
              onChange={(e) => (setLink(e.target.value), setLinkError(false))}
            />
            {linkError ? (
              <p role="alert" className="mt-1 text-sm font-medium text-debit">
                {t("linkInvalid")}
              </p>
            ) : null}
          </div>
          <div className="flex gap-2">
            <Button type="submit" size="sm">
              {t("linkApply")}
            </Button>
            {state?.link ? (
              <Button size="sm" variant="secondary" onClick={removeLink}>
                {t("linkRemove")}
              </Button>
            ) : null}
            <Button size="sm" variant="ghost" onClick={() => setLinkOpen(false)}>
              {t("linkCancel")}
            </Button>
          </div>
        </form>
      ) : null}

      <EditorContent editor={editor} />
      <p className="sr-only">{t("limitNote", { max: MAX_CHARACTERS })}</p>
    </div>
  );
}
