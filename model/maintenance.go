package model

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// RunMaintenance executes housekeeping tasks.
// Make sure tasks are idempotent and safe to run multiple times.
func RunMaintenance(ctx context.Context, s *Store) error {
	start := time.Now()
	log.Println("maintenance: start")

	// Try to acquire a DB-level singleton lock (Postgres only).
	unlock, err := tryAcquireLock(ctx, s)
	if err != nil {
		return err
	}
	if unlock != nil {
		defer unlock()
	}

	// 1) Delete API tokens that are either disabled or expired
	if err := deleteInvalidAPITokens(ctx, s); err != nil {
		return fmt.Errorf("delete invalid API tokens: %w", err)
	}

	// 2) Delete expired signup tokens
	if err := deleteExpiredSignupTokens(ctx, s); err != nil {
		return fmt.Errorf("delete expired signup tokens: %w", err)
	}

	// 3) Prune old recent views (older than 90 days)
	if err := pruneOldRecentViews(ctx, s, 90*24*time.Hour); err != nil {
		return fmt.Errorf("prune recent views: %w", err)
	}

	// 4) Run VACUUM/ANALYZE depending on the DB engine
	if err := vacuumAnalyze(ctx, s); err != nil {
		return fmt.Errorf("vacuum/analyze: %w", err)
	}

	// // 5) Delete stale files in XMLDir (older than 30 days)
	// _ = pruneTempFiles(s.Config.XMLDir, 30*24*time.Hour)

	log.Printf("maintenance: done in %s", time.Since(start).Truncate(time.Millisecond))
	return nil
}

// --------------------------------------------------------------------
// DB locking (only relevant for Postgres, safe no-op for SQLite)
// --------------------------------------------------------------------

func tryAcquireLock(ctx context.Context, s *Store) (func(), error) {
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, err
	}

	switch s.db.Dialector.Name() {
	case "postgres":
		var got bool
		// Pick any unique int64 identifier for your app
		if err := sqlDB.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", 91423001).Scan(&got); err != nil {
			return nil, err
		}
		if !got {
			return nil, errors.New("another maintenance run is in progress")
		}
		return func() {
			_, _ = sqlDB.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", 91423001)
		}, nil
	default:
		// No locking available in SQLite
		return nil, nil
	}
}

// --------------------------------------------------------------------
// Maintenance tasks
// --------------------------------------------------------------------

// deleteInvalidAPITokens removes tokens that are explicitly disabled
// or past their expiration date.
func deleteInvalidAPITokens(ctx context.Context, s *Store) error {
	return s.db.WithContext(ctx).
		Exec(`DELETE FROM api_tokens WHERE disabled = TRUE OR (expires_at IS NOT NULL AND expires_at < NOW())`).
		Error
}

// deleteExpiredSignupTokens removes signup tokens that are already expired
// or have been consumed.
func deleteExpiredSignupTokens(ctx context.Context, s *Store) error {
	return s.db.WithContext(ctx).
		Exec(`DELETE FROM signup_tokens WHERE expires_at < NOW() OR consumed_at IS NOT NULL`).
		Error
}

// pruneOldRecentViews removes entries older than given duration.
func pruneOldRecentViews(ctx context.Context, s *Store, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return s.db.WithContext(ctx).
		Exec(`DELETE FROM recent_views WHERE viewed_at < ?`, cutoff).
		Error
}

// vacuumAnalyze runs database cleanup commands depending on DB engine.
func vacuumAnalyze(ctx context.Context, s *Store) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	switch s.db.Dialector.Name() {
	case "postgres":
		_, err = sqlDB.ExecContext(ctx, "VACUUM (ANALYZE)")
	case "sqlite":
		_, err = sqlDB.ExecContext(ctx, "VACUUM")
		if err == nil {
			_, _ = sqlDB.ExecContext(ctx, "PRAGMA optimize")
		}
	}
	return err
}

// pruneTempFiles deletes files older than given duration in a directory.
// Does nothing if dir is empty or does not exist.
func pruneTempFiles(dir string, olderThan time.Duration) error {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-olderThan)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.ModTime().Before(cutoff) {
			_ = os.Remove(p)
		}
	}
	return nil
}
