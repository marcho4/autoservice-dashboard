CREATE TABLE services (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    autoservice_id uuid        NOT NULL REFERENCES autoservices (id) ON DELETE CASCADE,
    name           text        NOT NULL,
    description    text        NOT NULL DEFAULT '',
    is_active      boolean     NOT NULL DEFAULT true,
    position       integer     NOT NULL DEFAULT 0,
    fields         jsonb       NOT NULL DEFAULT '[]',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX services_autoservice_position_idx ON services (autoservice_id, position, created_at);

ALTER TABLE autoservices ADD COLUMN request_seq integer NOT NULL DEFAULT 0;

ALTER TABLE requests
    ADD COLUMN number       integer,
    ADD COLUMN service_id   uuid REFERENCES services (id) ON DELETE SET NULL,
    ADD COLUMN service_name text  NOT NULL DEFAULT '',
    ADD COLUMN answers      jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN message      text  NOT NULL DEFAULT '',
    ADD COLUMN note         text  NOT NULL DEFAULT '';

UPDATE requests
SET service_name = 'Заявка',
    answers = jsonb_build_array(
        jsonb_build_object('label', 'Автомобиль', 'value', car_brand || ' ' || car_model),
        jsonb_build_object('label', 'Описание', 'value', description));

WITH numbered AS (
    SELECT id, row_number() OVER (PARTITION BY autoservice_id ORDER BY created_at, id) AS n
    FROM requests
)
UPDATE requests r SET number = numbered.n FROM numbered WHERE numbered.id = r.id;

UPDATE autoservices a
SET request_seq = COALESCE((SELECT max(r.number) FROM requests r WHERE r.autoservice_id = a.id), 0);

ALTER TABLE requests
    ALTER COLUMN number SET NOT NULL,
    ADD CONSTRAINT requests_autoservice_number_key UNIQUE (autoservice_id, number),
    DROP COLUMN car_brand,
    DROP COLUMN car_model,
    DROP COLUMN description;

ALTER TABLE bots
    ADD COLUMN status       text        NOT NULL DEFAULT 'pending',
    ADD COLUMN status_error text        NOT NULL DEFAULT '',
    ADD COLUMN status_at    timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN link_code    text        NOT NULL DEFAULT replace(gen_random_uuid()::text, '-', ''),
    ADD CONSTRAINT bots_status_check CHECK (status IN ('pending', 'running', 'error'));

CREATE TABLE notify_recipients (
    autoservice_id   uuid        NOT NULL REFERENCES bots (autoservice_id) ON DELETE CASCADE,
    telegram_chat_id bigint      NOT NULL,
    name             text        NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (autoservice_id, telegram_chat_id)
);

CREATE TABLE conversations (
    autoservice_id   uuid        NOT NULL REFERENCES autoservices (id) ON DELETE CASCADE,
    telegram_chat_id bigint      NOT NULL,
    state            jsonb       NOT NULL,
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (autoservice_id, telegram_chat_id)
);
