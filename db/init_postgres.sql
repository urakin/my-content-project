CREATE TABLE IF NOT EXISTS articles
(
    id         SERIAL PRIMARY KEY,
    title      TEXT      NOT NULL,
    content    TEXT,
    source_url TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
