-- Schema and seed data for error message mappings (Ultima & Oracle)
-- Safe to re-run: uses IF NOT EXISTS and NOT EXISTS guards

-- Create schema
CREATE SCHEMA IF NOT EXISTS "mapping";

-- Table for error message mappings
CREATE TABLE IF NOT EXISTS "mapping".error_message_mappings (
    id              BIGSERIAL PRIMARY KEY,
    system_type     TEXT NOT NULL, -- 'ultima' or 'oracle'
    error_pattern   TEXT NOT NULL, -- Error message pattern to match (now supports regex)
    response_code   TEXT NOT NULL, -- Tokopedia response code
    is_exact_match  BOOLEAN NOT NULL DEFAULT FALSE, -- TRUE for exact match, FALSE for partial match
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(system_type, error_pattern)
);

-- Seed Ultima mappings (insert only when missing)
INSERT INTO "mapping".error_message_mappings (system_type, error_pattern, response_code, description)
SELECT * FROM (VALUES
    ('ultima', 'yang anda masukkan salah', '20', 'Unregistered number'),
    ('ultima', 'kwh melebihi batas maksimum', '21', 'Number blocked'),
    ('ultima', 'no payment', '22', 'Online payment blocked'),
    ('ultima', 'failed system (timeout)', '41', 'Biller timeout'),
    ('ultima', 'sistem sedang kendala', '64', 'Biller error'),
    ('ultima', 'cut-off', '63', 'Biller maintenance (partial match for dynamic cutoff error message)')
) AS v(system_type, error_pattern, response_code, description)
WHERE NOT EXISTS (
    SELECT 1 FROM "mapping".error_message_mappings emm
    WHERE emm.system_type = v.system_type AND emm.error_pattern = v.error_pattern
);

-- Seed Oracle mappings (insert only when missing)
INSERT INTO "mapping".error_message_mappings (system_type, error_pattern, response_code, description)
SELECT * FROM (VALUES
    ('oracle', 'h2h - error :', '61', 'Server maintenance (partial match for dynamic cutoff error message)'),
    ('oracle', 'error 02 : signature tidak benar, komponen tidak syah.', '32', 'Invalid signature'),
    ('oracle', 'error 01 : signature tidak benar.', '32', 'Invalid signature'),
    ('oracle', 'produk belum dimapping', '14', 'Ineligible Product'),
    ('oracle', 'kode voucher is not found', '14', 'Ineligible Product'),
    ('oracle', 'kode voucher', '14', 'Ineligible Product (partial match for dynamic voucher)'),
    ('oracle', 'tidak tersedia untuk account anda.', '14', 'Ineligible Product (partial match for dynamic voucher)'),
    ('oracle', 'fee amount', '14', 'Ineligible Product (partial match for dynamic voucher)'),
    ('oracle', 'belum disetting', '14', 'Ineligible Product (partial match for dynamic voucher)'),
    ('oracle', 'sell error 02 : ip belum di setting.', '60', 'Access is not allowed'),
    ('oracle', 'ip anda', '60', 'Access is not allowed (partial match for dynamic IP)'),
    ('oracle', 'tidak sama dengan settingan', '60', 'Access is not allowed (partial match)'),
    ('oracle', 'anda tidak diperkenankan transaksi lewat web', '60', 'Access is not allowed (partial match)'),
    ('oracle', 'sell error 02 : queue setting is not ready', '60', 'Access is not allowed'),
    ('oracle', 'harga jual belum di setting untuk anda.', '14', 'Ineligible Product'),
    ('oracle', 'stock voucher', '14', 'Ineligible Product'),
    ('oracle', 'habis, transaksi kami batalkan', '14', 'Ineligible Product'),
    ('oracle', 'deposit anda kurang untuk memenuhi penjualan.', '43', 'Insufficient balance'),
    ('oracle', 'no transaksi', '12', 'Transaction not found (partial match for dynamic txn number)'),
    ('oracle', 'tidak ditemukan', '12', 'Transaction not found (partial match)')
) AS v(system_type, error_pattern, response_code, description)
WHERE NOT EXISTS (
    SELECT 1 FROM "mapping".error_message_mappings emm
    WHERE emm.system_type = v.system_type AND emm.error_pattern = v.error_pattern
);

-- Indexes to improve lookup
CREATE INDEX IF NOT EXISTS idx_error_message_mappings_system_type
    ON "mapping".error_message_mappings(system_type);

CREATE INDEX IF NOT EXISTS idx_error_message_mappings_response_code
    ON "mapping".error_message_mappings(response_code);

-- Function to resolve error message mapping (exact first, then partial by longest pattern)
CREATE OR REPLACE FUNCTION "mapping".get_error_message_mapping(
    p_system_type TEXT,
    p_error_message TEXT
)
RETURNS TABLE(response_code TEXT, description TEXT, found BOOLEAN)
LANGUAGE plpgsql
AS $$
DECLARE
    v_response_code TEXT;
    v_description   TEXT;
BEGIN
        -- Exact match (case-insensitive, tanpa filter system_type)
        SELECT emm.response_code, emm.description INTO v_response_code, v_description
        FROM "mapping".error_message_mappings emm
        WHERE emm.is_exact_match = TRUE
            AND LOWER(emm.error_pattern) = LOWER(p_error_message)
        LIMIT 1;

    IF v_response_code IS NOT NULL THEN
        -- Return populated row (avoid NULLs and name clash with FOUND)
        RETURN QUERY SELECT v_response_code, v_description, TRUE;
        RETURN;
    END IF;

        -- Partial match using regex (case-insensitive, longest pattern first, tanpa filter system_type)
        SELECT emm.response_code, emm.description INTO v_response_code, v_description
        FROM "mapping".error_message_mappings emm
        WHERE emm.is_exact_match = FALSE
            AND p_error_message ~* emm.error_pattern
        ORDER BY LENGTH(emm.error_pattern) DESC
        LIMIT 1;

    IF v_response_code IS NOT NULL THEN
        -- Return populated row (avoid NULLs and name clash with FOUND)
        RETURN QUERY SELECT v_response_code, v_description, TRUE;
        RETURN;
    END IF;

    -- Not found: return no rows so caller can decide fallback
    RETURN;
END;
$$;
