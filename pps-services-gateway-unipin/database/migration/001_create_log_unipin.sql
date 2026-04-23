CREATE SCHEMA IF NOT EXISTS log_unipin;

CREATE TABLE log_unipin.api_log (
    id              BIGSERIAL       PRIMARY KEY,
    endpoint        VARCHAR(255)    NOT NULL,
    method          VARCHAR(10)     NOT NULL,
    request_url     TEXT            NOT NULL,
    request_headers JSONB,
    request_body    TEXT,
    response_code   INT,
    response_body   TEXT,
    duration_ms     BIGINT,
    error_message   TEXT,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_log_endpoint ON log_unipin.api_log (endpoint);
CREATE INDEX idx_api_log_created_at ON log_unipin.api_log (created_at);
CREATE INDEX idx_api_log_response_code ON log_unipin.api_log (response_code);
