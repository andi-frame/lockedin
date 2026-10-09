package domain

import (
	"reflect"
	"testing"
)

func TestSwitchableEmailKinds(t *testing.T) {
	// SPEC §9: a person can switch off the emails whose content is also in the app and which
	// carry no deadline that works against them. Dispute and settlement emails stay on.
	for kind, want := range map[string]bool{
		"proof_submitted":     true,
		"proof_rejected":      true,
		"proof_overridden":    true,
		"proof_auto_approved": true,
		"terms_changed":       true,
		"terms_signed":        true,
		"dispute_opened":      false,
		"pact_settled":        false,
		"invite_mail":         false,
		"day_missed":          false,
		"no_such_kind":        false,
	} {
		if got := IsSwitchableEmailKind(kind); got != want {
			t.Errorf("IsSwitchableEmailKind(%q) = %v, want %v", kind, got, want)
		}
	}
}

func TestNormaliseEmailOff(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"empty stays empty and not nil", nil, []string{}, false},
		{"sorted and deduplicated", []string{"terms_signed", "proof_rejected", "terms_signed"}, []string{"proof_rejected", "terms_signed"}, false},
		{"a kind that must stay on is refused", []string{"proof_rejected", "dispute_opened"}, nil, true},
		{"an unknown kind is refused", []string{"nope"}, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormaliseEmailOff(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
