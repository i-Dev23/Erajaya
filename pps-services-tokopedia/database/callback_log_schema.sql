-- Schema, tables, and procedures for callback logging repository
-- Matches calls in internal/repository/callback_postgres_repository.go

-- Create schema
CREATE SCHEMA IF NOT EXISTS "callback";

-- ==========================
-- Tables
-- ==========================

-- HTTP Callback Logs table
CREATE TABLE IF NOT EXISTS "callback".http_callback_logs (
    id BIGSERIAL PRIMARY KEY,
    ref_id VARCHAR(50) NOT NULL,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(500) NOT NULL,
    query_params TEXT,
    request_headers JSONB,
    request_body TEXT,
    status_code INTEGER NOT NULL,
    response_headers JSONB,
    response_body TEXT,
    client_ip INET,
    user_agent TEXT,
    response_time_ms BIGINT NOT NULL,
    request_time TIMESTAMP WITH TIME ZONE NOT NULL,
    response_time TIMESTAMP WITH TIME ZONE NOT NULL,
    error_message TEXT,
    callback_type VARCHAR(50), -- e.g., 'payment_status', 'inquiry_result', etc.
    partner_ref_id TEXT, -- Reference from partner system
    client_number TEXT, -- Client/MSISDN number
    product_code TEXT, -- Product code
    response_code TEXT, -- Business response code (e.g., "01", "12")
    total_amount NUMERIC(18,2), -- Transaction amount
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ==========================
-- Procedures (PL/pgSQL)
-- ==========================

-- SELECT "callback".http_callback_log_oninsert($1..$20)
-- RETURNS (http_callback_log_id BIGINT, error INT, message TEXT)
CREATE OR REPLACE FUNCTION "callback".http_callback_log_oninsert(
    p_ref_id VARCHAR(50),
    p_method VARCHAR(10),
    p_path VARCHAR(500),
    p_query_params TEXT,
    p_request_headers TEXT,
    p_request_body TEXT,
    p_status_code INTEGER,
    p_response_headers TEXT,
    p_response_body TEXT,
    p_client_ip INET,
    p_user_agent TEXT,
    p_response_time_ms BIGINT,
    p_request_time TIMESTAMP WITH TIME ZONE,
    p_response_time TIMESTAMP WITH TIME ZONE,
    p_error_message TEXT,
    p_callback_type VARCHAR(50),
    p_partner_ref_id TEXT,
    p_client_number TEXT,
    p_product_code TEXT,
    p_response_code TEXT,
    p_total_amount NUMERIC(18,2)
)
RETURNS TABLE(http_callback_log_id BIGINT, error INT, message TEXT)
LANGUAGE plpgsql
AS $$
DECLARE
    v_request_headers_json JSONB;
    v_response_headers_json JSONB;
BEGIN
    -- Validate input parameters
    IF p_ref_id IS NULL OR p_ref_id = '' THEN
        http_callback_log_id := NULL;
        error := 1;
        message := 'Ref ID is required';
        RETURN NEXT;
        RETURN;
    END IF;

    IF p_method IS NULL OR p_method = '' THEN
        http_callback_log_id := NULL;
        error := 2;
        message := 'HTTP method is required';
        RETURN NEXT;
        RETURN;
    END IF;

    IF p_path IS NULL OR p_path = '' THEN
        http_callback_log_id := NULL;
        error := 3;
        message := 'Path is required';
        RETURN NEXT;
        RETURN;
    END IF;

    IF p_status_code IS NULL OR p_status_code < 100 OR p_status_code > 599 THEN
        http_callback_log_id := NULL;
        error := 4;
        message := 'Valid status code is required';
        RETURN NEXT;
        RETURN;
    END IF;

    -- Parse JSON headers
    BEGIN
        IF p_request_headers IS NOT NULL AND p_request_headers != '' THEN
            v_request_headers_json := p_request_headers::JSONB;
        ELSE
            v_request_headers_json := '{}'::JSONB;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        http_callback_log_id := NULL;
        error := 5;
        message := 'Invalid request headers JSON format';
        RETURN NEXT;
        RETURN;
    END;

    BEGIN
        IF p_response_headers IS NOT NULL AND p_response_headers != '' THEN
            v_response_headers_json := p_response_headers::JSONB;
        ELSE
            v_response_headers_json := '{}'::JSONB;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        http_callback_log_id := NULL;
        error := 6;
        message := 'Invalid response headers JSON format';
        RETURN NEXT;
        RETURN;
    END;

    -- Insert the HTTP callback log
    BEGIN
        INSERT INTO "callback".http_callback_logs (
            ref_id,
            method,
            path,
            query_params,
            request_headers,
            request_body,
            status_code,
            response_headers,
            response_body,
            client_ip,
            user_agent,
            response_time_ms,
            request_time,
            response_time,
            error_message,
            callback_type,
            partner_ref_id,
            client_number,
            product_code,
            response_code,
            total_amount
        ) VALUES (
            p_ref_id,
            p_method,
            p_path,
            p_query_params,
            v_request_headers_json,
            p_request_body,
            p_status_code,
            v_response_headers_json,
            p_response_body,
            p_client_ip::INET,
            p_user_agent,
            p_response_time_ms,
            p_request_time,
            p_response_time,
            p_error_message,
            p_callback_type,
            p_partner_ref_id,
            p_client_number,
            p_product_code,
            p_response_code,
            p_total_amount
        ) RETURNING id INTO http_callback_log_id;

        error := 0;
        message := 'OK';
        RETURN NEXT;

    EXCEPTION WHEN unique_violation THEN
        http_callback_log_id := NULL;
        error := 7;
        message := 'Ref ID already exists';
        RETURN NEXT;
    WHEN OTHERS THEN
        http_callback_log_id := NULL;
        error := 8;
        message := 'Database error: ' || SQLERRM;
        RETURN NEXT;
    END;
END;
$$;

-- ==========================
-- Triggers
-- ==========================

-- Automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION "callback".set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_set_callback_log_updated_at ON "callback".http_callback_logs;
CREATE TRIGGER trg_set_callback_log_updated_at
BEFORE UPDATE ON "callback".http_callback_logs
FOR EACH ROW
EXECUTE FUNCTION "callback".set_updated_at();

-- ==========================
-- Utility Functions
-- ==========================

-- Function to clean up old callback logs (for maintenance)
CREATE OR REPLACE FUNCTION "callback".cleanup_old_callback_logs(
    p_days_to_keep INTEGER DEFAULT 31
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_deleted_count INTEGER;
BEGIN
    DELETE FROM "callback".http_callback_logs 
    WHERE created_at < NOW() - INTERVAL '1 day' * p_days_to_keep;
    
    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    
    RETURN v_deleted_count;
END;
$$;

-- Function to get callback log statistics
CREATE OR REPLACE FUNCTION "callback".get_callback_log_stats(
    p_start_time TIMESTAMP WITH TIME ZONE DEFAULT NOW() - INTERVAL '1 hour',
    p_end_time TIMESTAMP WITH TIME ZONE DEFAULT NOW()
)
RETURNS TABLE(
    total_callbacks BIGINT,
    success_count BIGINT,
    error_count BIGINT,
    avg_response_time NUMERIC,
    max_response_time BIGINT,
    min_response_time BIGINT,
    most_common_callback_type VARCHAR(50),
    most_common_response_code TEXT,
    most_common_status_code INTEGER
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(*) as total_callbacks,
        COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) as success_count,
        COUNT(*) FILTER (WHERE status_code >= 400) as error_count,
        ROUND(AVG(response_time_ms), 2) as avg_response_time,
        MAX(response_time_ms) as max_response_time,
        MIN(response_time_ms) as min_response_time,
        mode() WITHIN GROUP (ORDER BY callback_type) as most_common_callback_type,
        mode() WITHIN GROUP (ORDER BY response_code) as most_common_response_code,
        mode() WITHIN GROUP (ORDER BY status_code) as most_common_status_code
    FROM "callback".http_callback_logs
    WHERE request_time BETWEEN p_start_time AND p_end_time;
END;
$$;

-- ==========================
-- Indexes
-- ==========================

-- Primary indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_callback_logs_ref_id ON "callback".http_callback_logs(ref_id);
CREATE INDEX IF NOT EXISTS idx_callback_logs_method ON "callback".http_callback_logs(method);
CREATE INDEX IF NOT EXISTS idx_callback_logs_path ON "callback".http_callback_logs(path);
CREATE INDEX IF NOT EXISTS idx_callback_logs_status_code ON "callback".http_callback_logs(status_code);
CREATE INDEX IF NOT EXISTS idx_callback_logs_client_ip ON "callback".http_callback_logs(client_ip);
CREATE INDEX IF NOT EXISTS idx_callback_logs_request_time ON "callback".http_callback_logs(request_time);
CREATE INDEX IF NOT EXISTS idx_callback_logs_response_time ON "callback".http_callback_logs(response_time);
CREATE INDEX IF NOT EXISTS idx_callback_logs_created_at ON "callback".http_callback_logs(created_at);

-- Business logic indexes
CREATE INDEX IF NOT EXISTS idx_callback_logs_callback_type ON "callback".http_callback_logs(callback_type);
CREATE INDEX IF NOT EXISTS idx_callback_logs_partner_ref_id ON "callback".http_callback_logs(partner_ref_id);
CREATE INDEX IF NOT EXISTS idx_callback_logs_ref_id ON "callback".http_callback_logs(ref_id);
CREATE INDEX IF NOT EXISTS idx_callback_logs_client_number ON "callback".http_callback_logs(client_number);
CREATE INDEX IF NOT EXISTS idx_callback_logs_product_code ON "callback".http_callback_logs(product_code);
CREATE INDEX IF NOT EXISTS idx_callback_logs_response_code ON "callback".http_callback_logs(response_code);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_callback_logs_method_path ON "callback".http_callback_logs(method, path);
CREATE INDEX IF NOT EXISTS idx_callback_logs_status_time ON "callback".http_callback_logs(status_code, request_time);
CREATE INDEX IF NOT EXISTS idx_callback_logs_type_time ON "callback".http_callback_logs(callback_type, request_time);
CREATE INDEX IF NOT EXISTS idx_callback_logs_ref_time ON "callback".http_callback_logs(ref_id, request_time);
CREATE INDEX IF NOT EXISTS idx_callback_logs_partner_ref_time ON "callback".http_callback_logs(partner_ref_id, request_time);

-- Performance optimization for cleanup queries
-- Note: Cannot use NOW() in index predicate as it's not immutable
-- Instead, create a regular index on created_at for cleanup queries
CREATE INDEX IF NOT EXISTS idx_callback_logs_created_at_for_cleanup ON "callback".http_callback_logs(created_at);

-- ==========================
-- Comments
-- ==========================

COMMENT ON SCHEMA "callback" IS 'Schema for HTTP callback logging and tracking';
COMMENT ON TABLE "callback".http_callback_logs IS 'Logs all HTTP callback requests and responses with business context';
COMMENT ON COLUMN "callback".http_callback_logs.callback_type IS 'Type of callback: payment_status, inquiry_result, etc.';
COMMENT ON COLUMN "callback".http_callback_logs.ref_id IS 'Transaction reference ID (not unique, allows multiple callbacks per transaction)';
COMMENT ON COLUMN "callback".http_callback_logs.partner_ref_id IS 'Partner system reference ID';
COMMENT ON COLUMN "callback".http_callback_logs.response_code IS 'Business logic response code (01, 12, etc.)';

-- ==========================
-- Usage Examples
-- ==========================

-- Insert callback log
SELECT "callback".http_callback_log_oninsert(
    'REF-456', 'POST', '/callback/payment', 'param=value',
    '{"Content-Type":"application/json"}', '{"status":"success"}',
    200, '{"Content-Type":"application/json"}', '{"code":"01"}',
    '192.168.1.100', 'Mozilla/5.0', 150,
    NOW(), NOW(), NULL,
    'payment_status', 'PPS-123', '08123456789', 'PLN', '01', 50000.00
);

-- Cleanup logs lama (30 hari)
SELECT "callback".cleanup_old_callback_logs(30);

-- Lihat statistik
SELECT * FROM "callback".get_callback_log_stats();
