CREATE TABLE bot_updates (
    autoservice_id  uuid   NOT NULL REFERENCES bots (autoservice_id) ON DELETE CASCADE,
    telegram_bot_id bigint NOT NULL,
    update_id       bigint NOT NULL,
    payload         jsonb  NOT NULL,
    PRIMARY KEY (autoservice_id, telegram_bot_id, update_id)
);
