package domain

import "github.com/google/uuid"

type LedgerKind string

const (
	LedgerPotInitial LedgerKind = "pot_initial"
	LedgerDoerMiss   LedgerKind = "doer_miss"
	LedgerBackerMiss LedgerKind = "backer_miss"
	LedgerReversal   LedgerKind = "reversal"
	LedgerPayout     LedgerKind = "payout"
)

// ClampPenalty returns how many coins a doer miss may remove so the pot never
// drops below the floor (SPEC §6). The result is >= 0.
func ClampPenalty(balance, penalty, floor int64) int64 {
	room := balance - floor
	if room <= 0 {
		return 0
	}
	return min(penalty, room)
}

// ClampBonus returns how many coins a backer miss may add without exceeding the cap.
func ClampBonus(balance, bonus int64, potCap *int64) int64 {
	if potCap == nil {
		return bonus
	}
	room := *potCap - balance
	if room <= 0 {
		return 0
	}
	return min(bonus, room)
}

// LedgerDraft is a ledger row computed by the domain; the service adds ids and keys.
type LedgerDraft struct {
	Kind    LedgerKind
	Amount  int64 // signed: positive grows the pot
	Clamped bool  // true when floor/cap reduced the amount; still recorded (SPEC §6)
}

// PenaltyEntry turns a final miss into a signed, clamped ledger amount.
// Doer misses shrink the pot; backer misses grow it.
func PenaltyEntry(t Terms, role Role, penalty, balance int64) LedgerDraft {
	if role == RoleBacker {
		amt := ClampBonus(balance, penalty, t.PotCap)
		return LedgerDraft{Kind: LedgerBackerMiss, Amount: amt, Clamped: amt != penalty}
	}
	amt := ClampPenalty(balance, penalty, t.PotFloor)
	return LedgerDraft{Kind: LedgerDoerMiss, Amount: -amt, Clamped: amt != penalty}
}

// Idempotency keys (SPEC §6, invariant L3). One economic event, one key.
func InitialPotKey(pactID uuid.UUID) string  { return "pact:" + pactID.String() + ":initial" }
func PayoutKey(pactID uuid.UUID) string      { return "pact:" + pactID.String() + ":payout" }
func PenaltyKey(checkInID uuid.UUID) string  { return "checkin:" + checkInID.String() + ":penalty" }
func ReversalKey(checkInID uuid.UUID) string { return "checkin:" + checkInID.String() + ":reversal" }
