package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleBacker Role = "backer"
	RoleDoer   Role = "doer"
)

// Terms is the frozen rule set both members sign (SPEC §4). It is stored as JSONB
// together with its hash; acceptance is only valid for an identical hash.
type Terms struct {
	Version                int                       `json:"version"`
	Timezone               string                    `json:"timezone"`
	StartsOn               Date                      `json:"starts_on"`
	EndsOn                 Date                      `json:"ends_on"`
	CutoffLocalTime        string                    `json:"cutoff_local_time"`
	GraceMinutes           int                       `json:"grace_minutes"`
	CoinRateIDR            int64                     `json:"coin_rate_idr"`
	InitialPot             int64                     `json:"initial_pot"`
	PotFloor               int64                     `json:"pot_floor"`
	PotCap                 *int64                    `json:"pot_cap"`
	ReviewWindowHours      int                       `json:"review_window_hours"`
	DisputeWindowHours     int                       `json:"dispute_window_hours"`
	DisputeResolutionHours int                       `json:"dispute_resolution_hours"`
	OverrideWindowHours    int                       `json:"override_window_hours"`
	MaxOverrides           int                       `json:"max_overrides"`
	BackerCommits          bool                      `json:"backer_commits"`
	Members                map[uuid.UUID]MemberTerms `json:"members"`
}

type MemberTerms struct {
	Role           Role     `json:"role"`
	Commitment     string   `json:"commitment"`
	Schedule       []int    `json:"schedule"` // ISO weekdays, 1 = Monday
	PenaltyPerMiss int64    `json:"penalty_per_miss"`
	RestDays       int      `json:"rest_days"`
	Evidence       Evidence `json:"evidence"`
}

type Evidence struct {
	MinAttachments int `json:"min_attachments"`
	MinWords       int `json:"min_words"`
}

func ParseTerms(raw []byte) (Terms, error) {
	var t Terms
	if err := json.Unmarshal(raw, &t); err != nil {
		return Terms{}, fmt.Errorf("%w: %v", ErrInvalidTerms, err)
	}
	return t, nil
}

// MarshalCanonical returns deterministic JSON: struct fields keep declaration
// order and encoding/json sorts map keys, so equal terms give equal bytes.
func (t Terms) MarshalCanonical() ([]byte, error) { return json.Marshal(t) }

// Hash is sha256(canonical JSON) as lowercase hex.
func (t Terms) Hash() (string, error) {
	raw, err := t.MarshalCanonical()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// Location loads the pact's IANA timezone.
func (t Terms) Location() (*time.Location, error) { return time.LoadLocation(t.Timezone) }

func (t Terms) cutoffClock() (hour, minute int, err error) {
	var h, m int
	if n, scanErr := fmt.Sscanf(t.CutoffLocalTime, "%02d:%02d", &h, &m); scanErr != nil || n != 2 ||
		len(t.CutoffLocalTime) != 5 || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("cutoff_local_time must be HH:MM (got %q)", t.CutoffLocalTime)
	}
	return h, m, nil
}

// Backer returns the backer's user id.
func (t Terms) Backer() (uuid.UUID, bool) {
	for id, m := range t.Members {
		if m.Role == RoleBacker {
			return id, true
		}
	}
	return uuid.Nil, false
}

// ReviewerOf returns the other member: nobody reviews their own check-in (SPEC §2).
func (t Terms) ReviewerOf(member uuid.UUID) (uuid.UUID, error) {
	if _, ok := t.Members[member]; !ok {
		return uuid.Nil, fmt.Errorf("%s is not a member", member)
	}
	for id := range t.Members {
		if id != member {
			return id, nil
		}
	}
	return uuid.Nil, errors.New("pact has no second member")
}

// ScheduledDates lists every date in [starts_on, ends_on] on the member's schedule.
func (t Terms) ScheduledDates(member uuid.UUID) []Date {
	m, ok := t.Members[member]
	if !ok || (m.Role == RoleBacker && !t.BackerCommits) {
		return nil
	}
	days := map[int]bool{}
	for _, wd := range m.Schedule {
		days[wd] = true
	}
	var out []Date
	for d := t.StartsOn; !d.After(t.EndsOn); d = d.AddDays(1) {
		if days[d.ISOWeekday()] {
			out = append(out, d)
		}
	}
	return out
}

// Validate checks SPEC §4. All problems are reported together.
func (t Terms) Validate() error {
	var errs []string
	add := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	if _, err := t.Location(); err != nil || t.Timezone == "" {
		add("timezone %q is not a valid IANA zone", t.Timezone)
	}
	if _, _, err := t.cutoffClock(); err != nil {
		add("%v", err)
	}
	if t.StartsOn.IsZero() || t.EndsOn.IsZero() || !t.EndsOn.After(t.StartsOn) {
		add("ends_on must be after starts_on")
	} else if t.StartsOn.DaysUntil(t.EndsOn) > 366 {
		add("a pact lasts at most 366 days")
	}
	if t.GraceMinutes < 0 || t.GraceMinutes > 180 {
		add("grace_minutes must be 0..180")
	}
	for name, h := range map[string]int{
		"review_window_hours":      t.ReviewWindowHours,
		"dispute_window_hours":     t.DisputeWindowHours,
		"dispute_resolution_hours": t.DisputeResolutionHours,
		"override_window_hours":    t.OverrideWindowHours,
	} {
		if h < 1 || h > 72 {
			add("%s must be 1..72", name)
		}
	}
	if t.MaxOverrides < 0 || t.MaxOverrides > 10 {
		add("max_overrides must be 0..10")
	}
	if t.CoinRateIDR < 1 {
		add("coin_rate_idr must be at least 1")
	}
	if t.InitialPot < 1 {
		add("initial_pot must be at least 1")
	}
	if t.PotFloor < 0 || t.PotFloor >= t.InitialPot {
		add("pot_floor must be 0 or more and below initial_pot")
	}
	if t.PotCap != nil && *t.PotCap < t.InitialPot {
		add("pot_cap must be at least initial_pot")
	}

	if len(t.Members) != 2 {
		add("a pact has exactly two members")
	} else {
		backers := 0
		for id, m := range t.Members {
			if m.Role == RoleBacker {
				backers++
			}
			t.validateMember(id, m, add)
		}
		if backers != 1 {
			add("a pact has exactly one backer")
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrInvalidTerms, strings.Join(errs, "; "))
}

func (t Terms) validateMember(id uuid.UUID, m MemberTerms, add func(string, ...any)) {
	commits := m.Role == RoleDoer || (m.Role == RoleBacker && t.BackerCommits)
	if m.Role != RoleBacker && m.Role != RoleDoer {
		add("member %s has unknown role %q", id, m.Role)
		return
	}
	if m.Role == RoleBacker && !t.BackerCommits && len(m.Schedule) > 0 {
		add("backer has a schedule but backer_commits is false")
	}
	if !commits {
		return
	}
	if len(m.Schedule) == 0 {
		add("member %s needs a schedule", id)
	}
	seen := map[int]bool{}
	for _, wd := range m.Schedule {
		if wd < 1 || wd > 7 || seen[wd] {
			add("member %s has an invalid or repeated weekday %d", id, wd)
		}
		seen[wd] = true
	}
	if l := len(strings.TrimSpace(m.Commitment)); l == 0 || l > 200 {
		add("member %s commitment must be 1..200 characters", id)
	}
	if m.PenaltyPerMiss < 1 {
		add("member %s penalty_per_miss must be at least 1", id)
	}
	if m.RestDays < 0 || m.RestDays > 366 {
		add("member %s rest_days must be 0 or more", id)
	}
	if m.Evidence.MinAttachments < 0 || m.Evidence.MinAttachments > 10 || m.Evidence.MinWords < 0 || m.Evidence.MinWords > 2000 {
		add("member %s evidence rules are out of range", id)
	}
}

// CheckInDeadlines returns cutoff_at and submit_deadline for a local date (SPEC §5).
func (t Terms) CheckInDeadlines(d Date) (cutoffAt, submitDeadline time.Time, err error) {
	loc, err := t.Location()
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	h, m, err := t.cutoffClock()
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	cutoffAt = d.At(h, m, loc).UTC()
	return cutoffAt, cutoffAt.Add(time.Duration(t.GraceMinutes) * time.Minute), nil
}

// ReviewDeadline counts from cutoff for early submitters so the reviewer always
// gets the full window (SPEC §5).
func (t Terms) ReviewDeadline(cutoffAt, submittedAt time.Time) time.Time {
	start := cutoffAt
	if submittedAt.After(cutoffAt) {
		start = submittedAt
	}
	return start.Add(hours(t.ReviewWindowHours))
}

func (t Terms) DisputeDeadline(rejectedAt time.Time) time.Time {
	return rejectedAt.Add(hours(t.DisputeWindowHours))
}

func (t Terms) ResolutionDeadline(disputedAt time.Time) time.Time {
	return disputedAt.Add(hours(t.DisputeResolutionHours))
}

func (t Terms) OverrideDeadline(autoApprovedAt time.Time) time.Time {
	return autoApprovedAt.Add(hours(t.OverrideWindowHours))
}

func hours(h int) time.Duration { return time.Duration(h) * time.Hour }

// CheckEvidence applies the member's evidence rules to a proof (SPEC §5).
// Attachments still processing never count.
func (e Evidence) Check(wordCount, readyAttachments, pendingAttachments int) error {
	if pendingAttachments > 0 || readyAttachments < e.MinAttachments || wordCount < e.MinWords {
		return ErrEvidenceInsufficient
	}
	return nil
}
