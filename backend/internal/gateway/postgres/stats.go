package postgres

import (
	"context"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/metrics"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

func WriteStats(ctx context.Context, db *pgxpool.Pool, w io.Writer) error {
	var registered, withBot, withNotifications, withRequests, overdue uint64
	err := db.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM autoservices),
		(SELECT count(*) FROM bots),
		(SELECT count(DISTINCT autoservice_id) FROM notify_recipients),
		(SELECT count(DISTINCT autoservice_id) FROM requests),
		(SELECT count(*) FROM requests WHERE status = 'new' AND created_at < now() - interval '24 hours')`).
		Scan(&registered, &withBot, &withNotifications, &withRequests, &overdue)
	if err != nil {
		return err
	}
	metrics.WriteGaugeUint64(w, `autoservices{stage="registered"}`, registered)
	metrics.WriteGaugeUint64(w, `autoservices{stage="bot_connected"}`, withBot)
	metrics.WriteGaugeUint64(w, `autoservices{stage="notifications_on"}`, withNotifications)
	metrics.WriteGaugeUint64(w, `autoservices{stage="has_requests"}`, withRequests)
	metrics.WriteGaugeUint64(w, "requests_overdue", overdue)

	byStatus := map[domain.RequestStatus]uint64{
		domain.StatusNew: 0, domain.StatusInProgress: 0, domain.StatusClosed: 0, domain.StatusRejected: 0, domain.StatusCancelled: 0,
	}
	rows, err := db.Query(ctx, `SELECT status, count(*) FROM requests GROUP BY status`)
	if err != nil {
		return err
	}
	var status domain.RequestStatus
	var n uint64
	if _, err := pgx.ForEachRow(rows, []any{&status, &n}, func() error {
		byStatus[status] = n
		return nil
	}); err != nil {
		return err
	}
	for status, n := range byStatus {
		metrics.WriteGaugeUint64(w, fmt.Sprintf(`requests_by_status{status=%q}`, status), n)
	}

	rows, err = db.Query(ctx, `SELECT status, status_error, count(*) FROM bots GROUP BY status, status_error`)
	if err != nil {
		return err
	}
	var botStatus, reason string
	_, err = pgx.ForEachRow(rows, []any{&botStatus, &reason, &n}, func() error {
		metrics.WriteGaugeUint64(w, fmt.Sprintf(`bots_by_status{status=%q,reason=%q}`, botStatus, reason), n)
		return nil
	})
	return err
}
