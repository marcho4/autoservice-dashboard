package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type ServiceRepo struct{ db *pgxpool.Pool }

func NewServiceRepo(db *pgxpool.Pool) *ServiceRepo { return &ServiceRepo{db: db} }

const serviceColumns = `id, autoservice_id, name, description, is_active, position, fields, created_at, updated_at`

func scanService(row pgx.Row) (domain.Service, error) {
	var s domain.Service
	var fields []byte
	err := row.Scan(&s.ID, &s.AutoserviceID, &s.Name, &s.Description, &s.Active, &s.Position, &fields, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return s, notFound(err)
	}
	return s, json.Unmarshal(fields, &s.Fields)
}

func fieldsJSON(fields []domain.FormField) []byte {
	if fields == nil {
		fields = []domain.FormField{}
	}
	b, _ := json.Marshal(fields)
	return b
}

func insertServices(ctx context.Context, tx pgx.Tx, autoserviceID uuid.UUID, services []domain.ServiceInput) error {
	for _, s := range services {
		if _, err := tx.Exec(ctx,
			`INSERT INTO services (autoservice_id, name, description, is_active, fields, position)
			 VALUES ($1, $2, $3, $4, $5::jsonb,
				(SELECT COALESCE(max(position), 0) + 1 FROM services WHERE autoservice_id = $1))`,
			autoserviceID, s.Name, s.Description, s.Active, string(fieldsJSON(s.Fields))); err != nil {
			return err
		}
	}
	return nil
}

func (r *ServiceRepo) List(ctx context.Context, autoserviceID uuid.UUID, onlyActive bool) ([]domain.Service, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+serviceColumns+` FROM services
		 WHERE autoservice_id = $1 AND (is_active OR NOT $2)
		 ORDER BY position, created_at, id`, autoserviceID, onlyActive)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Service, error) { return scanService(row) })
}

func (r *ServiceRepo) Get(ctx context.Context, autoserviceID, id uuid.UUID) (domain.Service, error) {
	return scanService(r.db.QueryRow(ctx,
		`SELECT `+serviceColumns+` FROM services WHERE autoservice_id = $1 AND id = $2`, autoserviceID, id))
}

func (r *ServiceRepo) Create(ctx context.Context, autoserviceID uuid.UUID, in domain.ServiceInput) (domain.Service, error) {
	out, err := scanService(r.db.QueryRow(ctx,
		`INSERT INTO services (autoservice_id, name, description, is_active, fields, position)
		 SELECT $1::uuid, $2::text, $3::text, $4::boolean, $5::jsonb, COALESCE(max(position), 0) + 1
		 FROM services WHERE autoservice_id = $1::uuid
		 HAVING count(*) < $6::int
		 RETURNING `+serviceColumns,
		autoserviceID, in.Name, in.Description, in.Active, string(fieldsJSON(in.Fields)), domain.MaxServices))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Service{}, domain.ErrServiceLimit
	}
	return out, err
}

func (r *ServiceRepo) CreateDefaults(ctx context.Context, autoserviceID uuid.UUID, services []domain.ServiceInput) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM services WHERE autoservice_id = a.id)
			 FROM autoservices a WHERE a.id = $1 FOR UPDATE`, autoserviceID).Scan(&exists); err != nil {
			return notFound(err)
		}
		if exists {
			return domain.ErrServicesExist
		}
		return insertServices(ctx, tx, autoserviceID, services)
	})
}

func (r *ServiceRepo) Update(ctx context.Context, autoserviceID, id uuid.UUID, in domain.ServiceInput) (domain.Service, error) {
	return scanService(r.db.QueryRow(ctx,
		`UPDATE services SET name = $3, description = $4, is_active = $5, fields = $6::jsonb, updated_at = now()
		 WHERE autoservice_id = $1 AND id = $2 RETURNING `+serviceColumns,
		autoserviceID, id, in.Name, in.Description, in.Active, string(fieldsJSON(in.Fields))))
}

func (r *ServiceRepo) Delete(ctx context.Context, autoserviceID, id uuid.UUID) error {
	return affectedOrNotFound(r.db.Exec(ctx, `DELETE FROM services WHERE autoservice_id = $1 AND id = $2`, autoserviceID, id))
}

func (r *ServiceRepo) Reorder(ctx context.Context, autoserviceID uuid.UUID, ids []uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`WITH ranked AS (
			SELECT s.id, row_number() OVER (
				ORDER BY array_position($2::uuid[], s.id) NULLS LAST, s.position, s.created_at, s.id) AS n
			FROM services s WHERE s.autoservice_id = $1
		 )
		 UPDATE services s SET position = ranked.n FROM ranked WHERE ranked.id = s.id AND s.position <> ranked.n`,
		autoserviceID, ids)
	return err
}
