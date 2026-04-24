CREATE SCHEMA IF NOT EXISTS log_smb;

CREATE TABLE IF NOT EXISTS log_smb.smb_api_logs (
    id                      BIGSERIAL       PRIMARY KEY,
    endpoint                VARCHAR(255)    NOT NULL,
    method                  VARCHAR(10)     NOT NULL,
    client_number           VARCHAR(50),
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
CREATE INDEX IF NOT EXISTS idx_smb_api_logs_client_number ON log_smb.smb_api_logs (client_number);
CREATE INDEX IF NOT EXISTS idx_smb_api_logs_endpoint ON log_smb.smb_api_logs (endpoint);
CREATE INDEX IF NOT EXISTS idx_smb_api_logs_created_at ON log_smb.smb_api_logs (created_at);
CREATE INDEX IF NOT EXISTS idx_smb_api_logs_mid ON log_smb.smb_api_logs (mid);
CREATE INDEX IF NOT EXISTS idx_smb_api_logs_msg_id ON log_smb.smb_api_logs (msg_id);
CREATE INDEX IF NOT EXISTS idx_smb_api_logs_status_code ON log_smb.smb_api_logs (status_code);

COMMENT ON TABLE log_smb.smb_api_logs IS 'Log request dan response dari gateway ke SMB API';
