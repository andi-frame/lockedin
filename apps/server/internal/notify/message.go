// Package notify turns service-level notifications into emails: which kinds are mailed
// (SPEC §9), the Indonesian copy, and the SMTP transport. It never touches the database.
package notify

import "context"

// Message is one email, ready to send.
type Message struct {
	To      string
	ToName  string
	Subject string
	Text    string // plain text part, always present
	HTML    string
}

// Sender delivers a message. SMTP is the only real implementation; tests use a fake.
type Sender interface {
	Send(ctx context.Context, m Message) error
}
