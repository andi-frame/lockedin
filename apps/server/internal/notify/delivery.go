package notify

import "sort"

// Delivery says how a notification kind reaches the person, beyond the in-app inbox.
type Delivery int

const (
	InAppOnly Delivery = iota
	Immediate          // one email per notification
	Digest             // one email for a burst of notifications of the same kind in one pact
)

// SPEC §9: the "email" column. Reminders and the review deadline are in-app only; the other
// kinds the service emits (member_joined, day_missed, payout_*, ...) are not in the table, so
// they stay in-app too.
var delivery = map[string]Delivery{
	"terms_changed":       Immediate,
	"terms_signed":        Immediate,
	"proof_submitted":     Digest,
	"proof_rejected":      Immediate,
	"proof_overridden":    Immediate,
	"proof_auto_approved": Immediate,
	"dispute_opened":      Immediate,
	"pact_settled":        Immediate,
}

// DeliveryFor reports how a kind is delivered; unknown kinds are in-app only.
func DeliveryFor(kind string) Delivery { return delivery[kind] }

// EmailedKinds lists every kind that produces an email, sorted.
func EmailedKinds() []string {
	kinds := make([]string, 0, len(delivery))
	for k := range delivery {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}
