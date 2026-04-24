CREATE SCHEMA IF NOT EXISTS transaction_smb;

-- Tabel utama: satu baris per pesan RabbitMQ yang masuk
CREATE TABLE IF NOT EXISTS transaction_smb.smb_transaction (
    msg_id             VARCHAR     PRIMARY KEY,
    our_trx_id         VARCHAR     NOT NULL,
    client_number      VARCHAR     NOT NULL,
    mid                VARCHAR     NOT NULL,
    product_type       VARCHAR     NOT NULL,
    product_code       VARCHAR,
    amount             INTEGER     NOT NULL,
    queue_name         VARCHAR     NOT NULL,
    mq_transaction     VARCHAR,
    status             VARCHAR     NOT NULL DEFAULT 'PROCESSING',
    processing_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    success_at         TIMESTAMPTZ,
    failed_at          TIMESTAMPTZ,
    first_requested_at TIMESTAMPTZ,
    last_response_at   TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tabel respons: satu baris per respons dari SMB API (SYNC atau CALLBACK)
CREATE TABLE IF NOT EXISTS transaction_smb.smb_transaction_response (
    id                  BIGSERIAL   PRIMARY KEY,
    msg_id              VARCHAR     NOT NULL,
    our_trx_id          VARCHAR     NOT NULL,
    smb_trx_id          VARCHAR,
    response_type       VARCHAR     NOT NULL,
    status_code         VARCHAR,
    status_desc         VARCHAR,
    request_payload     JSONB,
    raw_payload         JSONB,
    requested_at        TIMESTAMPTZ,
    response_latency_ms INTEGER,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_smb_transaction_response_msg_id
    ON transaction_smb.smb_transaction_response (msg_id);

CREATE INDEX IF NOT EXISTS idx_smb_transaction_our_trx_id
    ON transaction_smb.smb_transaction (our_trx_id);

COMMENT ON TABLE transaction_smb.smb_transaction IS 'Tracking lifecycle transaksi dari RabbitMQ ke SMB API';
COMMENT ON TABLE transaction_smb.smb_transaction_response IS 'Log setiap respons dari SMB API (SYNC dan CALLBACK)';
COMMENT ON COLUMN transaction_smb.smb_transaction.status IS 'PROCESSING | SUCCESS | FAILED';
COMMENT ON COLUMN transaction_smb.smb_transaction_response.response_type IS 'SYNC | CALLBACK';
