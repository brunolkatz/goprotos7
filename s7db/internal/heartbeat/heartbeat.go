package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
)

type Client interface {
	Connect(ctx context.Context) error
	Close() error
	ReadBool(ctx context.Context, db, by, bit int) (bool, error)
	WriteBool(ctx context.Context, db, by, bit int, value bool) error
}

type Config struct {
	Address      address.Address
	Name         string
	Interval     time.Duration
	Timeout      time.Duration
	Mode         string
	Count        int
	Once         bool
	NoReadback   bool
	RequireClear bool
	ClearWithin  time.Duration
	Reconnect    bool
	Strict       bool
	Quiet        bool
	Verbose      bool
	JSON         bool
	DryRun       bool

	Now   func() time.Time
	Out   io.Writer
	Err   io.Writer
	Sleep func(context.Context, time.Duration) error
}

func Run(ctx context.Context, client Client, cfg Config) error {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Sleep == nil {
		cfg.Sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	if cfg.Mode == "" {
		cfg.Mode = "set-true"
	}

	connectOrRetry := func() error {
		if cfg.DryRun {
			return nil
		}
		var firstErrAt time.Time
		backoff := time.Second
		for {
			err := client.Connect(ctx)
			if err == nil {
				return nil
			}
			if !cfg.Reconnect {
				return err
			}
			if firstErrAt.IsZero() {
				firstErrAt = cfg.Now()
			}
			if cfg.Now().Sub(firstErrAt) > cfg.Timeout {
				return fmt.Errorf("heartbeat reconnect timeout exceeded (%s): %w", cfg.Timeout, err)
			}
			if cfg.Err != nil && !cfg.Quiet {
				fmt.Fprintf(cfg.Err, "reconnect failed: %v\n", err)
			}
			if err := cfg.Sleep(ctx, backoff); err != nil {
				return err
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}

	if err := connectOrRetry(); err != nil {
		return err
	}
	if !cfg.DryRun {
		defer client.Close()
	}

	var lastToggle bool
	haveLastToggle := false
	for beat := 1; ; beat++ {
		if ctx.Err() != nil {
			return nil
		}
		start := cfg.Now()
		var wrote bool
		var readback any = nil
		var opErr error

		if cfg.DryRun {
			if cfg.Mode == "toggle" {
				lastToggle = !lastToggle
				wrote = lastToggle
			} else if cfg.Mode == "set-false" {
				wrote = false
			} else {
				wrote = true
			}
		} else {
			switch cfg.Mode {
			case "toggle":
				var cur bool
				if haveLastToggle {
					cur = lastToggle
				} else {
					cur, opErr = client.ReadBool(ctx, cfg.Address.DB, cfg.Address.Byte, cfg.Address.Bit)
				}
				if opErr == nil {
					wrote = !cur
					opErr = client.WriteBool(ctx, cfg.Address.DB, cfg.Address.Byte, cfg.Address.Bit, wrote)
					if opErr == nil {
						lastToggle = wrote
						haveLastToggle = true
					}
				}
			case "set-false":
				var cur bool
				cur, opErr = client.ReadBool(ctx, cfg.Address.DB, cfg.Address.Byte, cfg.Address.Bit)
				if opErr == nil {
					if cur {
						opErr = client.WriteBool(ctx, cfg.Address.DB, cfg.Address.Byte, cfg.Address.Bit, false)
					}
				}
			default: // set-true
				wrote = true
				opErr = client.WriteBool(ctx, cfg.Address.DB, cfg.Address.Byte, cfg.Address.Bit, true)
			}
		}

		if opErr != nil {
			_ = client.Close()
			if err := connectOrRetry(); err != nil {
				return err
			}
			continue
		}

		if !cfg.NoReadback && !cfg.DryRun {
			v, err := client.ReadBool(ctx, cfg.Address.DB, cfg.Address.Byte, cfg.Address.Bit)
			if err != nil {
				_ = client.Close()
				if err := connectOrRetry(); err != nil {
					return err
				}
				continue
			}
			readback = v
		}
		status := "ok"
		if cfg.RequireClear && (cfg.Mode == "set-true" || cfg.Mode == "set-false") && !cfg.DryRun {
			if err := cfg.Sleep(ctx, cfg.ClearWithin); err != nil {
				return nil
			}
			clearVal, err := client.ReadBool(ctx, cfg.Address.DB, cfg.Address.Byte, cfg.Address.Bit)
			if err != nil {
				status = "clear-check-error"
				if cfg.Strict {
					return fmt.Errorf("clear verification failed: %w", err)
				}
			} else if clearVal {
				status = "clear-missing"
				if cfg.Strict {
					return fmt.Errorf("heartbeat bit %s was not cleared within %s", cfg.Address.Canonical(), cfg.ClearWithin)
				}
			}
		}

		if cfg.JSON {
			payload := map[string]any{
				"ts":       cfg.Now().UTC().Format(time.RFC3339Nano),
				"beat":     beat,
				"addr":     cfg.Address.Canonical(),
				"name":     cfg.Name,
				"wrote":    wrote,
				"readback": readback,
				"rtt_ms":   cfg.Now().Sub(start).Milliseconds(),
				"status":   status,
			}
			if err := json.NewEncoder(cfg.Out).Encode(payload); err != nil {
				return err
			}
		} else if !cfg.Quiet && cfg.Err != nil {
			fmt.Fprintf(
				cfg.Err,
				"%s  beat=%d  addr=%s  wrote=%v  readback=%v  %s\n",
				cfg.Now().UTC().Format(time.RFC3339Nano),
				beat,
				cfg.Address.Canonical(),
				wrote,
				readback,
				status,
			)
		}

		if cfg.Once || (cfg.Count > 0 && beat >= cfg.Count) {
			return nil
		}
		if err := cfg.Sleep(ctx, cfg.Interval); err != nil {
			return nil
		}
	}
}
