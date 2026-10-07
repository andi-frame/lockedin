package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	h, err := HashPassword("rahasia-belajar")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("PHC format with ARCHITECTURE §8 params: %s", h)
	}
	if ok, err := VerifyPassword("rahasia-belajar", h); err != nil || !ok {
		t.Fatalf("correct password: %v %v", ok, err)
	}
	if ok, _ := VerifyPassword("rahasia-belajaR", h); ok {
		t.Fatal("wrong password must fail")
	}
	h2, _ := HashPassword("rahasia-belajar")
	if h2 == h {
		t.Fatal("salts must differ")
	}
}

func TestPasswordPolicyAndBadHashes(t *testing.T) {
	for _, pw := range []string{"short", strings.Repeat("x", 129)} {
		if _, err := HashPassword(pw); !errors.Is(err, ErrWeakPassword) {
			t.Fatalf("%d chars should be rejected", len(pw))
		}
	}
	for _, bad := range []string{"", "$bcrypt$x", "$argon2id$v=1$m=1,t=1,p=1$AA$AA", "$argon2id$v=19$nonsense$AA$AA", "$argon2id$v=19$m=1,t=1,p=1$!!$AA"} {
		if ok, err := VerifyPassword("whatever123", bad); ok || err == nil {
			t.Fatalf("hash %q must not verify", bad)
		}
	}
}

func TestCSRFIsBoundToTheSession(t *testing.T) {
	s := NewSessions(nil, strings.Repeat("k", 64))
	tok := s.CSRFToken("session-a")
	if !s.ValidCSRF("session-a", tok) {
		t.Fatal("matching token")
	}
	if s.ValidCSRF("session-b", tok) {
		t.Fatal("a token from another session must fail")
	}
	if s.ValidCSRF("session-a", "") || s.ValidCSRF("", tok) {
		t.Fatal("missing values must fail")
	}
	other := NewSessions(nil, strings.Repeat("z", 64))
	if other.ValidCSRF("session-a", tok) {
		t.Fatal("a different secret must fail")
	}
}

func TestNormaliseEmail(t *testing.T) {
	if e, err := normaliseEmail("  Andi@Tepati.ID "); err != nil || e != "andi@tepati.id" {
		t.Fatalf("got %q %v", e, err)
	}
	for _, bad := range []string{"", "andi", "Andi <andi@x.id>", "a@"} {
		if _, err := normaliseEmail(bad); !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("%q should be invalid", bad)
		}
	}
}
