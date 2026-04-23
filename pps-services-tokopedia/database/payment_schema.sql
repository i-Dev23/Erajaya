-- Schema, tables, and procedures for payment repository
-- Matches calls in internal/repository/payment_postgres_repository.go

-- Create schema
CREATE SCHEMA IF NOT EXISTS "payment";

-- ==========================
-- Tables
-- ==========================

-- Payment requests
CREATE TABLE IF NOT EXISTS "payment".payment_requests (
    id                  BIGSERIAL PRIMARY KEY,
    ref_id              TEXT NOT NULL UNIQUE,
    partner_inquiry_id  TEXT NOT NULL,
    client_number       TEXT,
    category            TEXT NOT NULL,
    rsid                TEXT NOT NULL,
    product_code        TEXT NOT NULL,
    total_amount        NUMERIC(18,2) NOT NULL DEFAULT 0,
    ts                  TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Payment responses
CREATE TABLE IF NOT EXISTS "payment".payment_responses (
    id                  BIGSERIAL PRIMARY KEY,
    payment_request_id  BIGINT REFERENCES "payment".payment_requests(id) ON DELETE CASCADE,
    partner_ref_id      TEXT NOT NULL,
    client_number       TEXT,
    product_code        TEXT NOT NULL,
    response_code       TEXT NOT NULL,
    message             TEXT,
    admin_fee           NUMERIC(18,2) DEFAULT 0,
    total_amount        NUMERIC(18,2) NOT NULL DEFAULT 0,
    sales_price         NUMERIC(18,2) DEFAULT 0,
    ts                  TIMESTAMPTZ NOT NULL,
    -- Timestamp capturing the moment when response_code/message last changed
    response_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    bill_count          INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Payment bill details (separate from inquiry bill details)
CREATE TABLE IF NOT EXISTS "payment".payment_bill_details (
    id              BIGSERIAL PRIMARY KEY,
    partner_ref_id  TEXT NOT NULL,
    name            TEXT NOT NULL,
    value           TEXT,
    is_pii          BOOLEAN NOT NULL DEFAULT FALSE,
    is_show         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ==========================
-- Procedures (PL/pgSQL)
-- ==========================

-- SELECT "payment".payment_request_oninsert($1..$8)
-- RETURNS (payment_request_id BIGINT, error INT, message TEXT)
CREATE OR REPLACE FUNCTION "payment".payment_request_oninsert(
    p_ref_id             TEXT,
    p_partner_inquiry_id TEXT,
    p_client_number      TEXT,
    p_category           TEXT,
    p_rsid               TEXT,
    p_product_code       TEXT,
    p_total_amount       NUMERIC(18,2),
    p_timestamp          TIMESTAMPTZ
)
RETURNS TABLE(payment_request_id BIGINT, error INT, message TEXT)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO "payment".payment_requests(
        ref_id, partner_inquiry_id, client_number, category, rsid, product_code, total_amount, ts
    ) VALUES (
        p_ref_id, p_partner_inquiry_id, p_client_number, p_category, p_rsid, p_product_code, p_total_amount, p_timestamp
    ) RETURNING id INTO payment_request_id;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    payment_request_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
$$;

-- SELECT "payment".payment_response_oninsert($1..$10)
-- RETURNS (payment_response_id BIGINT, error INT, message TEXT)
CREATE OR REPLACE FUNCTION "payment".payment_response_oninsert(
    p_payment_request_id BIGINT,
    p_partner_ref_id     TEXT,
    p_client_number      TEXT,
    p_product_code       TEXT,
    p_response_code      TEXT,
    p_message            TEXT,
    p_admin_fee          NUMERIC(18,2),
    p_total_amount       NUMERIC(18,2),
    p_timestamp          TIMESTAMPTZ,
    p_bill_count         INT
)
RETURNS TABLE(payment_response_id BIGINT, error INT, message TEXT)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO "payment".payment_responses(
        payment_request_id, partner_ref_id, client_number, product_code,
        response_code, message, admin_fee, total_amount, ts, bill_count
    ) VALUES (
        p_payment_request_id, p_partner_ref_id, p_client_number, p_product_code,
        p_response_code, p_message, p_admin_fee, p_total_amount, p_timestamp, p_bill_count
    ) RETURNING id INTO payment_response_id;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    payment_response_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
$$;

-- ==========================
-- Triggers
-- ==========================

-- Automatically update response_updated_at when response_code or message changes
CREATE OR REPLACE FUNCTION "payment".set_response_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF (NEW.response_code IS DISTINCT FROM OLD.response_code)
        OR (NEW.message IS DISTINCT FROM OLD.message) THEN
        NEW.response_updated_at := now();
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_set_response_updated_at ON "payment".payment_responses;
CREATE TRIGGER trg_set_response_updated_at
BEFORE UPDATE OF response_code, message ON "payment".payment_responses
FOR EACH ROW
EXECUTE FUNCTION "payment".set_response_updated_at();

-- Insert payment bill detail
-- SELECT "payment".payment_bill_detail_oninsert($1..$5)
-- RETURNS (payment_bill_detail_id BIGINT, error INT, message TEXT)
CREATE OR REPLACE FUNCTION "payment".payment_bill_detail_oninsert(
    p_partner_ref_id TEXT,
    p_name           TEXT,
    p_value          TEXT,
    p_is_pii         BOOLEAN,
    p_is_show        BOOLEAN
)
RETURNS TABLE(payment_bill_detail_id BIGINT, error INT, message TEXT)
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO "payment".payment_bill_details(
        partner_ref_id, name, value, is_pii, is_show
    ) VALUES (
        p_partner_ref_id, p_name, p_value, p_is_pii, p_is_show
    ) RETURNING id INTO payment_bill_detail_id;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    payment_bill_detail_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
$$;

-- ==========================
-- Indexes
-- ==========================

-- Unique index for ref_id to ensure no duplicates
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_requests_ref_id
    ON "payment".payment_requests(ref_id);

-- Index for partner_inquiry_id validation
CREATE INDEX IF NOT EXISTS idx_payment_requests_partner_inquiry_id
    ON "payment".payment_requests(partner_inquiry_id);

-- Index for payment responses lookup by request
CREATE INDEX IF NOT EXISTS idx_payment_responses_request_id
    ON "payment".payment_responses(payment_request_id);

-- Index for payment responses lookup by partner_ref_id
CREATE INDEX IF NOT EXISTS idx_payment_responses_partner_ref_id
    ON "payment".payment_responses(partner_ref_id);

-- Index for payment bill details lookup
CREATE INDEX IF NOT EXISTS idx_payment_bill_details_partner_ref_id
    ON "payment".payment_bill_details(partner_ref_id);

-- Performance indexes for reconciliation report queries
-- Index for filtering by created_at date (used in daily reconciliation reports)
CREATE INDEX IF NOT EXISTS idx_payment_requests_created_at
    ON "payment".payment_requests(created_at DESC);

-- Index for filtering successful transactions (response_code = '00')
CREATE INDEX IF NOT EXISTS idx_payment_responses_response_code
    ON "payment".payment_responses(response_code);

-- Composite index for fast response_code filtering + join on payment_responses
-- Optimizes: WHERE response_code = '00' AND JOIN ON partner_ref_id
CREATE INDEX IF NOT EXISTS idx_payment_responses_code_partner_ref
    ON "payment".payment_responses(response_code, partner_ref_id);

-- Composite index for payment_responses created_at + partner_ref_id
-- Optimizes: date filtering on responses + bill_details join
CREATE INDEX IF NOT EXISTS idx_payment_responses_created_at_partner_ref
    ON "payment".payment_responses(created_at, partner_ref_id);

-- Index for bill detail name filtering (used in CASE statements for grouping)
CREATE INDEX IF NOT EXISTS idx_payment_bill_details_name
    ON "payment".payment_bill_details(partner_ref_id, name);


-- ==========================
-- Cleanup Utilities
-- ==========================

-- Function to clean up old payment logs (responses, bill_details, and orphaned requests)
CREATE OR REPLACE FUNCTION "payment".cleanup_old_payment_logs(
    p_days_to_keep INTEGER DEFAULT 31
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_deleted_count INTEGER := 0;
BEGIN
    -- Delete old payment_bill_details whose parent responses are old
    DELETE FROM "payment".payment_bill_details pbd
    USING "payment".payment_responses pr
    WHERE pbd.partner_ref_id = pr.partner_ref_id
      AND pr.created_at < NOW() - INTERVAL '1 day' * p_days_to_keep;

    -- Delete old payment_responses
    DELETE FROM "payment".payment_responses pr
    WHERE pr.created_at < NOW() - INTERVAL '1 day' * p_days_to_keep;

    -- Delete orphaned payment_requests that have no responses and are old
    DELETE FROM "payment".payment_requests pq
    WHERE pq.created_at < NOW() - INTERVAL '1 day' * p_days_to_keep
      AND NOT EXISTS (
        SELECT 1 FROM "payment".payment_responses pr
        WHERE pr.payment_request_id = pq.id
      );

    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    RETURN v_deleted_count;
END;
$$;


-- DROP FUNCTION payment.payment_status_onupdate(text, int4);
CREATE OR REPLACE FUNCTION payment.payment_status_onupdate(p_ref_id text, p_status_code integer)
 RETURNS TABLE(partnerrefid character varying, error integer, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
	vStatusMessage varchar;
	vResponseCode varchar;
BEGIN

	if p_status_code = 0 then
		vStatusMessage := 'Success';
		vResponseCode := '00';
	elsif p_status_code = 1 then
		vStatusMessage := 'Failed';
		vResponseCode := '02';
	end if;

	UPDATE "payment".payment_responses
	SET response_code = vResponseCode,
	"message" = vStatusMessage
	WHERE payment_request_id = (
		SELECT id FROM "payment".payment_requests pq
		WHERE pq.ref_id = p_ref_id
		LIMIT 1
	) returning partner_ref_id into partnerrefid;
	
	IF partnerrefid IS NULL THEN
		RAISE 'Payment not found';
	END IF;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
	partnerrefid := null;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
$function$
;

-- DROP FUNCTION payment.payment_status_onupdate(text, varchar, varchar);
CREATE OR REPLACE FUNCTION payment.payment_status_onupdate(p_ref_id text, p_responsecode character varying, p_responsemsg character varying)
 RETURNS TABLE(partnerrefid character varying, error integer, message text)
 LANGUAGE plpgsql
AS $function$
BEGIN

	--raise exception 'Test error';

	UPDATE "payment".payment_responses
	SET response_code = p_responsecode,
	"message" = p_responsemsg
	WHERE payment_request_id = (
		SELECT id FROM "payment".payment_requests pq
		WHERE pq.ref_id = p_ref_id
		LIMIT 1
	) returning partner_ref_id into partnerrefid;
	
	IF partnerrefid IS NULL THEN
		RAISE 'Payment not found';
	END IF;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
	partnerrefid := null;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
$function$


-- DROP FUNCTION payment.payment_status_onupdate_with_sales_price(text, varchar, varchar, numeric);
CREATE OR REPLACE FUNCTION payment.payment_status_onupdate_with_sales_price(
    p_ref_id text,
    p_responsecode character varying,
    p_responsemsg character varying,
    p_sales_price numeric
)
RETURNS TABLE(partnerrefid character varying, error integer, message text)
LANGUAGE plpgsql
AS $function$
BEGIN
    UPDATE "payment".payment_responses
    SET response_code = p_responsecode,
        "message" = p_responsemsg,
        sales_price = p_sales_price
    WHERE payment_request_id = (
        SELECT id FROM "payment".payment_requests pq
        WHERE pq.ref_id = p_ref_id
        LIMIT 1
    ) RETURNING partner_ref_id INTO partnerrefid;

    IF partnerrefid IS NULL THEN
        RAISE 'Payment not found';
    END IF;

    error := 0;
    message := 'OK';
    RETURN NEXT;
EXCEPTION WHEN others THEN
        partnerrefid := null;
        error := 1;
        message := SQLERRM;
        RETURN NEXT;
END;
$function$;


-- ==========================
-- Reconciliation Report
-- ==========================

-- Function to get daily reconciliation report for yesterday's successful payments
-- DROP FUNCTION payment.get_daily_reconciliation_report(date);
CREATE OR REPLACE FUNCTION payment.get_daily_reconciliation_report(p_report_date date DEFAULT CURRENT_DATE - INTERVAL '1 day')
RETURNS TABLE(
    timestamp_col text,
    ref_id text,
    client_number text,
    client_name text,
    tarif_daya text,
    rp_token text,
    jumlah_kwh text,
    nomor_token text,
    amount numeric,
    sales_price numeric,
    partner_ref_id text
)
LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        TO_CHAR(pr.created_at, 'YYYY-MM-DD HH24:MI:SS') AS timestamp_col,
        pr.ref_id,
        pr.client_number,
        MAX(CASE WHEN pbd.name = 'Nama' THEN pbd.value END) AS client_name,
        MAX(CASE WHEN pbd.name = 'Tarif/Daya' THEN pbd.value END) AS tarif_daya,
        MAX(CASE WHEN pbd.name = 'Rp Stroom/Token' THEN pbd.value END) AS rp_token,
        MAX(CASE WHEN pbd.name = 'kWh' THEN pbd.value END) AS jumlah_kwh,
        MAX(CASE WHEN pbd.name = 'Token' THEN pbd.value END) AS nomor_token,
        resp.total_amount AS amount,
        resp.sales_price,
        resp.partner_ref_id
    FROM payment.payment_requests pr
    JOIN payment.payment_responses resp
        ON resp.payment_request_id = pr.id
    LEFT JOIN payment.payment_bill_details pbd
        ON pbd.partner_ref_id = resp.partner_ref_id
        AND pbd.name IN ('Nama', 'Tarif/Daya', 'Rp Stroom/Token', 'kWh', 'Token')
    WHERE resp.response_code = '00'
      AND pr.created_at >= p_report_date::date
      AND pr.created_at < p_report_date::date + INTERVAL '1 day'
    GROUP BY pr.id, pr.ref_id, pr.client_number, pr.created_at, 
             resp.total_amount, resp.sales_price, resp.partner_ref_id
    ORDER BY pr.created_at DESC;
END;
$function$;
