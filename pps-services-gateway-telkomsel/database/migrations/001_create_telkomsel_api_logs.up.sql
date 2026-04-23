CREATE SCHEMA IF NOT EXISTS log;

CREATE TABLE log.telkomsel_api_logs (
    id                      BIGSERIAL       PRIMARY KEY,
    endpoint                VARCHAR(255)    NOT NULL,
    method                  VARCHAR(10)     NOT NULL,
    external_transaction_id VARCHAR(50),
    msisdn                  VARCHAR(20),
    mid                     VARCHAR(50),
    queue_name              VARCHAR(100),
    msg_id                  VARCHAR(100),

    -- Request
    request_url             TEXT            NOT NULL,
    request_headers         JSONB,
    request_body            JSONB,

    -- Response
    response_status_code    INT,
    response_body           JSONB,
    response_duration_ms    INT,

    -- Result
    status_code             VARCHAR(100),
    status_desc             TEXT,
    error_message           TEXT,
    error_type              VARCHAR(20),

    created_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- Index untuk query yang sering dipakai
CREATE INDEX idx_telkomsel_api_logs_ext_tx_id ON log.telkomsel_api_logs (external_transaction_id);
CREATE INDEX idx_telkomsel_api_logs_msisdn ON log.telkomsel_api_logs (msisdn);
CREATE INDEX idx_telkomsel_api_logs_endpoint ON log.telkomsel_api_logs (endpoint);
CREATE INDEX idx_telkomsel_api_logs_created_at ON log.telkomsel_api_logs (created_at);
CREATE INDEX idx_telkomsel_api_logs_mid ON log.telkomsel_api_logs (mid);
CREATE INDEX idx_telkomsel_api_logs_status_code ON log.telkomsel_api_logs (status_code);

COMMENT ON TABLE log.telkomsel_api_logs IS 'Log request dan response dari gateway ke Telkomsel ESB API';
COMMENT ON COLUMN log.telkomsel_api_logs.endpoint IS 'Path endpoint, e.g. /esb/v1/modern/recharge/dealer';
COMMENT ON COLUMN log.telkomsel_api_logs.external_transaction_id IS 'Transaction ID yang dikirim ke Telkomsel';
COMMENT ON COLUMN log.telkomsel_api_logs.status_code IS 'Telkomsel status_code dari response (00000 = success)';
COMMENT ON COLUMN log.telkomsel_api_logs.error_type IS 'business atau technical';
COMMENT ON COLUMN log.telkomsel_api_logs.response_duration_ms IS 'Durasi round-trip dalam milliseconds';
