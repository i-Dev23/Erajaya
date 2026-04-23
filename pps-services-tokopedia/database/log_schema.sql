-- HTTP Logging Schema for PostgreSQL
-- This schema creates tables and stored procedures for HTTP request/response logging

-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS log;

-- HTTP Logs table
CREATE TABLE IF NOT EXISTS log.http_logs (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(50) NOT NULL UNIQUE,
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
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_http_logs_request_id ON log.http_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_http_logs_method ON log.http_logs(method);
CREATE INDEX IF NOT EXISTS idx_http_logs_path ON log.http_logs(path);
CREATE INDEX IF NOT EXISTS idx_http_logs_status_code ON log.http_logs(status_code);
CREATE INDEX IF NOT EXISTS idx_http_logs_client_ip ON log.http_logs(client_ip);
CREATE INDEX IF NOT EXISTS idx_http_logs_request_time ON log.http_logs(request_time);
CREATE INDEX IF NOT EXISTS idx_http_logs_response_time ON log.http_logs(response_time);
CREATE INDEX IF NOT EXISTS idx_http_logs_created_at ON log.http_logs(created_at);

-- Composite index for common queries
CREATE INDEX IF NOT EXISTS idx_http_logs_method_path ON log.http_logs(method, path);
CREATE INDEX IF NOT EXISTS idx_http_logs_status_time ON log.http_logs(status_code, request_time);

-- Stored procedure for inserting HTTP logs
CREATE OR REPLACE FUNCTION log.http_log_oninsert(
    p_request_id VARCHAR(50),
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
    p_error_message TEXT DEFAULT NULL
)
RETURNS RECORD
LANGUAGE plpgsql
AS $$
DECLARE
    v_http_log_id BIGINT;
    v_error_code INTEGER := 0;
    v_message TEXT := 'Success';
    v_request_headers_json JSONB;
    v_response_headers_json JSONB;
BEGIN
    -- Validate input parameters
    IF p_request_id IS NULL OR p_request_id = '' THEN
        v_error_code := 1;
        v_message := 'Request ID is required';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END IF;

    IF p_method IS NULL OR p_method = '' THEN
        v_error_code := 2;
        v_message := 'HTTP method is required';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END IF;

    IF p_path IS NULL OR p_path = '' THEN
        v_error_code := 3;
        v_message := 'Path is required';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END IF;

    IF p_status_code IS NULL OR p_status_code < 100 OR p_status_code > 599 THEN
        v_error_code := 4;
        v_message := 'Valid status code is required';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END IF;

    -- Parse JSON headers
    BEGIN
        IF p_request_headers IS NOT NULL AND p_request_headers != '' THEN
            v_request_headers_json := p_request_headers::JSONB;
        ELSE
            v_request_headers_json := '{}'::JSONB;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        v_error_code := 5;
        v_message := 'Invalid request headers JSON format';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END;

    BEGIN
        IF p_response_headers IS NOT NULL AND p_response_headers != '' THEN
            v_response_headers_json := p_response_headers::JSONB;
        ELSE
            v_response_headers_json := '{}'::JSONB;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        v_error_code := 6;
        v_message := 'Invalid response headers JSON format';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END;

    -- Insert the HTTP log
    BEGIN
        INSERT INTO log.http_logs (
            request_id,
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
            error_message
        ) VALUES (
            p_request_id,
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
            p_error_message
        ) RETURNING id INTO v_http_log_id;

        -- Return success
        RETURN (v_http_log_id, v_error_code, v_message);

    EXCEPTION WHEN unique_violation THEN
        v_error_code := 7;
        v_message := 'Request ID already exists';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    WHEN OTHERS THEN
        v_error_code := 8;
        v_message := 'Database error: ' || SQLERRM;
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END;
END;
$$;

-- Function to clean up old HTTP logs (for maintenance)
CREATE OR REPLACE FUNCTION log.cleanup_old_http_logs(
    p_days_to_keep INTEGER DEFAULT 31
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_deleted_count INTEGER;
BEGIN
    DELETE FROM log.http_logs 
    WHERE created_at < NOW() - INTERVAL '1 day' * p_days_to_keep;
    
    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    
    RETURN v_deleted_count;
END;
$$;

-- Function to get HTTP log statistics
CREATE OR REPLACE FUNCTION log.get_http_log_stats(
    p_start_time TIMESTAMP WITH TIME ZONE DEFAULT NOW() - INTERVAL '1 hour',
    p_end_time TIMESTAMP WITH TIME ZONE DEFAULT NOW()
)
RETURNS TABLE(
    total_requests BIGINT,
    success_count BIGINT,
    error_count BIGINT,
    avg_response_time NUMERIC,
    max_response_time BIGINT,
    min_response_time BIGINT,
    most_common_method VARCHAR(10),
    most_common_path VARCHAR(500),
    most_common_status_code INTEGER
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(*) as total_requests,
        COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) as success_count,
        COUNT(*) FILTER (WHERE status_code >= 400) as error_count,
        ROUND(AVG(response_time_ms), 2) as avg_response_time,
        MAX(response_time_ms) as max_response_time,
        MIN(response_time_ms) as min_response_time,
        mode() WITHIN GROUP (ORDER BY method) as most_common_method,
        mode() WITHIN GROUP (ORDER BY path) as most_common_path,
        mode() WITHIN GROUP (ORDER BY status_code) as most_common_status_code
    FROM log.http_logs
    WHERE request_time BETWEEN p_start_time AND p_end_time;
END;
$$;

-- Grant permissions (adjust as needed for your environment)
-- GRANT USAGE ON SCHEMA log TO your_app_user;
-- GRANT SELECT, INSERT, UPDATE, DELETE ON log.http_logs TO your_app_user;
-- GRANT USAGE, SELECT ON SEQUENCE log.http_logs_id_seq TO your_app_user;
-- GRANT EXECUTE ON FUNCTION log.http_log_oninsert TO your_app_user;
-- GRANT EXECUTE ON FUNCTION log.cleanup_old_http_logs TO your_app_user;
-- GRANT EXECUTE ON FUNCTION log.get_http_log_stats TO your_app_user;
