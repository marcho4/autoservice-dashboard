CREATE TABLE employees (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL,
    full_name     text        NOT NULL,
    password_hash text        NOT NULL,
    token_version integer     NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT employees_email_key UNIQUE (email)
);

CREATE TABLE revoked_tokens (
    jti        text PRIMARY KEY,
    expires_at timestamptz NOT NULL
);
CREATE INDEX revoked_tokens_expires_at_idx ON revoked_tokens (expires_at);

-- One autoservice per employee: owner_id is unique. Deleting the employee deletes the autoservice.
CREATE TABLE autoservices (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid        NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    name        text        NOT NULL,
    address     text        NOT NULL DEFAULT '',
    phone       text        NOT NULL DEFAULT '',
    description text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT autoservices_owner_id_key UNIQUE (owner_id)
);

-- One bot per autoservice, and a Telegram bot can be connected to only one autoservice.
CREATE TABLE bots (
    autoservice_id  uuid PRIMARY KEY REFERENCES autoservices (id) ON DELETE CASCADE,
    telegram_bot_id bigint      NOT NULL,
    username        text        NOT NULL,
    token           text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT bots_telegram_bot_id_key UNIQUE (telegram_bot_id)
);

CREATE TABLE clients (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    autoservice_id    uuid        NOT NULL REFERENCES autoservices (id) ON DELETE CASCADE,
    telegram_id       bigint      NOT NULL,
    telegram_username text        NOT NULL DEFAULT '',
    name              text        NOT NULL DEFAULT '',
    phone             text        NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT clients_autoservice_telegram_key UNIQUE (autoservice_id, telegram_id),
    CONSTRAINT clients_id_autoservice_key UNIQUE (id, autoservice_id)
);

CREATE TABLE requests (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    autoservice_id uuid        NOT NULL REFERENCES autoservices (id) ON DELETE CASCADE,
    client_id      uuid        NOT NULL,
    status         text        NOT NULL DEFAULT 'new',
    car_brand      text        NOT NULL,
    car_model      text        NOT NULL,
    description    text        NOT NULL,
    phone          text        NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT requests_status_check CHECK (status IN ('new', 'in_progress', 'closed', 'rejected', 'cancelled')),
    -- The client must belong to the same autoservice as the request.
    CONSTRAINT requests_client_fkey FOREIGN KEY (client_id, autoservice_id)
        REFERENCES clients (id, autoservice_id) ON DELETE CASCADE
);
CREATE INDEX requests_autoservice_created_idx ON requests (autoservice_id, created_at DESC);
CREATE INDEX requests_autoservice_status_created_idx ON requests (autoservice_id, status, created_at DESC);
CREATE INDEX requests_client_created_idx ON requests (client_id, created_at DESC);
