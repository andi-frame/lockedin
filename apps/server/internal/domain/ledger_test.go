package domain

import "testing"

func TestClampPenalty(t *testing.T) {
	cases := []struct {
		name                    string
		balance, penalty, floor int64
		want                    int64
	}{
		{"plenty of room", 1000, 50, 0, 50},
		{"exactly to floor", 50, 50, 0, 50},
		{"partially clamped", 30, 50, 0, 30},
		{"at floor clamps to zero", 0, 50, 0, 0},
		{"custom floor", 120, 50, 100, 20},
		{"below floor never goes negative", 90, 50, 100, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClampPenalty(tc.balance, tc.penalty, tc.floor); got != tc.want {
				t.Fatalf("want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestClampBonus(t *testing.T) {
	cases := []struct {
		name           string
		balance, bonus int64
		cap            *int64
		want           int64
	}{
		{"no cap", 1000, 50, nil, 50},
		{"room under cap", 1000, 50, i64(1500), 50},
		{"partially clamped", 1480, 50, i64(1500), 20},
		{"at cap clamps to zero", 1500, 50, i64(1500), 0},
		{"above cap never negative", 1600, 50, i64(1500), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClampBonus(tc.balance, tc.bonus, tc.cap); got != tc.want {
				t.Fatalf("want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestPenaltyEntrySignAndKind(t *testing.T) {
	terms := sampleTerms()
	doer := PenaltyEntry(terms, RoleDoer, 50, 1000)
	if doer.Kind != LedgerDoerMiss || doer.Amount != -50 {
		t.Fatalf("doer miss: %+v", doer)
	}
	backer := PenaltyEntry(terms, RoleBacker, 50, 1480)
	if backer.Kind != LedgerBackerMiss || backer.Amount != 20 {
		t.Fatalf("backer miss clamped by cap: %+v", backer)
	}
	if !backer.Clamped || doer.Clamped {
		t.Fatalf("clamped flags wrong: doer=%v backer=%v", doer.Clamped, backer.Clamped)
	}
}

func TestIdempotencyKeys(t *testing.T) {
	if PenaltyKey(doerID) != "checkin:"+doerID.String()+":penalty" {
		t.Fatal("penalty key format")
	}
	if ReversalKey(doerID) != "checkin:"+doerID.String()+":reversal" {
		t.Fatal("reversal key format")
	}
	if InitialPotKey(backerID) != "pact:"+backerID.String()+":initial" || PayoutKey(backerID) != "pact:"+backerID.String()+":payout" {
		t.Fatal("pact key format")
	}
}
