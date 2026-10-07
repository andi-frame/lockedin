package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// Date is a calendar date without a time zone (a check-in's local_date).
// The zero value is not a valid date.
type Date struct{ t time.Time } // always midnight UTC

const dateLayout = "2006-01-02"

func NewDate(year int, month time.Month, day int) Date {
	return Date{time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// ParseDate parses YYYY-MM-DD and rejects impossible dates such as 2026-02-30.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return Date{t}, nil
}

func MustDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Date) String() string        { return d.t.Format(dateLayout) }
func (d Date) IsZero() bool          { return d.t.IsZero() }
func (d Date) AddDays(n int) Date    { return Date{d.t.AddDate(0, 0, n)} }
func (d Date) Weekday() time.Weekday { return d.t.Weekday() }
func (d Date) Before(o Date) bool    { return d.t.Before(o.t) }
func (d Date) After(o Date) bool     { return d.t.After(o.t) }
func (d Date) Equal(o Date) bool     { return d.t.Equal(o.t) }
func (d Date) DaysUntil(o Date) int  { return int(o.t.Sub(d.t).Hours() / 24) }
func (d Date) Time() time.Time       { return d.t }

// ISOWeekday returns 1 (Monday) … 7 (Sunday), the encoding used in pact schedules.
func (d Date) ISOWeekday() int {
	if wd := int(d.t.Weekday()); wd != 0 {
		return wd
	}
	return 7
}

// At returns the instant of hh:mm on this date in loc.
func (d Date) At(hour, minute int, loc *time.Location) time.Time {
	return time.Date(d.t.Year(), d.t.Month(), d.t.Day(), hour, minute, 0, 0, loc)
}

// DateIn returns the calendar date of instant t in loc.
func DateIn(t time.Time, loc *time.Location) Date {
	l := t.In(loc)
	return NewDate(l.Year(), l.Month(), l.Day())
}

func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
