import CharacterCount from "@tiptap/extension-character-count";
import TaskItem from "@tiptap/extension-task-item";
import TaskList from "@tiptap/extension-task-list";
import StarterKit from "@tiptap/starter-kit";

/** Enough for any honest study log; the server refuses a document over 200 KB of JSON. */
export const MAX_CHARACTERS = 20_000;

/**
 * Exactly the nodes and marks the server accepts (SPEC §8, `src/lib/proof/doc.ts`): StarterKit's
 * underline and trailing node are switched off because they are not on the list, headings are
 * levels 2 and 3, and links are http or https. A test builds the schema and checks it against
 * the allow-list, so adding an extension here that the server would refuse fails the build.
 * Inline images are not offered: attachments live in the tray.
 */
export function proofExtensions() {
  return [
    StarterKit.configure({
      heading: { levels: [2, 3] },
      underline: false,
      trailingNode: false,
      link: { openOnClick: false, autolink: true, defaultProtocol: "https", protocols: ["http", "https"] },
    }),
    TaskList,
    TaskItem.configure({ nested: false }),
    CharacterCount.configure({ limit: MAX_CHARACTERS }),
  ];
}
