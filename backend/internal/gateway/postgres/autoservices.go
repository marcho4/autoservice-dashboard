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

// Create inserts the autoservice together with its starter services in one transaction.
func (r *AutoserviceRepo) Create(ctx context.Context, a domain.Autoservice, services []domain.ServiceInput) (domain.Autoservice, error) {
	var out domain.Autoservice
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		out, err = scanAutoservice(tx.QueryRow(ctx,
			`INSERT INTO autoservices (owner_id, name, address, phone, description)
			 VALUES ($1, $2, $3, $4, $5) RETURNING `+autoserviceColumns,
			a.OwnerID, a.Name, a.Address, a.Phone, a.Description))
		if err != nil {
			return err
		}
		return insertServices(ctx, tx, out.ID, services)
	})
	if uniqueConstraint(err) == "autoservices_owner_id_key" {
		return domain.Autoservice{}, domain.ErrAutoserviceExists
	}
	return out, err
}

func (r *AutoserviceRepo) Get(ctx context.Context, id uuid.UUID) (domain.Autoservice, error) {
	return scanAutoservice(r.db.QueryRow(ctx, `SELECT `+autoserviceColumns+` FROM autoservices WHERE id = $1`, id))
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

const botColumns = `autoservice_id, telegram_bot_id, username, token, status, status_error, status_at, link_code, created_at, updated_at`

func scanBot(row pgx.Row) (domain.Bot, error) {
	var b domain.Bot
	err := row.Scan(&b.AutoserviceID, &b.TelegramBotID, &b.Username, &b.Token,
		&b.Status, &b.StatusError, &b.StatusAt, &b.LinkCode, &b.CreatedAt, &b.UpdatedAt)
	return b, notFound(err)
}

// Upsert connects a bot or replaces its token. The runner status is reset to pending;
// when a different Telegram bot is connected, masters have to subscribe again because they never started the new bot.
func (r *BotRepo) Upsert(ctx context.Context, b domain.Bot) (domain.Bot, error) {
	var out domain.Bot
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM notify_recipients n USING bots b
			 WHERE n.autoservice_id = $1 AND b.autoservice_id = n.autoservice_id AND b.telegram_bot_id <> $2`,
			b.AutoserviceID, b.TelegramBotID); err != nil {
			return err
		}
		var err error
		out, err = scanBot(tx.QueryRow(ctx,
			`INSERT INTO bots (autoservice_id, telegram_bot_id, username, token, link_code)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (autoservice_id) DO UPDATE SET
				telegram_bot_id = EXCLUDED.telegram_bot_id,
				username = EXCLUDED.username,
				token = EXCLUDED.token,
				status = 'pending',
				status_error = '',
				status_at = now(),
				updated_at = now()
			 RETURNING `+botColumns,
			b.AutoserviceID, b.TelegramBotID, b.Username, b.Token, b.LinkCode))
		return err
	})
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

// SetStatus records what the runner sees. The token guard keeps a stale poller from overwriting the status of a replaced token.
func (r *BotRepo) SetStatus(ctx context.Context, autoserviceID uuid.UUID, token string, status domain.BotStatus, reason string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE bots SET status = $3, status_error = $4, status_at = now()
		 WHERE autoservice_id = $1 AND token = $2 AND (status <> $3 OR status_error <> $4)`,
		autoserviceID, token, status, reason)
	return err
}

// AddRecipient subscribes a chat if the code matches the current link code, and rotates the code so the link works once.
func (r *BotRepo) AddRecipient(ctx context.Context, autoserviceID uuid.UUID, code, newCode string, chatID int64, name string) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := affectedOrNotFound(tx.Exec(ctx,
			`UPDATE bots SET link_code = $3 WHERE autoservice_id = $1 AND link_code = $2`,
			autoserviceID, code, newCode)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO notify_recipients (autoservice_id, telegram_chat_id, name) VALUES ($1, $2, $3)
			 ON CONFLICT (autoservice_id, telegram_chat_id) DO UPDATE SET name = EXCLUDED.name`,
			autoserviceID, chatID, name)
		return err
	})
}

func (r *BotRepo) ListRecipients(ctx context.Context, autoserviceID uuid.UUID) ([]domain.NotifyRecipient, error) {
	rows, err := r.db.Query(ctx,
		`SELECT autoservice_id, telegram_chat_id, name, created_at FROM notify_recipients
		 WHERE autoservice_id = $1 ORDER BY created_at, telegram_chat_id`, autoserviceID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.NotifyRecipient, error) {
		var n domain.NotifyRecipient
		err := row.Scan(&n.AutoserviceID, &n.ChatID, &n.Name, &n.CreatedAt)
		return n, err
	})
}

func (r *BotRepo) RemoveRecipient(ctx context.Context, autoserviceID uuid.UUID, chatID int64) error {
	return affectedOrNotFound(r.db.Exec(ctx,
		`DELETE FROM notify_recipients WHERE autoservice_id = $1 AND telegram_chat_id = $2`, autoserviceID, chatID))
}
