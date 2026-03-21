// Package registrygc implements the registry garbage collector.
//
// The GC identifies objects that are unreachable from any tag (and have not
// been accessed in 24 hours), deletes them from blob storage, then removes
// their database records.  Storage is always cleaned up before the DB record
// is removed: if a storage delete fails the DB row is left intact so the next
// GC pass will retry.  Because S3/R2 DeleteObject is idempotent, a blob that
// was already removed from storage will silently succeed on the next attempt,
// allowing the DB record to be cleaned up.
package registrygc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"bin2.io/internal/db"
	"bin2.io/internal/registrystorage"
)

// Config controls GC behaviour.
type Config struct {
	// BatchSize is the maximum number of objects collected per pass.
	// Defaults to 100.
	BatchSize int

	// Interval is the pause between passes when using Run.
	// Defaults to 5 minutes.
	Interval time.Duration

	// MinAge is the minimum age an unreferenced object must have before it is
	// eligible for collection.  Defaults to 24 hours.  Set lower for testing.
	MinAge time.Duration
}

// Runner performs GC passes against a registry database and blob store.
type Runner struct {
	db        *db.DB
	storage   registrystorage.BlobDeleter
	batchSize int
	interval  time.Duration
	minAge    time.Duration
}

// New creates a Runner.  Zero-value Config fields are replaced with defaults.
func New(database *db.DB, storage registrystorage.BlobDeleter, cfg Config) *Runner {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.MinAge < 0 {
		cfg.MinAge = 24 * time.Hour
	}
	return &Runner{
		db:        database,
		storage:   storage,
		batchSize: cfg.BatchSize,
		interval:  cfg.Interval,
		minAge:    cfg.MinAge,
	}
}

// Diagnose logs object counts at each stage of the GC filter pipeline.
// Useful for understanding why RunOnce collects 0 objects.
func (r *Runner) Diagnose(ctx context.Context) {
	diag, err := r.db.DiagnoseGC(ctx, r.minAge)
	if err != nil {
		slog.Warn("registrygc: diagnose failed", slog.Any("err", err))
		return
	}
	slog.Info("registrygc: diagnostics",
		slog.Int("total_objects", diag.TotalObjects),
		slog.Int("reachable", diag.ReachableObjects),
		slog.Int("unreachable", diag.UnreachableObjects),
		slog.Int("eligible_for_collection", diag.EligibleObjects),
		slog.Duration("min_age", r.minAge),
	)
}

// RunOnce performs a single GC pass and returns the number of objects collected.
//
// For each unreferenced object:
//  1. Delete from blob storage (idempotent; errors are logged and skipped).
//  2. Delete the database record.
//
// Objects whose storage deletion fails are skipped for this pass; the DB
// record is left intact so the next pass will retry.
func (r *Runner) RunOnce(ctx context.Context) (int, error) {
	digests, err := r.db.ListUnreferencedObjectDigests(ctx, r.batchSize, r.minAge)
	if err != nil {
		return 0, fmt.Errorf("list unreferenced objects: %w", err)
	}

	var collected int
	for _, digest := range digests {
		digestHex, ok := digestToHex(digest)
		if !ok {
			slog.Warn("registrygc: skipping malformed digest", slog.String("digest", digest))
			continue
		}

		if err := r.storage.DeleteBlob(ctx, digestHex); err != nil {
			slog.Warn("registrygc: storage delete failed, will retry next pass",
				slog.String("digest", digest),
				slog.Any("err", err),
			)
			continue
		}

		if err := r.db.DeleteObject(ctx, digest); err != nil {
			// Blob is gone from storage but DB record remains.  The next pass
			// will call DeleteBlob again (no-op on S3/R2) then clean up the DB.
			slog.Warn("registrygc: db delete failed after storage delete",
				slog.String("digest", digest),
				slog.Any("err", err),
			)
			continue
		}

		slog.Info("registrygc: collected", slog.String("digest", digest))
		collected++
	}
	return collected, nil
}

// Run executes GC passes in a loop, pausing Config.Interval between each pass,
// until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	slog.Info("registrygc: starting",
		slog.Int("batch_size", r.batchSize),
		slog.Duration("interval", r.interval),
	)
	for {
		n, err := r.RunOnce(ctx)
		if err != nil {
			slog.Error("registrygc: pass failed", slog.Any("err", err))
		} else {
			slog.Info("registrygc: pass complete", slog.Int("collected", n))
		}

		select {
		case <-ctx.Done():
			slog.Info("registrygc: stopping")
			return ctx.Err()
		case <-time.After(r.interval):
		}
	}
}

// digestToHex strips the "sha256:" prefix and validates the remainder.
func digestToHex(digest string) (string, bool) {
	hex, ok := strings.CutPrefix(digest, "sha256:")
	if !ok || len(hex) != 64 {
		return "", false
	}
	return hex, true
}
