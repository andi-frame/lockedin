package ctl

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// Show renders a pact for an operator: status, members, every check-in and the ledger. It reads
// the database directly and bypasses the membership filter on purpose: it is an admin tool run by
// someone who already has database access.
func Show(ctx context.Context, st *store.Store, id uuid.UUID) (string, error) {
	p, err := st.GetPact(ctx, id)
	if err != nil {
		if store.IsNoRows(err) {
			return "", fmt.Errorf("no pact %s", id)
		}
		return "", err
	}
	members, err := st.ListPactMembers(ctx, id)
	if err != nil {
		return "", err
	}
	names := make(map[uuid.UUID]string, len(members))
	for _, m := range members {
		names[m.UserID] = m.DisplayName
	}
	checkIns, err := st.ListCheckInsForPact(ctx, store.ListCheckInsForPactParams{
		PactID: id, FromDate: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), ToDate: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		return "", err
	}
	ledger, err := st.ListLedgerPage(ctx, store.ListLedgerPageParams{PactID: id, MaxRows: 1000})
	if err != nil {
		return "", err
	}
	balance, err := st.PotBalance(ctx, id)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s  status=%s  %s → %s  tz=%s\n", p.Title, p.ID, p.Status, p.StartsOn.Format("2006-01-02"), p.EndsOn.Format("2006-01-02"), p.Timezone)
	fmt.Fprintf(&b, "terms v%d  hash %s\n\nmembers\n", p.TermsVersion, p.TermsHash)
	for _, m := range members {
		signed := "not signed"
		if m.AcceptedAt != nil {
			signed = "signed " + m.AcceptedAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(&b, "  %-7s %s <%s>  %s\n", m.Role, m.DisplayName, m.Email, signed)
	}

	tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "\ncheck-ins\ndate\tmember\tstatus\tfinal\tpenalty\tsubmit deadline (UTC)\n")
	for _, c := range checkIns {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%t\t%t\t%s\n", c.LocalDate.Format("2006-01-02"), names[c.MemberID], c.Status, c.IsFinal, c.PenaltyApplied, c.SubmitDeadline.UTC().Format("2006-01-02 15:04"))
	}
	fmt.Fprintf(tw, "\nledger\tid\tkind\tamount\tbalance after\tnote\n")
	for _, l := range ledger {
		note := ""
		if l.Note != nil {
			note = *l.Note
		}
		fmt.Fprintf(tw, "\t%d\t%s\t%+d\t%d\t%s\n", l.ID, l.Kind, l.Amount, l.BalanceAfter, note)
	}
	_ = tw.Flush()
	fmt.Fprintf(&b, "\nbalance %d coins\n", balance)
	return b.String(), nil
}
