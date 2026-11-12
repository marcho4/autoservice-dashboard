package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const botRunnerLockKey = 7_311_202_602

// LeaderLock elects a single bot runner among all replicas with a session-level advisory lock:
// Telegram allows only one getUpdates consumer per bot, so exactly one process may poll.
type LeaderLock struct {
	db       *pgxpool.Pool
	interval time.Duration
}

func NewLeaderLock(db *pgxpool.Pool) *LeaderLock {
	return &LeaderLock{db: db, interval: 3 * time.Second}
}

// Acquire blocks until this process is the leader. The returned channel is closed when leadership is lost
// (the connection holding the lock died); release gives the lock up voluntarily.
func (l *LeaderLock) Acquire(ctx context.Context) (lost <-chan struct{}, release func(), err error) {
	for {
		conn, err := l.db.Acquire(ctx)
		if err == nil {
			var locked bool
			err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, botRunnerLockKey).Scan(&locked)
			if err == nil && locked {
				lost, release := l.hold(conn)
				return lost, release, nil
			}
			conn.Release()
		}
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(l.interval):
		}
	}
}

func (l *LeaderLock) hold(conn *pgxpool.Conn) (<-chan struct{}, func()) {
	lost := make(chan struct{})
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(l.interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), l.interval)
				err := conn.Ping(ctx)
				cancel()
				if err != nil {
					close(lost)
					return
				}
			}
		}
	}()
	release := func() {
		close(stop)
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), l.interval)
		defer cancel()
		if _, err := conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, botRunnerLockKey); err != nil {
			// A broken connection must not go back to the pool still holding the lock.
			_ = conn.Conn().Close(ctx)
		}
		conn.Release()
	}
	return lost, release
}
