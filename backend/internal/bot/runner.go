package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/google/uuid"

	"github.com/marcho4/autoservice-dashboard/backend/internal/domain"
	"github.com/marcho4/autoservice-dashboard/backend/internal/gateway/telegram"
)

type Telegram interface {
	Setup(ctx context.Context, token string, commands []telegram.BotCommand) error
	SetDescription(ctx context.Context, token, description, shortDescription string) error
	GetUpdates(ctx context.Context, token string, offset int64, timeout time.Duration) ([]telegram.Update, error)
	SendMessage(ctx context.Context, token string, m telegram.OutMessage) error
	AnswerCallbackQuery(ctx context.Context, token, id, text string) error
	EditMessageText(ctx context.Context, token string, chatID, messageID int64, text string) error
	ClearInlineKeyboard(ctx context.Context, token string, chatID, messageID int64) error
	SendTyping(ctx context.Context, token string, chatID int64) error
}

type Registry interface {
	ListBots(ctx context.Context) ([]domain.Bot, error)
	Autoservice(ctx context.Context, autoserviceID uuid.UUID) (domain.Autoservice, error)
	SetBotStatus(ctx context.Context, bot domain.Bot, status domain.BotStatus, reason string) error
	RemoveRecipient(ctx context.Context, autoserviceID uuid.UUID, chatID int64) error
}

type Leader interface {
	Acquire(ctx context.Context) (lost <-chan struct{}, release func(), err error)
}

type UpdateStore interface {
	Save(ctx context.Context, bot domain.Bot, updates []telegram.Update) error
	Pending(ctx context.Context, bot domain.Bot) ([]telegram.Update, error)
	Delete(ctx context.Context, bot domain.Bot, updateID int64) error
}

type RunnerConfig struct {
	ReconcileInterval time.Duration
	PollTimeout       time.Duration
	HandleTimeout     time.Duration
	HandleGrace       time.Duration
}

func (c RunnerConfig) withDefaults() RunnerConfig {
	if c.ReconcileInterval <= 0 {
		c.ReconcileInterval = 3 * time.Second
	}
	if c.PollTimeout <= 0 {
		c.PollTimeout = 25 * time.Second
	}
	if c.HandleTimeout <= 0 {
		c.HandleTimeout = time.Minute
	}
	if c.HandleGrace <= 0 {
		c.HandleGrace = 10 * time.Second
	}
	return c
}

type Runner struct {
	tg       Telegram
	registry Registry
	engine   *Engine
	updates  UpdateStore
	leader   Leader
	cfg      RunnerConfig
	log      *slog.Logger
}

func NewRunner(tg Telegram, registry Registry, engine *Engine, updates UpdateStore, leader Leader, cfg RunnerConfig, log *slog.Logger) *Runner {
	return &Runner{tg: tg, registry: registry, engine: engine, updates: updates, leader: leader, cfg: cfg.withDefaults(), log: log}
}

var commands = []telegram.BotCommand{
	{Command: "start", Description: "Оставить заявку"},
	{Command: "my", Description: "Мои заявки"},
	{Command: "cancel", Description: "Отменить заполнение заявки"},
}

func (r *Runner) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		lost, release, err := r.leader.Acquire(ctx)
		if err != nil {
			break
		}
		r.log.Info("bot runner is the leader, starting bots")
		leader := metrics.GetOrCreateGauge("bot_runner_leader", nil)
		leader.Set(1)
		leadCtx, cancel := context.WithCancel(ctx)
		go func() {
			select {
			case <-lost:
				r.log.Warn("bot runner lost leadership, stopping bots")
				cancel()
			case <-leadCtx.Done():
			}
		}()
		r.supervise(leadCtx)
		leader.Set(0)
		cancel()
		release()
	}
	r.log.Info("bot runner stopped")
	return nil
}

type botProc struct {
	token  string
	cancel context.CancelFunc
	done   chan struct{}
}

func (r *Runner) supervise(ctx context.Context) {
	running := map[uuid.UUID]*botProc{}
	stop := func(id uuid.UUID) {
		running[id].cancel()
		<-running[id].done
		delete(running, id)
	}
	defer func() {
		for _, proc := range running {
			proc.cancel()
		}
		for _, proc := range running {
			<-proc.done
		}
		metrics.GetOrCreateGauge("bot_runner_bots", nil).Set(0)
	}()

	ticker := time.NewTicker(r.cfg.ReconcileInterval)
	defer ticker.Stop()
	var cleaned time.Time
	for {
		if time.Since(cleaned) >= time.Hour {
			if err := r.engine.state.DeleteExpired(ctx); err != nil && ctx.Err() == nil {
				r.log.Error("bot runner: delete expired conversations", "error", err)
			}
			cleaned = time.Now()
		}
		bots, err := r.registry.ListBots(ctx)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			r.log.Error("bot runner: list bots", "error", err)
		default:
			wanted := map[uuid.UUID]bool{}
			for _, bot := range bots {
				wanted[bot.AutoserviceID] = true
				if proc, ok := running[bot.AutoserviceID]; ok {
					if proc.token == bot.Token {
						continue
					}
					stop(bot.AutoserviceID)
				}
				botCtx, cancel := context.WithCancel(ctx)
				proc := &botProc{token: bot.Token, cancel: cancel, done: make(chan struct{})}
				running[bot.AutoserviceID] = proc
				go func() {
					defer close(proc.done)
					r.poll(botCtx, bot)
				}()
			}
			for id := range running {
				if !wanted[id] {
					stop(id)
				}
			}
			metrics.GetOrCreateGauge("bot_runner_bots", nil).Set(float64(len(running)))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Runner) poll(ctx context.Context, bot domain.Bot) {
	log := r.log.With("bot", bot.Username, "autoservice_id", bot.AutoserviceID)
	log.Info("bot started")
	defer log.Info("bot stopped")

	chats := newChatQueue(log)
	handleCtx, stopHandlers := context.WithCancel(context.WithoutCancel(ctx))
	stopGrace := context.AfterFunc(ctx, func() {
		time.AfterFunc(r.cfg.HandleGrace, stopHandlers)
	})
	defer func() {
		chats.wait()
		stopGrace()
		stopHandlers()
	}()
	var queuedMu sync.Mutex
	queued := map[int64]bool{}
	dispatch := func(pending []telegram.Update) {
		blocked := map[int64]bool{}
		for _, u := range pending {
			chatID, ok := chatOf(u)
			if !ok {
				if err := r.updates.Delete(ctx, bot, u.UpdateID); err != nil {
					log.Error("delete ignored update", "error", err)
				}
				continue
			}
			if blocked[chatID] {
				continue
			}
			queuedMu.Lock()
			if queued[u.UpdateID] {
				queuedMu.Unlock()
				continue
			}
			queued[u.UpdateID] = true
			queuedMu.Unlock()
			if !chats.submit(chatID, func() {
				defer func() {
					queuedMu.Lock()
					delete(queued, u.UpdateID)
					queuedMu.Unlock()
				}()
				for ctx.Err() == nil {
					hctx, cancel := context.WithTimeout(handleCtx, r.cfg.HandleTimeout)
					err := r.handle(hctx, bot, u, log)
					if err == nil {
						err = r.updates.Delete(hctx, bot, u.UpdateID)
					}
					cancel()
					if err == nil {
						return
					}
					log.Warn("retry saved bot update", "update_id", u.UpdateID, "error", err)
					sleep(ctx, time.Second)
				}
			}) {
				queuedMu.Lock()
				delete(queued, u.UpdateID)
				queuedMu.Unlock()
				blocked[chatID] = true
			}
		}
	}

	ready := false
	var offset int64
	failures := 0
	reported := ""
	report := func(status domain.BotStatus, reason string) {
		if key := string(status) + ":" + reason; key != reported {
			r.setStatus(ctx, bot, status, reason)
			reported = key
		}
	}
	for ctx.Err() == nil {
		pending, err := r.updates.Pending(ctx, bot)
		if err != nil {
			log.Error("load saved bot updates", "error", err)
			sleep(ctx, time.Second)
			continue
		}
		dispatch(pending)
		var updates []telegram.Update
		pollTimeout := r.cfg.PollTimeout
		if len(pending) > 0 {
			pollTimeout = min(pollTimeout, time.Second)
		}
		if !ready {
			if err = r.tg.Setup(ctx, bot.Token, commands); err == nil {
				ready = true
				r.describe(ctx, bot, log)
				report(domain.BotRunning, "")
			}
		}
		if ready {
			updates, err = r.tg.GetUpdates(ctx, bot.Token, offset, pollTimeout)
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			failures++
			reason, pause := classifyPollError(err, failures)
			if reason != domain.BotErrUnavailable || failures >= 3 {
				report(domain.BotError, reason)
			}
			log.Warn("bot polling failed", "error", err, "reason", reason, "retry_in", pause.String())
			sleep(ctx, pause)
			continue
		}
		failures = 0
		report(domain.BotRunning, "")

		if err := r.updates.Save(ctx, bot, updates); err != nil {
			log.Error("save bot updates before acknowledgment", "error", err)
			sleep(ctx, time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
		}
	}
}

func (r *Runner) describe(ctx context.Context, bot domain.Bot, log *slog.Logger) {
	a, err := r.registry.Autoservice(ctx, bot.AutoserviceID)
	if err != nil {
		log.Warn("bot description: load autoservice", "error", err)
		return
	}
	description := fmt.Sprintf("Бот автосервиса «%s». Опишите, что нужно сделать с машиной, — я передам мастеру, и он вам перезвонит.", a.Name)
	var contacts []string
	if a.Address != "" {
		contacts = append(contacts, "Адрес: "+a.Address+".")
	}
	if a.Phone != "" {
		contacts = append(contacts, "Телефон: "+a.Phone+".")
	}
	if len(contacts) > 0 {
		description += "\n" + strings.Join(contacts, " ")
	}
	short := fmt.Sprintf("Заявки в автосервис «%s»", truncate(a.Name, 96))
	if err := r.tg.SetDescription(ctx, bot.Token, truncate(description, 511), short); err != nil && ctx.Err() == nil {
		log.Warn("bot description: set", "error", err)
	}
}

func classifyPollError(err error, failures int) (reason string, pause time.Duration) {
	switch telegram.ErrorCode(err) {
	case http.StatusUnauthorized, http.StatusNotFound:
		return domain.BotErrUnauthorized, time.Minute
	case http.StatusConflict:
		return domain.BotErrConflict, 10 * time.Second
	}
	return domain.BotErrUnavailable, min(time.Duration(failures)*2*time.Second, 30*time.Second)
}

func (r *Runner) setStatus(ctx context.Context, bot domain.Bot, status domain.BotStatus, reason string) {
	if err := r.registry.SetBotStatus(ctx, bot, status, reason); err != nil && ctx.Err() == nil {
		r.log.Error("bot runner: save bot status", "bot", bot.Username, "error", err)
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func chatOf(u telegram.Update) (int64, bool) {
	switch {
	case u.Message != nil && u.Message.From != nil && u.Message.Chat.Type == "private":
		return u.Message.Chat.ID, true
	case u.CallbackQuery != nil && u.CallbackQuery.Message != nil && u.CallbackQuery.Message.Chat.Type == "private":
		return u.CallbackQuery.Message.Chat.ID, true
	}
	return 0, false
}

func identity(u telegram.User) domain.ClientIdentity {
	return domain.ClientIdentity{TelegramID: u.ID, TelegramUsername: u.Username, Name: u.FullName()}
}

func (r *Runner) handle(ctx context.Context, bot domain.Bot, u telegram.Update, log *slog.Logger) (handleErr error) {
	defer func() {
		if p := recover(); p != nil {
			handleErr = fmt.Errorf("bot update panic: %v", p)
			metrics.GetOrCreateCounter("bot_update_errors_total").Inc()
			log.Error("bot update panic", "error", handleErr, "stack", string(debug.Stack()))
		}
	}()
	var in Incoming
	kind := "message"
	switch {
	case u.CallbackQuery != nil:
		kind = "callback"
		q := u.CallbackQuery
		in = Incoming{ChatID: q.Message.Chat.ID, From: identity(q.From), Callback: q.Data, MessageText: q.Message.Text}
	default:
		m := u.Message
		in = Incoming{ChatID: m.Chat.ID, From: identity(*m.From), Text: m.Text}
		if m.Contact != nil {
			in.Contact = m.Contact.PhoneNumber
		}
		go func() { _ = r.tg.SendTyping(ctx, bot.Token, in.ChatID) }()
	}
	metrics.GetOrCreateCounter(`bot_updates_total{kind="` + kind + `"}`).Inc()

	in.UpdateID, in.BotID = u.UpdateID, bot.TelegramBotID
	res, err := r.engine.Handle(ctx, bot.AutoserviceID, in)
	handleErr = err
	if err != nil {
		metrics.GetOrCreateCounter("bot_update_errors_total").Inc()
		log.Error("bot failed to handle update", "chat_id", in.ChatID, "error", err)
		res = Result{
			Toast:    "Что-то пошло не так, попробуйте ещё раз",
			Messages: []Outgoing{{ChatID: in.ChatID, Text: "Что-то пошло не так. Попробуйте, пожалуйста, ещё раз чуть позже."}},
		}
		if u.CallbackQuery != nil {
			res.Messages = nil
		}
	}

	if q := u.CallbackQuery; q != nil {
		if err := r.tg.AnswerCallbackQuery(ctx, bot.Token, q.ID, res.Toast); err != nil {
			log.Debug("answer callback query", "error", err)
		}
		switch {
		case res.EditText != "":
			err = r.tg.EditMessageText(ctx, bot.Token, in.ChatID, q.Message.MessageID, res.EditText)
		case res.ClearButtons:
			err = r.tg.ClearInlineKeyboard(ctx, bot.Token, in.ChatID, q.Message.MessageID)
		}
		if err != nil {
			log.Debug("edit message", "error", err)
		}
	}

	for _, m := range res.Messages {
		err := r.tg.SendMessage(ctx, bot.Token, toTelegram(m))
		if m.ChatID != in.ChatID {
			metrics.GetOrCreateCounter(fmt.Sprintf(`bot_notifications_total{delivered="%t"}`, err == nil)).Inc()
		}
		if err == nil {
			continue
		}
		log.Warn("bot failed to send message", "chat_id", m.ChatID, "error", err)
		if m.ChatID != in.ChatID && telegram.ErrorCode(err) == http.StatusForbidden {
			if err := r.registry.RemoveRecipient(ctx, bot.AutoserviceID, m.ChatID); err != nil {
				log.Debug("remove unreachable recipient", "error", err)
			}
		}
	}
	return handleErr
}

func toTelegram(m Outgoing) telegram.OutMessage {
	out := telegram.OutMessage{ChatID: m.ChatID, Text: m.Text, RemoveKeyboard: m.RemoveKeyboard}
	for _, row := range m.Buttons {
		var buttons []telegram.InlineButton
		for _, b := range row {
			buttons = append(buttons, telegram.InlineButton{Text: b.Text, CallbackData: b.Data, URL: b.URL})
		}
		out.Inline = append(out.Inline, buttons)
	}
	for _, row := range m.Keyboard {
		var buttons []telegram.KeyboardButton
		for _, b := range row {
			buttons = append(buttons, telegram.KeyboardButton{Text: b.Text, RequestContact: b.RequestContact})
		}
		out.Keyboard = append(out.Keyboard, buttons)
	}
	return out
}

const (
	maxQueuedPerChat    = 20
	maxParallelHandlers = 16
)

type chatQueue struct {
	mu      sync.Mutex
	pending map[int64][]func()
	wg      sync.WaitGroup
	slots   chan struct{}
	log     *slog.Logger
}

func newChatQueue(log *slog.Logger) *chatQueue {
	return &chatQueue{pending: map[int64][]func(){}, slots: make(chan struct{}, maxParallelHandlers), log: log}
}

func (q *chatQueue) submit(chatID int64, fn func()) bool {
	q.mu.Lock()
	queue, busy := q.pending[chatID]
	if len(queue) >= maxQueuedPerChat {
		q.mu.Unlock()
		return false
	}
	q.pending[chatID] = append(queue, fn)
	q.mu.Unlock()
	if busy {
		return true
	}
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		for {
			q.mu.Lock()
			queue := q.pending[chatID]
			if len(queue) == 0 {
				delete(q.pending, chatID)
				q.mu.Unlock()
				return
			}
			fn := queue[0]
			q.pending[chatID] = queue[1:]
			q.mu.Unlock()

			q.slots <- struct{}{}
			q.run(chatID, fn)
			<-q.slots
		}
	}()
	return true
}

func (q *chatQueue) run(chatID int64, fn func()) {
	defer func() {
		if p := recover(); p != nil {
			metrics.GetOrCreateCounter("bot_update_errors_total").Inc()
			q.log.Error("bot panicked while handling an update", "chat_id", chatID, "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
		}
	}()
	fn()
}

func (q *chatQueue) wait() { q.wg.Wait() }
