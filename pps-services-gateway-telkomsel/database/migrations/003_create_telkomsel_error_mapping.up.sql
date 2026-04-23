-- Tabel error mapping: pemetaan HTTP Status Code + ESB Status Code ke RC PPS
CREATE TABLE IF NOT EXISTS log.telkomsel_error_mapping (
    id                SERIAL        PRIMARY KEY,
    http_status_code  INTEGER       NOT NULL,
    esb_status_code   VARCHAR(100)   NOT NULL,
    rc_pps            INTEGER,
    description       TEXT,
    created_at        TIMESTAMPTZ   DEFAULT NOW(),
    updated_at        TIMESTAMPTZ   DEFAULT NOW()
);

COMMENT ON TABLE log.telkomsel_error_mapping IS 'Mapping kombinasi HTTP Status Code + ESB Status Code ke RC PPS';
COMMENT ON COLUMN log.telkomsel_error_mapping.rc_pps IS '0 = sukses, 1 = gagal, 9 = pending';
