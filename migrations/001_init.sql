CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS campaigns (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             TEXT NOT NULL,
    pricing_model    TEXT NOT NULL CHECK (pricing_model IN ('CPM','CPC')),
    bid_micros       BIGINT NOT NULL,
    budget_total     BIGINT NOT NULL,
    budget_daily     BIGINT NOT NULL,
    spent_total      BIGINT NOT NULL DEFAULT 0,
    spent_today      BIGINT NOT NULL DEFAULT 0,
    spent_today_date DATE NOT NULL DEFAULT CURRENT_DATE,
    active           BOOLEAN NOT NULL DEFAULT TRUE,
    start_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_at           TIMESTAMPTZ,
    targeting_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
    frequency_cap    INT NOT NULL DEFAULT 10
);

CREATE TABLE IF NOT EXISTS creatives (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id  UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    duration_sec INT NOT NULL,
    video_url    TEXT NOT NULL,
    landing_url  TEXT NOT NULL,
    lang         TEXT NOT NULL,
    placement    TEXT NOT NULL,
    category     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS impressions (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token        TEXT NOT NULL UNIQUE,
    request_id   TEXT UNIQUE, -- идемпотентность RequestAd
    campaign_id  UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    creative_id  UUID NOT NULL REFERENCES creatives(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL,
    session_id   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS clicks (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    impression_id UUID NOT NULL REFERENCES impressions(id) ON DELETE CASCADE,
    token         TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (impression_id) -- идемпотентность CPC-биллинга по impression
);

CREATE TABLE IF NOT EXISTS views (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    impression_id UUID NOT NULL REFERENCES impressions(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (impression_id) -- один просмотр на показ (при желании)
);

CREATE TABLE IF NOT EXISTS user_campaign_daily (
    user_id     UUID NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    day         DATE NOT NULL,
    impressions INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, campaign_id, day)
);

CREATE TABLE IF NOT EXISTS spend_events (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id   UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    event_type    TEXT NOT NULL CHECK (event_type IN ('CPM_IMPRESSION','CPC_CLICK')),
    amount_micros BIGINT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_impressions_campaign_created_at
    ON impressions (campaign_id, created_at);

CREATE INDEX IF NOT EXISTS idx_clicks_impression_created_at
    ON clicks (impression_id, created_at);

CREATE INDEX IF NOT EXISTS idx_spend_events_campaign_time
    ON spend_events (campaign_id, occurred_at);
