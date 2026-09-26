package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
)

type AutoserviceRepo struct{ db *pgxpool.Pool }

func NewAutoserviceRepo(db *pgxpool.Pool) *AutoserviceRepo { return &AutoserviceRepo{db: db} }

const autoserviceColumns = `id, owner_id, name, address, phone, description, created_at, updated_at`

func scanAutoservice(row pgx.Row) (domain.Autoservice, error) {
	var a domain.Autoservice
	err := row.Scan(&a.ID, &a.OwnerID, &a.Name, &a.Address, &a.Phone, &a.Description, &a.CreatedAt, &a.UpdatedAt)
	return a, notFound(err)
}

func (r *AutoserviceRepo) Create(ctx context.Context, a domain.Autoservice) (domain.Autoservice, error) {
	out, err := scanAutoservice(r.db.QueryRow(ctx,
		`INSERT INTO autoservices (owner_id, name, address, phone, description)
		 VALUES ($1, $2, $3, $4, $5) RETURNING `+autoserviceColumns,
		a.OwnerID, a.Name, a.Address, a.Phone, a.Description))
	if uniqueConstraint(err) == "autoservices_owner_id_key" {
		return domain.Autoservice{}, domain.ErrAutoserviceExists
	}
	return out, err
}

func (r *AutoserviceRepo) GetByOwner(ctx context.Context, ownerID uuid.UUID) (domain.Autoservice, error) {
	return scanAutoservice(r.db.QueryRow(ctx, `SELECT `+autoserviceColumns+` FROM autoservices WHERE owner_id = $1`, ownerID))
}

func (r *AutoserviceRepo) Update(ctx context.Context, id uuid.UUID, in domain.AutoserviceInput) (domain.Autoservice, error) {
	return scanAutoservice(r.db.QueryRow(ctx,
		`UPDATE autoservices SET name = $2, address = $3, phone = $4, description = $5, updated_at = now()
		 WHERE id = $1 RETURNING `+autoserviceColumns,
		id, in.Name, in.Address, in.Phone, in.Description))
}

func (r *AutoserviceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return affectedOrNotFound(r.db.Exec(ctx, `DELETE FROM autoservices WHERE id = $1`, id))
}

type BotRepo struct{ db *pgxpool.Pool }

func NewBotRepo(db *pgxpool.Pool) *BotRepo { return &BotRepo{db: db} }

const botColumns = `autoservice_id, telegram_bot_id, username, token, created_at, updated_at`

func scanBot(row pgx.Row) (domain.Bot, error) {
	var b domain.Bot
	err := row.Scan(&b.AutoserviceID, &b.TelegramBotID, &b.Username, &b.Token, &b.CreatedAt, &b.UpdatedAt)
	return b, notFound(err)
}

func (r *BotRepo) Upsert(ctx context.Context, b domain.Bot) (domain.Bot, error) {
	out, err := scanBot(r.db.QueryRow(ctx,
		`INSERT INTO bots (autoservice_id, telegram_bot_id, username, token)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (autoservice_id) DO UPDATE SET
			telegram_bot_id = EXCLUDED.telegram_bot_id,
			username = EXCLUDED.username,
			token = EXCLUDED.token,
			updated_at = now()
		 RETURNING `+botColumns,
		b.AutoserviceID, b.TelegramBotID, b.Username, b.Token))
	if uniqueConstraint(err) == "bots_telegram_bot_id_key" {
		return domain.Bot{}, domain.ErrBotAlreadyLinked
	}
	return out, err
}

func (r *BotRepo) GetByAutoservice(ctx context.Context, autoserviceID uuid.UUID) (domain.Bot, error) {
	return scanBot(r.db.QueryRow(ctx, `SELECT `+botColumns+` FROM bots WHERE autoservice_id = $1`, autoserviceID))
}

func (r *BotRepo) GetByTelegramID(ctx context.Context, telegramBotID int64) (domain.Bot, error) {
	return scanBot(r.db.QueryRow(ctx, `SELECT `+botColumns+` FROM bots WHERE telegram_bot_id = $1`, telegramBotID))
}

func (r *BotRepo) List(ctx context.Context) ([]domain.Bot, error) {
	rows, err := r.db.Query(ctx, `SELECT `+botColumns+` FROM bots ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Bot, error) { return scanBot(row) })
}

func (r *BotRepo) Delete(ctx context.Context, autoserviceID uuid.UUID) error {
	return affectedOrNotFound(r.db.Exec(ctx, `DELETE FROM bots WHERE autoservice_id = $1`, autoserviceID))
}
