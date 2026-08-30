CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS procedures (
    id                   TEXT PRIMARY KEY,
    auction_id           TEXT,
    selling_method       TEXT,
    selling_entity       TEXT,
    status               TEXT,
    start_amount         DOUBLE PRECISION,
    currency             TEXT,
    title                TEXT,
    description          TEXT,
    auction_start        TIMESTAMPTZ,
    date_modified        TIMESTAMPTZ,
    previous_auction_id  TEXT,
    tender_attempts      INT,
    raw                  JSONB,
    created_at           TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS procedures_status_idx ON procedures (status);
CREATE INDEX IF NOT EXISTS procedures_modified_idx ON procedures (date_modified DESC);

CREATE TABLE IF NOT EXISTS lot_attrs (
    procedure_id      TEXT PRIMARY KEY REFERENCES procedures(id) ON DELETE CASCADE,
    kind              TEXT NOT NULL,
    rooms             INT,
    area              DOUBLE PRECISION,
    year              INT,
    city              TEXT,
    brand             TEXT,
    model             TEXT,
    marka_id          INT,
    model_id          INT,
    city_id           INT,
    parse_confidence  DOUBLE PRECISION,
    updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ria_dicts (
    kind      TEXT NOT NULL,
    ria_id    INT NOT NULL,
    parent_id INT,
    name      TEXT NOT NULL,
    PRIMARY KEY (kind, ria_id)
);

CREATE TABLE IF NOT EXISTS price_snapshots (
    spec_hash   TEXT PRIMARY KEY,
    median      DOUBLE PRECISION,
    arithmetic  DOUBLE PRECISION,
    currency    TEXT,
    fetched_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS scores (
    procedure_id  TEXT PRIMARY KEY REFERENCES procedures(id) ON DELETE CASCADE,
    discount_pct  DOUBLE PRECISION,
    pass_n        INT,
    auction_family TEXT,
    ria_median    DOUBLE PRECISION,
    scored_at     TIMESTAMPTZ DEFAULT now()
);
