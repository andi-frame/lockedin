package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// ErrInvalidProofDoc rejects rich-text JSON outside the allow-list (SPEC §8).
var ErrInvalidProofDoc = &Error{"proof.invalid_doc", "the proof contains unsupported content"}

// ProofNode mirrors the ProseMirror/Tiptap JSON shape.
type ProofNode struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []ProofNode    `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []ProofMark    `json:"marks,omitempty"`
}

type ProofMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

var allowedNodes = map[string]bool{
	"doc": true, "paragraph": true, "heading": true, "bulletList": true, "orderedList": true,
	"listItem": true, "blockquote": true, "codeBlock": true, "hardBreak": true, "horizontalRule": true,
	"text": true, "taskList": true, "taskItem": true, "attachmentImage": true,
}

var allowedMarks = map[string]bool{"bold": true, "italic": true, "strike": true, "code": true, "link": true}

const (
	maxProofDocBytes = 200 << 10 // 200 KB of JSON is far beyond any honest study log
	maxProofDepth    = 12
)

// ProofText is what the server derives from a validated document.
type ProofText struct {
	Text          string
	WordCount     int
	AttachmentIDs []uuid.UUID // attachments embedded inline, in document order
}

// ParseProofDoc validates a Tiptap document against the allow-list and derives its
// plain text and word count. Client-supplied text is never trusted.
func ParseProofDoc(raw []byte) (ProofText, error) {
	if len(raw) == 0 || len(raw) > maxProofDocBytes {
		return ProofText{}, fmt.Errorf("%w: document size", ErrInvalidProofDoc)
	}
	var root ProofNode
	if err := json.Unmarshal(raw, &root); err != nil {
		return ProofText{}, fmt.Errorf("%w: %v", ErrInvalidProofDoc, err)
	}
	if root.Type != "doc" {
		return ProofText{}, fmt.Errorf("%w: root must be doc", ErrInvalidProofDoc)
	}
	var b strings.Builder
	var ids []uuid.UUID
	if err := walkProof(root, 0, &b, &ids); err != nil {
		return ProofText{}, err
	}
	text := strings.TrimSpace(b.String())
	return ProofText{Text: text, WordCount: len(strings.Fields(text)), AttachmentIDs: ids}, nil
}

func walkProof(n ProofNode, depth int, b *strings.Builder, ids *[]uuid.UUID) error {
	if depth > maxProofDepth {
		return fmt.Errorf("%w: nested too deeply", ErrInvalidProofDoc)
	}
	if !allowedNodes[n.Type] {
		return fmt.Errorf("%w: node %q", ErrInvalidProofDoc, n.Type)
	}
	switch n.Type {
	case "heading":
		if lvl, _ := n.Attrs["level"].(float64); lvl != 2 && lvl != 3 {
			return fmt.Errorf("%w: heading level must be 2 or 3", ErrInvalidProofDoc)
		}
	case "attachmentImage":
		// Inline images reference an uploaded attachment, never an external src.
		id, err := uuid.Parse(fmt.Sprint(n.Attrs["attachmentId"]))
		if err != nil || len(n.Attrs) != 1 {
			return fmt.Errorf("%w: attachmentImage needs only a valid attachmentId", ErrInvalidProofDoc)
		}
		*ids = append(*ids, id)
	case "text":
		b.WriteString(n.Text)
	}
	for _, m := range n.Marks {
		if !allowedMarks[m.Type] {
			return fmt.Errorf("%w: mark %q", ErrInvalidProofDoc, m.Type)
		}
		if m.Type == "link" && !safeLink(fmt.Sprint(m.Attrs["href"])) {
			return fmt.Errorf("%w: links must be http or https", ErrInvalidProofDoc)
		}
	}
	for _, c := range n.Content {
		if err := walkProof(c, depth+1, b, ids); err != nil {
			return err
		}
	}
	switch n.Type { // block boundaries separate words
	case "paragraph", "heading", "listItem", "taskItem", "codeBlock", "blockquote", "hardBreak":
		b.WriteByte('\n')
	}
	return nil
}

func safeLink(href string) bool {
	u, err := url.Parse(href)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// ValidateLinks checks the separate links list on a proof.
func ValidateLinks(links []string) error {
	if len(links) > 20 {
		return fmt.Errorf("%w: at most 20 links", ErrInvalidProofDoc)
	}
	for _, l := range links {
		if len(l) > 2048 || !safeLink(l) {
			return fmt.Errorf("%w: link %q", ErrInvalidProofDoc, l)
		}
	}
	return nil
}
