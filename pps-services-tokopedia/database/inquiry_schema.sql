-- Schema, tables, and procedures for inquiry repository
-- Matches calls in internal/repository/inquiry_postgres_repository.go

-- Create schema
CREATE SCHEMA IF NOT EXISTS "inquiry";

-- ==========================
-- Tables
-- ==========================

-- Inquiry requests
CREATE TABLE IF NOT EXISTS "inquiry".inquiry_requests (
    id               BIGSERIAL PRIMARY KEY,
    ref_id           TEXT,
    client_number    TEXT,
    category         TEXT NOT NULL,
    rsid             TEXT NOT NULL,
    product_code     TEXT NOT NULL,
    ts               TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Inquiry responses
CREATE TABLE IF NOT EXISTS "inquiry".inquiry_responses (
    id                 BIGSERIAL PRIMARY KEY,
    inquiry_request_id BIGINT REFERENCES "inquiry".inquiry_requests(id) ON DELETE CASCADE,
    pps_inquiry_id     TEXT,
    client_number      TEXT,
    product_code       TEXT NOT NULL,
    response_code      TEXT NOT NULL,
    message            TEXT,
    total_amount       NUMERIC(18,2) NOT NULL DEFAULT 0,
    ts                 TIMESTAMPTZ NOT NULL,
    bill_count         INT NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Bill details (linked by pps_inquiry_id as used by repository)
CREATE TABLE IF NOT EXISTS "inquiry".bill_details (
    id              BIGSERIAL PRIMARY KEY,
    pps_inquiry_id  TEXT NOT NULL,
    name            TEXT NOT NULL,
    value           TEXT,
    is_pii          BOOLEAN NOT NULL DEFAULT FALSE,
    is_show         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ==========================
-- Procedures (PL/pgSQL)
-- ==========================

-- SELECT "inquiry".inquiry_request_oninsert($1..$6)
-- RETURNS (inquiry_request_id BIGINT, error INT, message TEXT)
CREATE OR REPLACE FUNCTION "inquiry".inquiry_request_oninsert(
    p_ref_id        TEXT,
    p_client_number TEXT,
    p_category      TEXT,
    p_rsid          TEXT,
    p_product_code  TEXT,
    p_timestamp     TIMESTAMPTZ
)
RETURNS TABLE(inquiry_request_id BIGINT, error INT, message TEXT)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO "inquiry".inquiry_requests(
        ref_id, client_number, category, rsid, product_code, ts
    ) VALUES (
        p_ref_id, p_client_number, p_category, p_rsid, p_product_code, p_timestamp
    ) RETURNING id INTO inquiry_request_id;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    inquiry_request_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
$$;

-- SELECT "inquiry".inquiry_response_oninsert($1..$9)
-- RETURNS (inquiry_response_id BIGINT, error INT, message TEXT)
CREATE OR REPLACE FUNCTION "inquiry".inquiry_response_oninsert(
    p_inquiry_request_id BIGINT,
    p_pps_inquiry_id     TEXT,
    p_client_number      TEXT,
    p_product_code       TEXT,
    p_response_code      TEXT,
    p_message            TEXT,
    p_total_amount       NUMERIC(18,2),
    p_timestamp          TIMESTAMPTZ,
    p_bill_count         INT
)
RETURNS TABLE(inquiry_response_id BIGINT, error INT, message TEXT)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO "inquiry".inquiry_responses(
        inquiry_request_id, pps_inquiry_id, client_number, product_code,
        response_code, message, total_amount, ts, bill_count
    ) VALUES (
        p_inquiry_request_id, p_pps_inquiry_id, p_client_number, p_product_code,
        p_response_code, p_message, p_total_amount, p_timestamp, p_bill_count
    ) RETURNING id INTO inquiry_response_id;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    inquiry_response_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
$$;

-- Insert bill detail
-- SELECT "inquiry".bill_detail_oninsert($1..$5)
-- RETURNS (bill_detail_id BIGINT, error INT, message TEXT)
CREATE OR REPLACE FUNCTION "inquiry".bill_detail_oninsert(
    p_pps_inquiry_id TEXT,
    p_name           TEXT,
    p_value          TEXT,
    p_is_pii         BOOLEAN,
    p_is_show        BOOLEAN
)
RETURNS TABLE(bill_detail_id BIGINT, error INT, message TEXT)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO "inquiry".bill_details(
        pps_inquiry_id, name, value, is_pii, is_show
    ) VALUES (
        p_pps_inquiry_id, p_name, p_value, p_is_pii, p_is_show
    ) RETURNING id INTO bill_detail_id;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    bill_detail_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
$$;

-- Optional indexes to improve lookup performance
CREATE INDEX IF NOT EXISTS idx_inquiry_requests_ref_id
    ON "inquiry".inquiry_requests(ref_id);

CREATE INDEX IF NOT EXISTS idx_inquiry_responses_request_id
    ON "inquiry".inquiry_responses(inquiry_request_id);

-- Composite index for ValidateInquiryId query performance
CREATE INDEX IF NOT EXISTS idx_inquiry_responses_validation
    ON "inquiry".inquiry_responses(pps_inquiry_id, product_code, total_amount);

CREATE INDEX IF NOT EXISTS idx_bill_details_pps_inquiry_id
    ON "inquiry".bill_details(pps_inquiry_id);


-- ==========================
-- Cleanup Utilities
-- ==========================

-- Function to clean up old inquiry logs (requests, responses, and bill_details)
CREATE OR REPLACE FUNCTION "inquiry".cleanup_old_inquiry_logs(
    p_days_to_keep INTEGER DEFAULT 31
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_deleted_count INTEGER := 0;
BEGIN
    -- Delete old bill_details whose parent inquiry_responses are old
    DELETE FROM "inquiry".bill_details bd
    USING "inquiry".inquiry_responses ir
    WHERE bd.pps_inquiry_id = ir.pps_inquiry_id
      AND ir.created_at < NOW() - INTERVAL '1 day' * p_days_to_keep;

    -- Delete old inquiry_responses
    DELETE FROM "inquiry".inquiry_responses ir
    WHERE ir.created_at < NOW() - INTERVAL '1 day' * p_days_to_keep;

    -- Delete orphaned inquiry_requests that have no responses and are old
    DELETE FROM "inquiry".inquiry_requests iq
    WHERE iq.created_at < NOW() - INTERVAL '1 day' * p_days_to_keep
      AND NOT EXISTS (
        SELECT 1 FROM "inquiry".inquiry_responses ir
        WHERE ir.inquiry_request_id = iq.id
      );

    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    RETURN v_deleted_count;
END;
$$;

