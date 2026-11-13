package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type ClientRepo struct{ db *pgxpool.Pool }

func NewClientRepo(db *pgxpool.Pool) *ClientRepo { return &ClientRepo{db: db} }

const clientColumns = `c.id, c.autoservice_id, c.telegram_id, c.telegram_username, c.name, c.phone, c.created_at, c.updated_at`

func clientDest(c *domain.Client) []any {
	return []any{&c.ID, &c.AutoserviceID, &c.TelegramID, &c.TelegramUsername, &c.Name, &c.Phone, &c.CreatedAt, &c.UpdatedAt}
}

func scanClient(row pgx.Row) (domain.Client, error) {
	var c domain.Client
	err := row.Scan(clientDest(&c)...)
	return c, notFound(err)
}

func (r *ClientRepo) Upsert(ctx context.Context, autoserviceID uuid.UUID, who domain.ClientIdentity, phone string) (domain.Client, error) {
	return scanClient(r.db.QueryRow(ctx,
		`INSERT INTO clients AS c (autoservice_id, telegram_id, telegram_username, name, phone)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (autoservice_id, telegram_id) DO UPDATE SET
			telegram_username = EXCLUDED.telegram_username,
			name = CASE WHEN EXCLUDED.name <> '' THEN EXCLUDED.name ELSE c.name END,
			phone = EXCLUDED.phone,
			updated_at = now()
		 RETURNING `+clientColumns,
		autoserviceID, who.TelegramID, who.TelegramUsername, who.Name, phone))
}

func (r *ClientRepo) Get(ctx context.Context, autoserviceID, id uuid.UUID) (domain.Client, error) {
	return scanClient(r.db.QueryRow(ctx,
		`SELECT `+clientColumns+` FROM clients c WHERE c.autoservice_id = $1 AND c.id = $2`, autoserviceID, id))
}

func (r *ClientRepo) GetByTelegramID(ctx context.Context, autoserviceID uuid.UUID, telegramID int64) (domain.Client, error) {
	return scanClient(r.db.QueryRow(ctx,
		`SELECT `+clientColumns+` FROM clients c WHERE c.autoservice_id = $1 AND c.telegram_id = $2`, autoserviceID, telegramID))
}

func (r *ClientRepo) List(ctx context.Context, autoserviceID uuid.UUID) ([]domain.ClientSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+clientColumns+`, count(r.id), max(r.created_at)
		 FROM clients c LEFT JOIN requests r ON r.client_id = c.id
		 WHERE c.autoservice_id = $1
		 GROUP BY c.id
		 ORDER BY max(r.created_at) DESC NULLS LAST, c.created_at DESC`, autoserviceID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ClientSummary, error) {
		var s domain.ClientSummary
		err := row.Scan(append(clientDest(&s.Client), &s.RequestsCount, &s.LastRequestAt)...)
		return s, err
	})
}

type RequestRepo struct{ db *pgxpool.Pool }

func NewRequestRepo(db *pgxpool.Pool) *RequestRepo { return &RequestRepo{db: db} }

const requestColumns = `r.id, r.autoservice_id, r.client_id, r.number, r.status, r.service_id, r.service_name, r.answers, r.message, r.phone, r.note, r.created_at, r.updated_at`

const requestFilterCondition = `r.autoservice_id = $1 AND ($2::text IS NULL OR r.status = $2)`

// answersJSON scans the jsonb answers column into the request.
type answersJSON struct{ dst *[]domain.Answer }

func (a answersJSON) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return errors.New("answers: unsupported column type")
	}
	return json.Unmarshal(raw, a.dst)
}

func marshalAnswers(answers []domain.Answer) string {
	if answers == nil {
		answers = []domain.Answer{}
	}
	b, _ := json.Marshal(answers)
	return string(b)
}

func requestDest(r *domain.Request) []any {
	return []any{&r.ID, &r.AutoserviceID, &r.ClientID, &r.Number, &r.Status, &r.ServiceID, &r.ServiceName,
		answersJSON{&r.Answers}, &r.Message, &r.Phone, &r.Note, &r.CreatedAt, &r.UpdatedAt}
}

func scanRequest(row pgx.Row) (domain.Request, error) {
	var r domain.Request
	err := row.Scan(requestDest(&r)...)
	return r, notFound(err)
}

func scanRequestWithClient(row pgx.Row) (domain.RequestWithClient, error) {
	var r domain.RequestWithClient
	err := row.Scan(append(requestDest(&r.Request), clientDest(&r.Client)...)...)
	return r, notFound(err)
}

func orderBy(s domain.SortOrder) string {
	if s == domain.SortAsc {
		return "r.created_at ASC, r.id ASC"
	}
	return "r.created_at DESC, r.id DESC"
}

// Create stores the request under req.ID and the next per-autoservice number.
// A service that was deleted while the client was filling the form is stored by name only.
// The number is taken in the same statement as the insert, so a failed insert (e.g. a taken ID) does not use it up.
func (r *RequestRepo) Create(ctx context.Context, req domain.Request) (domain.Request, error) {
	return scanRequest(r.db.QueryRow(ctx,
		`WITH seq AS (
			UPDATE autoservices SET request_seq = request_seq + 1 WHERE id = $1::uuid RETURNING request_seq
		 )
		 INSERT INTO requests AS r (id, autoservice_id, client_id, status, number, service_id, service_name, answers, message, phone)
		 SELECT $9::uuid, $1::uuid, $2::uuid, $3::text, seq.request_seq,
			(SELECT s.id FROM services s WHERE s.id = $4::uuid AND s.autoservice_id = $1::uuid),
			$5::text, $6::jsonb, $7::text, $8::text
		 FROM seq
		 RETURNING `+requestColumns,
		req.AutoserviceID, req.ClientID, req.Status, req.ServiceID, req.ServiceName,
		marshalAnswers(req.Answers), req.Message, req.Phone, req.ID))
}

func (r *RequestRepo) Get(ctx context.Context, autoserviceID, id uuid.UUID) (domain.RequestWithClient, error) {
	return scanRequestWithClient(r.db.QueryRow(ctx,
		`SELECT `+requestColumns+`, `+clientColumns+`
		 FROM requests r JOIN clients c ON c.id = r.client_id
		 WHERE r.autoservice_id = $1 AND r.id = $2`, autoserviceID, id))
}

func (r *RequestRepo) List(ctx context.Context, autoserviceID uuid.UUID, f domain.RequestFilter) ([]domain.RequestWithClient, int, error) {
	var total int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM requests r WHERE `+requestFilterCondition,
		autoserviceID, f.Status).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+requestColumns+`, `+clientColumns+`
		 FROM requests r JOIN clients c ON c.id = r.client_id
		 WHERE `+requestFilterCondition+`
		 ORDER BY `+orderBy(f.Sort)+` LIMIT $3 OFFSET $4`,
		autoserviceID, f.Status, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.RequestWithClient, error) {
		return scanRequestWithClient(row)
	})
	return items, total, err
}

func (r *RequestRepo) Counts(ctx context.Context, autoserviceID uuid.UUID) (domain.RequestCounts, error) {
	rows, err := r.db.Query(ctx,
		`SELECT status, count(*) FROM requests WHERE autoservice_id = $1 GROUP BY status`, autoserviceID)
	if err != nil {
		return nil, err
	}
	counts := domain.RequestCounts{}
	var status domain.RequestStatus
	var n int
	_, err = pgx.ForEachRow(rows, []any{&status, &n}, func() error {
		counts[status] = n
		return nil
	})
	return counts, err
}

func (r *RequestRepo) CountByClientSince(ctx context.Context, autoserviceID, clientID uuid.UUID, since time.Time) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM requests WHERE autoservice_id = $1 AND client_id = $2 AND created_at > $3`,
		autoserviceID, clientID, since).Scan(&n)
	return n, err
}

func (r *RequestRepo) ListByClient(ctx context.Context, autoserviceID, clientID uuid.UUID) ([]domain.Request, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+requestColumns+` FROM requests r
		 WHERE r.autoservice_id = $1 AND r.client_id = $2
		 ORDER BY `+orderBy(domain.SortDesc), autoserviceID, clientID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Request, error) { return scanRequest(row) })
}

func (r *RequestRepo) UpdateStatus(ctx context.Context, autoserviceID, id uuid.UUID, from, to domain.RequestStatus) (domain.Request, error) {
	out, err := scanRequest(r.db.QueryRow(ctx,
		`UPDATE requests r SET status = $4, updated_at = now()
		 WHERE r.autoservice_id = $1 AND r.id = $2 AND r.status = $3
		 RETURNING `+requestColumns, autoserviceID, id, from, to))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Request{}, domain.ErrInvalidTransition
	}
	return out, err
}

func (r *RequestRepo) UpdateContent(ctx context.Context, autoserviceID, clientID, id uuid.UUID, in domain.RequestInput) (domain.Request, error) {
	out, err := scanRequest(r.db.QueryRow(ctx,
		`UPDATE requests r SET
			service_id = (SELECT s.id FROM services s WHERE s.id = $5::uuid AND s.autoservice_id = $1),
			service_name = $6, answers = $7::jsonb, message = $8, phone = $9, updated_at = now()
		 WHERE r.autoservice_id = $1 AND r.client_id = $2 AND r.id = $3 AND r.status = $4
		 RETURNING `+requestColumns,
		autoserviceID, clientID, id, domain.StatusNew, in.ServiceID, in.ServiceName,
		marshalAnswers(in.Answers), in.Message, in.Phone))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Request{}, domain.ErrRequestNotEditable
	}
	return out, err
}

func (r *RequestRepo) UpdateNote(ctx context.Context, autoserviceID, id uuid.UUID, note string) (domain.Request, error) {
	return scanRequest(r.db.QueryRow(ctx,
		`UPDATE requests r SET note = $3, updated_at = now()
		 WHERE r.autoservice_id = $1 AND r.id = $2 RETURNING `+requestColumns, autoserviceID, id, note))
}
