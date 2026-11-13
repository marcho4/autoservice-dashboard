package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConversationRepo keeps the bot dialog state of each client chat as an opaque JSON document.
type ConversationRepo struct{ db *pgxpool.Pool }

func NewConversationRepo(db *pgxpool.Pool) *ConversationRepo { return &ConversationRepo{db: db} }

// conversationTTL: a client who comes back after a day starts a new dialog instead of answering a forgotten question.
const conversationTTL = `interval '24 hours'`

// Load returns nil when the chat has no saved state or it has expired.
func (r *ConversationRepo) Load(ctx context.Context, autoserviceID uuid.UUID, chatID int64) ([]byte, error) {
	var state []byte
	err := r.db.QueryRow(ctx,
		`SELECT state FROM conversations
		 WHERE autoservice_id = $1 AND telegram_chat_id = $2 AND updated_at > now() - `+conversationTTL,
		autoserviceID, chatID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return state, err
}

func (r *ConversationRepo) Save(ctx context.Context, autoserviceID uuid.UUID, chatID int64, state []byte) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO conversations (autoservice_id, telegram_chat_id, state) VALUES ($1, $2, $3::jsonb)
		 ON CONFLICT (autoservice_id, telegram_chat_id) DO UPDATE SET state = EXCLUDED.state, updated_at = now()`,
		autoserviceID, chatID, string(state))
	return err
}

// DeleteExpired removes the states Load no longer returns, so client data of abandoned dialogs is not kept.
func (r *ConversationRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM conversations WHERE updated_at < now() - `+conversationTTL)
	return err
}
