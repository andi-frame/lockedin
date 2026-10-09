package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

func TestDeliveryFollowsTheSpecTable(t *testing.T) {
	// SPEC §9: which notifications also reach the inbox of an email address.
	for kind, want := range map[string]Delivery{
		"terms_changed":       Immediate,
		"terms_signed":        Immediate,
		"proof_submitted":     Digest,
		"proof_rejected":      Immediate,
		"proof_overridden":    Immediate,
		"proof_auto_approved": Immediate,
		"dispute_opened":      Immediate,
		"pact_settled":        Immediate,
		// in-app only
		"reminder_cutoff_3h":   InAppOnly,
		"reminder_cutoff_30m":  InAppOnly,
		"review_deadline_soon": InAppOnly,
		"member_joined":        InAppOnly,
		"day_missed":           InAppOnly,
		"no_such_kind":         InAppOnly,
	} {
		if got := DeliveryFor(kind); got != want {
			t.Errorf("DeliveryFor(%q) = %v, want %v", kind, got, want)
		}
	}
}

func TestEveryEmailedKindHasCopy(t *testing.T) {
	r := NewRenderer("http://localhost:3000")
	for _, kind := range EmailedKinds() {
		var msg Message
		var err error
		if DeliveryFor(kind) == Digest {
			msg, err = r.Digest(service.DigestEmail{Kind: kind, To: "a@b.test", ToName: "Bima", PactID: uuid.New(), PactTitle: "UTBK", Count: 2})
		} else {
			msg, err = r.Notification(service.NotificationEmail{Kind: kind, To: "a@b.test", ToName: "Bima", PactID: uuid.New(), PactTitle: "UTBK"})
		}
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if msg.Subject == "" || msg.Text == "" || msg.HTML == "" {
			t.Errorf("%s: incomplete message %+v", kind, msg)
		}
	}
}

func TestNotificationMailIsIndonesianWithALinkToThePact(t *testing.T) {
	r := NewRenderer("https://tepati.example/")
	pact := uuid.MustParse("01a116e3-178e-7380-bc60-1b1d9d6a6569")
	msg, err := r.Notification(service.NotificationEmail{Kind: "proof_rejected", To: "bima@tepati.test", ToName: "Bima", PactID: pact, PactTitle: "UTBK November"})
	if err != nil {
		t.Fatal(err)
	}
	if msg.To != "bima@tepati.test" || msg.ToName != "Bima" {
		t.Fatalf("recipient = %q %q", msg.To, msg.ToName)
	}
	if !strings.Contains(msg.Subject, "UTBK November") || !strings.Contains(strings.ToLower(msg.Subject), "ditolak") {
		t.Fatalf("subject = %q", msg.Subject)
	}
	link := "https://tepati.example/pacts/01a116e3-178e-7380-bc60-1b1d9d6a6569"
	for name, body := range map[string]string{"text": msg.Text, "html": msg.HTML} {
		if !strings.Contains(body, link) {
			t.Errorf("%s part lacks %s:\n%s", name, link, body)
		}
		if !strings.Contains(body, "Bima") {
			t.Errorf("%s part does not greet the recipient", name)
		}
	}
	if strings.Contains(msg.Text, "<") {
		t.Errorf("the plain text part carries markup:\n%s", msg.Text)
	}
}

func TestMailEscapesUserSuppliedTitles(t *testing.T) {
	r := NewRenderer("http://localhost:3000")
	title := `<script>alert(1)</script> "UTBK"`
	msg, err := r.Notification(service.NotificationEmail{Kind: "dispute_opened", To: "a@b.test", ToName: "Andi", PactID: uuid.New(), PactTitle: title})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.HTML, "<script>") {
		t.Fatalf("title was not escaped in the HTML part:\n%s", msg.HTML)
	}
	if !strings.Contains(msg.Text, title) {
		t.Fatal("the plain text part should show the title as typed")
	}
}

func TestSubjectCannotInjectHeaders(t *testing.T) {
	r := NewRenderer("http://localhost:3000")
	msg, err := r.Notification(service.NotificationEmail{Kind: "pact_settled", To: "a@b.test", ToName: "Andi", PactID: uuid.New(), PactTitle: "UTBK\r\nBcc: x@evil.test"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		t.Fatalf("subject has a line break: %q", msg.Subject)
	}
}

func TestDigestCountsTheEvents(t *testing.T) {
	r := NewRenderer("http://localhost:3000")
	msg, err := r.Digest(service.DigestEmail{Kind: "proof_submitted", To: "a@b.test", ToName: "Andi", PactID: uuid.New(), PactTitle: "UTBK", Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg.Text, "3 bukti") {
		t.Fatalf("digest does not say how many proofs:\n%s", msg.Text)
	}
	one, _ := r.Digest(service.DigestEmail{Kind: "proof_submitted", To: "a@b.test", ToName: "Andi", PactID: uuid.New(), PactTitle: "UTBK", Count: 1})
	if !strings.Contains(one.Text, "1 bukti") {
		t.Fatalf("singular digest:\n%s", one.Text)
	}
}

func TestInviteMailCarriesTheLinkAndExpiry(t *testing.T) {
	r := NewRenderer("https://tepati.example")
	exp := time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC) // 09:00 WIB
	msg, err := r.Invite(service.InviteEmail{To: "calon@tepati.test", PactTitle: "UTBK", BackerName: "Andi", ExpiresAt: exp, Token: "tok_123"})
	if err != nil {
		t.Fatal(err)
	}
	if msg.To != "calon@tepati.test" || !strings.Contains(msg.Subject, "Andi") {
		t.Fatalf("message = %+v", msg)
	}
	for name, body := range map[string]string{"text": msg.Text, "html": msg.HTML} {
		if !strings.Contains(body, "https://tepati.example/invite/tok_123") {
			t.Errorf("%s part lacks the invite link:\n%s", name, body)
		}
		if !strings.Contains(body, "3 November 2026") {
			t.Errorf("%s part lacks the expiry date:\n%s", name, body)
		}
	}
}

func TestInviteTokenIsEscapedInTheLink(t *testing.T) {
	r := NewRenderer("https://tepati.example")
	msg, err := r.Invite(service.InviteEmail{To: "c@t.test", PactTitle: "UTBK", BackerName: "Andi", ExpiresAt: time.Now(), Token: "a b/c?d"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg.Text, "/invite/a%20b%2Fc%3Fd") {
		t.Fatalf("token not path-escaped:\n%s", msg.Text)
	}
}

// PLAN 9.4: adding an emailed kind forces a decision about whether a person may switch it off
// (domain.IsSwitchableEmailKind); this list is the other half of that decision.
func TestEveryEmailedKindIsSwitchableOrMustStayOn(t *testing.T) {
	mustStayOn := map[string]bool{"dispute_opened": true, "pact_settled": true}
	emailed := map[string]bool{}
	for _, kind := range EmailedKinds() {
		emailed[kind] = true
		if domain.IsSwitchableEmailKind(kind) == mustStayOn[kind] {
			t.Errorf("%s: decide it: it is switchable=%v and listed as must-stay-on=%v here", kind, domain.IsSwitchableEmailKind(kind), mustStayOn[kind])
		}
	}
	for _, kind := range []string{"terms_changed", "terms_signed", "proof_submitted", "proof_rejected", "proof_overridden", "proof_auto_approved"} {
		if !emailed[kind] {
			t.Errorf("%s can be switched off but is never emailed", kind)
		}
	}
}
