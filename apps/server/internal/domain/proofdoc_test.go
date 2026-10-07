package domain

import (
	"errors"
	"strings"
	"testing"
)

const goodDoc = `{"type":"doc","content":[
  {"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Sesi 1"}]},
  {"type":"paragraph","content":[{"type":"text","text":"Mengerjakan 50 soal "},
     {"type":"text","text":"TPS","marks":[{"type":"bold"},{"type":"link","attrs":{"href":"https://example.com/tps"}}]}]},
  {"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"benar 41"}]}]}]},
  {"type":"attachmentImage","attrs":{"attachmentId":"00000000-0000-7000-8000-0000000000d1"}}
]}`

func TestParseProofDocDerivesTextAndWords(t *testing.T) {
	p, err := ParseProofDoc([]byte(goodDoc))
	if err != nil {
		t.Fatal(err)
	}
	if p.WordCount != 8 { // Sesi 1 / Mengerjakan 50 soal TPS / benar 41
		t.Fatalf("word count: %d (%q)", p.WordCount, p.Text)
	}
	if !strings.Contains(p.Text, "Sesi 1\nMengerjakan") {
		t.Fatalf("block boundaries must separate words: %q", p.Text)
	}
	if len(p.AttachmentIDs) != 1 || p.AttachmentIDs[0].String() != "00000000-0000-7000-8000-0000000000d1" {
		t.Fatalf("inline attachments: %v", p.AttachmentIDs)
	}
}

func TestParseProofDocRejects(t *testing.T) {
	cases := map[string]string{
		"not json":         `{`,
		"empty":            ``,
		"root not doc":     `{"type":"paragraph"}`,
		"unknown node":     `{"type":"doc","content":[{"type":"iframe"}]}`,
		"external image":   `{"type":"doc","content":[{"type":"image","attrs":{"src":"https://evil"}}]}`,
		"h1":               `{"type":"doc","content":[{"type":"heading","attrs":{"level":1}}]}`,
		"script link":      `{"type":"doc","content":[{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":"javascript:alert(1)"}}]}]}`,
		"unknown mark":     `{"type":"doc","content":[{"type":"text","text":"x","marks":[{"type":"textStyle"}]}]}`,
		"attachment extra": `{"type":"doc","content":[{"type":"attachmentImage","attrs":{"attachmentId":"00000000-0000-7000-8000-0000000000d1","src":"x"}}]}`,
		"bad attachment":   `{"type":"doc","content":[{"type":"attachmentImage","attrs":{"attachmentId":"nope"}}]}`,
		"too deep":         `{"type":"doc","content":[` + strings.Repeat(`{"type":"blockquote","content":[`, 14) + strings.Repeat(`]}`, 14) + `]}`,
		"too big":          `{"type":"doc","content":[{"type":"text","text":"` + strings.Repeat("a", 210<<10) + `"}]}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseProofDoc([]byte(doc)); !errors.Is(err, ErrInvalidProofDoc) {
				t.Fatalf("want ErrInvalidProofDoc, got %v", err)
			}
		})
	}
}

func TestValidateLinks(t *testing.T) {
	if err := ValidateLinks([]string{"https://youtu.be/abc", "http://notes.example/x"}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{{"ftp://x"}, {"https://"}, {"javascript:alert(1)"}, make([]string, 21)} {
		if err := ValidateLinks(bad); !errors.Is(err, ErrInvalidProofDoc) {
			t.Fatalf("%v should fail", bad)
		}
	}
}
