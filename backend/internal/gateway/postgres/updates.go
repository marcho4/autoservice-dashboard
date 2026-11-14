package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/telegram"
)

type UpdateRepo struct{ db *pgxpool.Pool }

func NewUpdateRepo(db *pgxpool.Pool) *UpdateRepo { return &UpdateRepo{db: db} }

func (r *UpdateRepo) Save(ctx context.Context, bot domain.Bot, updates []telegram.Update) error {
	if len(updates) == 0 {
		return nil
	}
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var currentID int64
		if err := tx.QueryRow(ctx, `SELECT telegram_bot_id FROM bots WHERE autoservice_id = $1 FOR SHARE`, bot.AutoserviceID).Scan(&currentID); err != nil {
			return err
		}
		if currentID != bot.TelegramBotID {
			return domain.ErrBotNotConnected
		}
		for _, update := range updates {
			raw, err := json.Marshal(update)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO bot_updates (autoservice_id, telegram_bot_id, update_id, payload) VALUES ($1, $2, $3, $4::jsonb)
				 ON CONFLICT DO NOTHING`, bot.AutoserviceID, bot.TelegramBotID, update.UpdateID, string(raw)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *UpdateRepo) Pending(ctx context.Context, bot domain.Bot) ([]telegram.Update, error) {
	rows, err := r.db.Query(ctx,
		`SELECT payload FROM bot_updates WHERE autoservice_id = $1 AND telegram_bot_id = $2 ORDER BY update_id LIMIT 100`,
		bot.AutoserviceID, bot.TelegramBotID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (telegram.Update, error) {
		var raw []byte
		var update telegram.Update
		if err := row.Scan(&raw); err != nil {
			return update, err
		}
		err := json.Unmarshal(raw, &update)
		return update, err
	})
}

func (r *UpdateRepo) Delete(ctx context.Context, bot domain.Bot, updateID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM bot_updates WHERE autoservice_id = $1 AND telegram_bot_id = $2 AND update_id = $3`,
		bot.AutoserviceID, bot.TelegramBotID, updateID)
	return err
}
