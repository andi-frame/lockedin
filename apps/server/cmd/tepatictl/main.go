// Command tepatictl is the admin CLI: development seeds and a pact inspector.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/buildinfo"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
	"github.com/andi-frame/lockedin/apps/server/internal/ctl"
	"github.com/andi-frame/lockedin/apps/server/internal/storage"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

const usage = `usage: tepatictl <command>

commands:
  version                          print the build version
  seed --scenario overdue|invite|today|passbook|review|settlement   create development data
                                     overdue  an active pact with check-ins past their deadline;
                                              the worker should mark them missed within 2 minutes
                                     invite   a proposed pact with an open invite link
                                     today    new users with four active pacts whose check-ins
                                              dated today are open, submitted, approved and missed
                                     passbook one active pact with 24 days of printed lines, the
                                              doer's day today still open
                                     review   a backer with three proofs to review: two submitted
                                              and one auto-approved, inside its override window
                                     settlement two pacts whose days are over and settled, in settling
                                              with 900 coins owed, waiting for payout
  advance --pact <id>              move a clock to just past the pact's next open deadline and run
                                   the real sweep (the e2e's test clock)
  pact show <id>                   print a pact, its check-ins and its ledger
  probe [--timeout 3s] <url>       exit 0 if the URL answers 2xx (the containers' health check)
  storage-init                     set the staging bucket's CORS rule for APP_BASE_URL (run by the
                                   deploy, from inside the compose network; safe to repeat)

Reads DATABASE_URL and the rest of the app config from the environment (see .env).
`

func main() {
	ctx, stop := signalContext()
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "tepatictl:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(out, usage)
		return errors.New("usage: a command is required")
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(out, buildinfo.Version)
		return nil
	case "seed":
		return runSeed(ctx, args[1:], out)
	case "pact":
		return runPact(ctx, args[1:], out)
	case "advance":
		return runAdvance(ctx, args[1:], out)
	case "probe":
		return runProbe(ctx, args[1:])
	case "storage-init":
		return runStorageInit(ctx, out)
	default:
		fmt.Fprint(out, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runSeed(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	scenario := fs.String("scenario", "", "overdue, invite, today, passbook, review or settlement")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch *scenario {
	case "":
		return errors.New("seed needs --scenario overdue|invite|today|passbook|review|settlement")
	case "overdue", "invite", "today", "passbook", "review", "settlement":
	default:
		return fmt.Errorf("unknown scenario %q (want overdue, invite, today, passbook, review or settlement)", *scenario)
	}

	cfg, st, closeDB, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeDB()

	now := time.Now()
	var s ctl.Seeded
	switch *scenario {
	case "overdue":
		s, err = ctl.SeedOverdue(ctx, st, now)
	case "invite":
		s, err = ctl.SeedInvite(ctx, st, now)
	case "passbook":
		s, err = ctl.SeedPassbook(ctx, st, now, strconv.FormatInt(now.UnixMilli(), 36))
	case "review":
		s, err = ctl.SeedReview(ctx, st, now, strconv.FormatInt(now.UnixMilli(), 36))
	case "settlement":
		s, err = ctl.SeedSettlement(ctx, st, now, strconv.FormatInt(now.UnixMilli(), 36))
	default:
		// A new tag per run gives new users, so repeated runs never hit the open-pact limit.
		s, err = ctl.SeedToday(ctx, st, now, strconv.FormatInt(now.UnixMilli(), 36))
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "pact     %s  (%s)\n", s.Pact.ID, s.Pact.Status)
	fmt.Fprintf(out, "backer   %s  password %s\n", s.Backer.Email, ctl.SeedPassword)
	fmt.Fprintf(out, "doer     %s  password %s\n", s.Doer.Email, ctl.SeedPassword)
	if s.InviteToken != "" {
		fmt.Fprintf(out, "invite   %s/invite/%s  (to %s)\n", cfg.BaseURL, s.InviteToken, s.InviteEmail)
	}
	fmt.Fprintf(out, "inspect  tepatictl pact show %s\n", s.Pact.ID)
	return nil
}

func runPact(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "show" {
		return errors.New("usage: tepatictl pact show <id>")
	}
	if len(args) < 2 {
		return errors.New("pact show needs a pact id")
	}
	id, err := uuid.Parse(args[1])
	if err != nil {
		return fmt.Errorf("%q is not a UUID", args[1])
	}
	_, st, closeDB, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeDB()
	text, err := ctl.Show(ctx, st, id)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

func runAdvance(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("advance", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pact := fs.String("pact", "", "the pact id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := uuid.Parse(*pact)
	if err != nil {
		return errors.New("usage: tepatictl advance --pact <id>")
	}
	_, st, closeDB, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeDB()
	adv, err := ctl.Advance(ctx, st, id)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "advanced to %s: %d check-in(s) moved\n", adv.At.Format(time.RFC3339), adv.Moved)
	return nil
}

func runStorageInit(ctx context.Context, out io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	blobs, err := storage.FromConfig(cfg)
	if err != nil {
		return err
	}
	s3, ok := blobs.(*storage.S3)
	if !ok {
		return fmt.Errorf("storage-init: STORAGE_DRIVER=%s has no bucket to configure (use s3)", cfg.Storage.Driver)
	}
	if err := s3.PutStagingCORS(ctx, cfg.BaseURL); err != nil {
		return err
	}
	fmt.Fprintf(out, "CORS on %s allows PUT from %s\n", cfg.Storage.BucketStaging, cfg.BaseURL)
	return nil
}

func open(ctx context.Context) (config.Config, *store.Store, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	return cfg, store.NewStore(pool), pool.Close, nil
}
