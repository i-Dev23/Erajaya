--
-- PostgreSQL database dump
--

\restrict G8FvpzaXQb5WMEUuKk6PqC6U33gdRI64njSUomArhMaUOdsckP7z7jxi3zEOSmi

-- Dumped from database version 15.14 (Debian 15.14-1.pgdg13+1)
-- Dumped by pg_dump version 15.14 (Debian 15.14-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: callback; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA "callback";


--
-- Name: SCHEMA "callback"; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA "callback" IS 'Schema for HTTP callback logging and tracking';


--
-- Name: inquiry; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA "inquiry";


--
-- Name: log; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA "log";


--
-- Name: maintenance; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA "maintenance";


--
-- Name: mapping; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA "mapping";


--
-- Name: payment; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA "payment";


--
-- Name: cleanup_old_callback_logs(integer); Type: FUNCTION; Schema: callback; Owner: -
--

CREATE FUNCTION "callback"."cleanup_old_callback_logs"("p_days_to_keep" integer DEFAULT 31) RETURNS integer
    LANGUAGE "plpgsql"
    AS '
DECLARE
    v_deleted_count INTEGER;
BEGIN
    DELETE FROM "callback".http_callback_logs 
    WHERE created_at < NOW() - INTERVAL ''1 day'' * p_days_to_keep;
    
    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    
    RETURN v_deleted_count;
END;
';


--
-- Name: get_callback_log_stats(timestamp with time zone, timestamp with time zone); Type: FUNCTION; Schema: callback; Owner: -
--

CREATE FUNCTION "callback"."get_callback_log_stats"("p_start_time" timestamp with time zone DEFAULT ("now"() - '01:00:00'::interval), "p_end_time" timestamp with time zone DEFAULT "now"()) RETURNS TABLE("total_callbacks" bigint, "success_count" bigint, "error_count" bigint, "avg_response_time" numeric, "max_response_time" bigint, "min_response_time" bigint, "most_common_callback_type" character varying, "most_common_response_code" "text", "most_common_status_code" integer)
    LANGUAGE "plpgsql"
    AS '
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
';


--
-- Name: http_callback_log_oninsert(character varying, character varying, character varying, "text", "text", "text", integer, "text", "text", "inet", "text", bigint, timestamp with time zone, timestamp with time zone, "text", character varying, "text", "text", "text", "text", numeric); Type: FUNCTION; Schema: callback; Owner: -
--

CREATE FUNCTION "callback"."http_callback_log_oninsert"("p_ref_id" character varying, "p_method" character varying, "p_path" character varying, "p_query_params" "text", "p_request_headers" "text", "p_request_body" "text", "p_status_code" integer, "p_response_headers" "text", "p_response_body" "text", "p_client_ip" "inet", "p_user_agent" "text", "p_response_time_ms" bigint, "p_request_time" timestamp with time zone, "p_response_time" timestamp with time zone, "p_error_message" "text", "p_callback_type" character varying, "p_partner_ref_id" "text", "p_client_number" "text", "p_product_code" "text", "p_response_code" "text", "p_total_amount" numeric) RETURNS TABLE("http_callback_log_id" bigint, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
DECLARE
    v_request_headers_json JSONB;
    v_response_headers_json JSONB;
BEGIN
    -- Validate input parameters
    IF p_ref_id IS NULL OR p_ref_id = '''' THEN
        http_callback_log_id := NULL;
        error := 1;
        message := ''Ref ID is required'';
        RETURN NEXT;
        RETURN;
    END IF;

    IF p_method IS NULL OR p_method = '''' THEN
        http_callback_log_id := NULL;
        error := 2;
        message := ''HTTP method is required'';
        RETURN NEXT;
        RETURN;
    END IF;

    IF p_path IS NULL OR p_path = '''' THEN
        http_callback_log_id := NULL;
        error := 3;
        message := ''Path is required'';
        RETURN NEXT;
        RETURN;
    END IF;

    IF p_status_code IS NULL OR p_status_code < 100 OR p_status_code > 599 THEN
        http_callback_log_id := NULL;
        error := 4;
        message := ''Valid status code is required'';
        RETURN NEXT;
        RETURN;
    END IF;

    -- Parse JSON headers
    BEGIN
        IF p_request_headers IS NOT NULL AND p_request_headers != '''' THEN
            v_request_headers_json := p_request_headers::JSONB;
        ELSE
            v_request_headers_json := ''{}''::JSONB;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        http_callback_log_id := NULL;
        error := 5;
        message := ''Invalid request headers JSON format'';
        RETURN NEXT;
        RETURN;
    END;

    BEGIN
        IF p_response_headers IS NOT NULL AND p_response_headers != '''' THEN
            v_response_headers_json := p_response_headers::JSONB;
        ELSE
            v_response_headers_json := ''{}''::JSONB;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        http_callback_log_id := NULL;
        error := 6;
        message := ''Invalid response headers JSON format'';
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
        message := ''OK'';
        RETURN NEXT;

    EXCEPTION WHEN unique_violation THEN
        http_callback_log_id := NULL;
        error := 7;
        message := ''Ref ID already exists'';
        RETURN NEXT;
    WHEN OTHERS THEN
        http_callback_log_id := NULL;
        error := 8;
        message := ''Database error: '' || SQLERRM;
        RETURN NEXT;
    END;
END;
';


--
-- Name: set_updated_at(); Type: FUNCTION; Schema: callback; Owner: -
--

CREATE FUNCTION "callback"."set_updated_at"() RETURNS "trigger"
    LANGUAGE "plpgsql"
    AS '
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
';


--
-- Name: bill_detail_oninsert("text", "text", "text", boolean, boolean); Type: FUNCTION; Schema: inquiry; Owner: -
--

CREATE FUNCTION "inquiry"."bill_detail_oninsert"("p_pps_inquiry_id" "text", "p_name" "text", "p_value" "text", "p_is_pii" boolean, "p_is_show" boolean) RETURNS TABLE("bill_detail_id" bigint, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
BEGIN
    INSERT INTO "inquiry".bill_details(
        pps_inquiry_id, name, value, is_pii, is_show
    ) VALUES (
        p_pps_inquiry_id, p_name, p_value, p_is_pii, p_is_show
    ) RETURNING id INTO bill_detail_id;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    bill_detail_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
';


--
-- Name: cleanup_old_inquiry_logs(integer); Type: FUNCTION; Schema: inquiry; Owner: -
--

CREATE FUNCTION "inquiry"."cleanup_old_inquiry_logs"("p_days_to_keep" integer DEFAULT 31) RETURNS integer
    LANGUAGE "plpgsql"
    AS '
DECLARE
    v_deleted_count INTEGER := 0;
BEGIN
    -- Delete old bill_details whose parent inquiry_responses are old
    DELETE FROM "inquiry".bill_details bd
    USING "inquiry".inquiry_responses ir
    WHERE bd.pps_inquiry_id = ir.pps_inquiry_id
      AND ir.created_at < NOW() - INTERVAL ''1 day'' * p_days_to_keep;

    -- Delete old inquiry_responses
    DELETE FROM "inquiry".inquiry_responses ir
    WHERE ir.created_at < NOW() - INTERVAL ''1 day'' * p_days_to_keep;

    -- Delete orphaned inquiry_requests that have no responses and are old
    DELETE FROM "inquiry".inquiry_requests iq
    WHERE iq.created_at < NOW() - INTERVAL ''1 day'' * p_days_to_keep
      AND NOT EXISTS (
        SELECT 1 FROM "inquiry".inquiry_responses ir
        WHERE ir.inquiry_request_id = iq.id
      );

    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    RETURN v_deleted_count;
END;
';


--
-- Name: inquiry_request_oninsert("text", "text", "text", "text", "text", timestamp with time zone); Type: FUNCTION; Schema: inquiry; Owner: -
--

CREATE FUNCTION "inquiry"."inquiry_request_oninsert"("p_ref_id" "text", "p_client_number" "text", "p_category" "text", "p_rsid" "text", "p_product_code" "text", "p_timestamp" timestamp with time zone) RETURNS TABLE("inquiry_request_id" bigint, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
BEGIN
    INSERT INTO "inquiry".inquiry_requests(
        ref_id, client_number, category, rsid, product_code, ts
    ) VALUES (
        p_ref_id, p_client_number, p_category, p_rsid, p_product_code, p_timestamp
    ) RETURNING id INTO inquiry_request_id;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    inquiry_request_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
';


--
-- Name: inquiry_response_oninsert(bigint, "text", "text", "text", "text", "text", numeric, timestamp with time zone, integer); Type: FUNCTION; Schema: inquiry; Owner: -
--

CREATE FUNCTION "inquiry"."inquiry_response_oninsert"("p_inquiry_request_id" bigint, "p_pps_inquiry_id" "text", "p_client_number" "text", "p_product_code" "text", "p_response_code" "text", "p_message" "text", "p_total_amount" numeric, "p_timestamp" timestamp with time zone, "p_bill_count" integer) RETURNS TABLE("inquiry_response_id" bigint, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
BEGIN
    INSERT INTO "inquiry".inquiry_responses(
        inquiry_request_id, pps_inquiry_id, client_number, product_code,
        response_code, message, total_amount, ts, bill_count
    ) VALUES (
        p_inquiry_request_id, p_pps_inquiry_id, p_client_number, p_product_code,
        p_response_code, p_message, p_total_amount, p_timestamp, p_bill_count
    ) RETURNING id INTO inquiry_response_id;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    inquiry_response_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
';


--
-- Name: cleanup_old_http_logs(integer); Type: FUNCTION; Schema: log; Owner: -
--

CREATE FUNCTION "log"."cleanup_old_http_logs"("p_days_to_keep" integer DEFAULT 31) RETURNS integer
    LANGUAGE "plpgsql"
    AS '
DECLARE
    v_deleted_count INTEGER;
BEGIN
    DELETE FROM log.http_logs 
    WHERE created_at < NOW() - INTERVAL ''1 day'' * p_days_to_keep;
    
    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    
    RETURN v_deleted_count;
END;
';


--
-- Name: get_http_log_stats(timestamp with time zone, timestamp with time zone); Type: FUNCTION; Schema: log; Owner: -
--

CREATE FUNCTION "log"."get_http_log_stats"("p_start_time" timestamp with time zone DEFAULT ("now"() - '01:00:00'::interval), "p_end_time" timestamp with time zone DEFAULT "now"()) RETURNS TABLE("total_requests" bigint, "success_count" bigint, "error_count" bigint, "avg_response_time" numeric, "max_response_time" bigint, "min_response_time" bigint, "most_common_method" character varying, "most_common_path" character varying, "most_common_status_code" integer)
    LANGUAGE "plpgsql"
    AS '
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
';


--
-- Name: http_log_oninsert(character varying, character varying, character varying, "text", "text", "text", integer, "text", "text", "inet", "text", bigint, timestamp with time zone, timestamp with time zone, "text"); Type: FUNCTION; Schema: log; Owner: -
--

CREATE FUNCTION "log"."http_log_oninsert"("p_request_id" character varying, "p_method" character varying, "p_path" character varying, "p_query_params" "text", "p_request_headers" "text", "p_request_body" "text", "p_status_code" integer, "p_response_headers" "text", "p_response_body" "text", "p_client_ip" "inet", "p_user_agent" "text", "p_response_time_ms" bigint, "p_request_time" timestamp with time zone, "p_response_time" timestamp with time zone, "p_error_message" "text" DEFAULT NULL::"text") RETURNS "record"
    LANGUAGE "plpgsql"
    AS '
DECLARE
    v_http_log_id BIGINT;
    v_error_code INTEGER := 0;
    v_message TEXT := ''Success'';
    v_request_headers_json JSONB;
    v_response_headers_json JSONB;
BEGIN
    -- Validate input parameters
    IF p_request_id IS NULL OR p_request_id = '''' THEN
        v_error_code := 1;
        v_message := ''Request ID is required'';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END IF;

    IF p_method IS NULL OR p_method = '''' THEN
        v_error_code := 2;
        v_message := ''HTTP method is required'';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END IF;

    IF p_path IS NULL OR p_path = '''' THEN
        v_error_code := 3;
        v_message := ''Path is required'';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END IF;

    IF p_status_code IS NULL OR p_status_code < 100 OR p_status_code > 599 THEN
        v_error_code := 4;
        v_message := ''Valid status code is required'';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END IF;

    -- Parse JSON headers
    BEGIN
        IF p_request_headers IS NOT NULL AND p_request_headers != '''' THEN
            v_request_headers_json := p_request_headers::JSONB;
        ELSE
            v_request_headers_json := ''{}''::JSONB;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        v_error_code := 5;
        v_message := ''Invalid request headers JSON format'';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END;

    BEGIN
        IF p_response_headers IS NOT NULL AND p_response_headers != '''' THEN
            v_response_headers_json := p_response_headers::JSONB;
        ELSE
            v_response_headers_json := ''{}''::JSONB;
        END IF;
    EXCEPTION WHEN OTHERS THEN
        v_error_code := 6;
        v_message := ''Invalid response headers JSON format'';
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
        v_message := ''Request ID already exists'';
        RETURN (NULL::BIGINT, v_error_code, v_message);
    WHEN OTHERS THEN
        v_error_code := 8;
        v_message := ''Database error: '' || SQLERRM;
        RETURN (NULL::BIGINT, v_error_code, v_message);
    END;
END;
';


--
-- Name: create_month_partition("regclass", "date"); Type: FUNCTION; Schema: maintenance; Owner: -
--

CREATE FUNCTION "maintenance"."create_month_partition"("p_parent_table" "regclass", "p_from" "date") RETURNS "void"
    LANGUAGE "plpgsql"
    AS '
DECLARE
    v_parent TEXT := p_parent_table::TEXT; -- e.g., ''log.http_logs''
    v_to DATE := (date_trunc(''month'', p_from) + INTERVAL ''1 month'')::DATE;
    v_suffix TEXT := to_char(p_from, ''YYYYMM'');
    v_child TEXT;
    v_created BOOLEAN := FALSE;
BEGIN
    v_child := v_parent || ''_'' || v_suffix;

    -- Ensure parent is partitioned
    IF NOT EXISTS (
        SELECT 1
        FROM pg_partitioned_table pt
        JOIN pg_class c ON c.oid = pt.partrelid
        WHERE c.oid = p_parent_table
    ) THEN
        -- Parent is not partitioned; do nothing
        RETURN;
    END IF;

    -- Create child partition if not exists
    IF NOT EXISTS (
        SELECT 1 FROM pg_class WHERE relname = split_part(v_child, ''.'', 2) AND relnamespace = (split_part(v_child, ''.'', 1))::regnamespace
    ) THEN
        EXECUTE format(
            ''CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%L) TO (%L)'',
            v_child,
            v_parent,
            p_from,
            v_to
        );
        v_created := TRUE;
    END IF;

    -- Optionally, create indexes matching parent (inherits automatically for PK/UNIQUE if defined on parent)
    -- Add any per-partition index here if needed.

    IF v_created THEN
        RAISE NOTICE ''Created partition % for range [% - %)'', v_child, p_from, v_to;
    END IF;
END;
';


--
-- Name: ensure_monthly_partitions(integer); Type: FUNCTION; Schema: maintenance; Owner: -
--

CREATE FUNCTION "maintenance"."ensure_monthly_partitions"("p_months_ahead" integer DEFAULT 2) RETURNS "void"
    LANGUAGE "plpgsql"
    AS '
DECLARE
    v_month DATE := date_trunc(''month'', now())::DATE;
    i INT := 0;
BEGIN
    -- Create for current + ahead months for each parent table
    FOR i IN 0..p_months_ahead LOOP
        PERFORM maintenance.create_month_partition(''log.http_logs''::regclass, (v_month + (i || '' months'')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition(''callback.http_callback_logs''::regclass, (v_month + (i || '' months'')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition(''inquiry.inquiry_requests''::regclass, (v_month + (i || '' months'')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition(''inquiry.inquiry_responses''::regclass, (v_month + (i || '' months'')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition(''payment.payment_requests''::regclass, (v_month + (i || '' months'')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition(''payment.payment_responses''::regclass, (v_month + (i || '' months'')::INTERVAL)::DATE);
        -- Additional bill detail tables (safe no-op if parents are not partitioned yet)
        PERFORM maintenance.create_month_partition(''inquiry.bill_details''::regclass, (v_month + (i || '' months'')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition(''payment.payment_bill_details''::regclass, (v_month + (i || '' months'')::INTERVAL)::DATE);
    END LOOP;
END;
';


--
-- Name: get_error_message_mapping("text", "text"); Type: FUNCTION; Schema: mapping; Owner: -
--

CREATE FUNCTION "mapping"."get_error_message_mapping"("p_system_type" "text", "p_error_message" "text") RETURNS TABLE("response_code" "text", "description" "text", "found" boolean)
    LANGUAGE "plpgsql"
    AS '
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
        RETURN QUERY SELECT v_response_code, v_description, TRUE;
        RETURN;
    END IF;

    RETURN;
END;
';


--
-- Name: cleanup_old_payment_logs(integer); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."cleanup_old_payment_logs"("p_days_to_keep" integer DEFAULT 31) RETURNS integer
    LANGUAGE "plpgsql"
    AS '
DECLARE
    v_deleted_count INTEGER := 0;
BEGIN
    -- Delete old payment_bill_details whose parent responses are old
    DELETE FROM "payment".payment_bill_details pbd
    USING "payment".payment_responses pr
    WHERE pbd.partner_ref_id = pr.partner_ref_id
      AND pr.created_at < NOW() - INTERVAL ''1 day'' * p_days_to_keep;

    -- Delete old payment_responses
    DELETE FROM "payment".payment_responses pr
    WHERE pr.created_at < NOW() - INTERVAL ''1 day'' * p_days_to_keep;

    -- Delete orphaned payment_requests that have no responses and are old
    DELETE FROM "payment".payment_requests pq
    WHERE pq.created_at < NOW() - INTERVAL ''1 day'' * p_days_to_keep
      AND NOT EXISTS (
        SELECT 1 FROM "payment".payment_responses pr
        WHERE pr.payment_request_id = pq.id
      );

    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    RETURN v_deleted_count;
END;
';


--
-- Name: get_daily_reconciliation_report("date"); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."get_daily_reconciliation_report"("p_report_date" "date" DEFAULT (CURRENT_DATE - '1 day'::interval)) RETURNS TABLE("timestamp_col" "text", "ref_id" "text", "client_number" "text", "client_name" "text", "tarif_daya" "text", "rp_token" "text", "jumlah_kwh" "text", "nomor_token" "text", "amount" numeric, "sales_price" numeric, "partner_ref_id" "text")
    LANGUAGE "plpgsql"
    AS '
BEGIN
    RETURN QUERY
    SELECT
        TO_CHAR(pr.created_at, ''YYYY-MM-DD HH24:MI:SS'') AS timestamp_col,
        pr.ref_id,
        pr.client_number,
        MAX(CASE WHEN pbd.name = ''Nama'' THEN pbd.value END) AS client_name,
        MAX(CASE WHEN pbd.name = ''Tarif/Daya'' THEN pbd.value END) AS tarif_daya,
        MAX(CASE WHEN pbd.name = ''Rp Stroom/Token'' THEN pbd.value END) AS rp_token,
        MAX(CASE WHEN pbd.name = ''kWh'' THEN pbd.value END) AS jumlah_kwh,
        MAX(CASE WHEN pbd.name = ''Token'' THEN pbd.value END) AS nomor_token,
        resp.total_amount AS amount,
        resp.sales_price,
        resp.partner_ref_id
    FROM payment.payment_requests pr
    JOIN payment.payment_responses resp
        ON resp.payment_request_id = pr.id
    LEFT JOIN payment.payment_bill_details pbd
        ON pbd.partner_ref_id = resp.partner_ref_id
        AND pbd.name IN (''Nama'', ''Tarif/Daya'', ''Rp Stroom/Token'', ''kWh'', ''Token'')
    WHERE resp.response_code = ''00''
      AND pr.created_at >= p_report_date::date
      AND pr.created_at < p_report_date::date + INTERVAL ''1 day''
    GROUP BY pr.id, pr.ref_id, pr.client_number, pr.created_at, 
             resp.total_amount, resp.sales_price, resp.partner_ref_id
    ORDER BY pr.created_at DESC;
END;
';


--
-- Name: payment_bill_detail_oninsert("text", "text", "text", boolean, boolean); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."payment_bill_detail_oninsert"("p_partner_ref_id" "text", "p_name" "text", "p_value" "text", "p_is_pii" boolean, "p_is_show" boolean) RETURNS TABLE("payment_bill_detail_id" bigint, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
BEGIN
	INSERT INTO "payment".payment_bill_details(
        partner_ref_id, name, value, is_pii, is_show
    ) VALUES (
        p_partner_ref_id, p_name, p_value, p_is_pii, p_is_show
    ) RETURNING id INTO payment_bill_detail_id;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    payment_bill_detail_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
';


--
-- Name: payment_request_oninsert("text", "text", "text", "text", "text", "text", numeric, timestamp with time zone); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."payment_request_oninsert"("p_ref_id" "text", "p_partner_inquiry_id" "text", "p_client_number" "text", "p_category" "text", "p_rsid" "text", "p_product_code" "text", "p_total_amount" numeric, "p_timestamp" timestamp with time zone) RETURNS TABLE("payment_request_id" bigint, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
BEGIN
    INSERT INTO "payment".payment_requests(
        ref_id, partner_inquiry_id, client_number, category, rsid, product_code, total_amount, ts
    ) VALUES (
        p_ref_id, p_partner_inquiry_id, p_client_number, p_category, p_rsid, p_product_code, p_total_amount, p_timestamp
    ) RETURNING id INTO payment_request_id;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    payment_request_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
';


--
-- Name: payment_response_oninsert(bigint, "text", "text", "text", "text", "text", numeric, numeric, timestamp with time zone, integer); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."payment_response_oninsert"("p_payment_request_id" bigint, "p_partner_ref_id" "text", "p_client_number" "text", "p_product_code" "text", "p_response_code" "text", "p_message" "text", "p_admin_fee" numeric, "p_total_amount" numeric, "p_timestamp" timestamp with time zone, "p_bill_count" integer) RETURNS TABLE("payment_response_id" bigint, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
BEGIN
    INSERT INTO "payment".payment_responses(
        payment_request_id, partner_ref_id, client_number, product_code,
        response_code, message, admin_fee, total_amount, ts, bill_count
    ) VALUES (
        p_payment_request_id, p_partner_ref_id, p_client_number, p_product_code,
        p_response_code, p_message, p_admin_fee, p_total_amount, p_timestamp, p_bill_count
    ) RETURNING id INTO payment_response_id;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
    payment_response_id := NULL;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
';


--
-- Name: payment_status_onupdate("text", integer); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."payment_status_onupdate"("p_ref_id" "text", "p_status_code" integer) RETURNS TABLE("partnerrefid" character varying, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
DECLARE
	vStatusMessage varchar;
	vResponseCode varchar;
BEGIN

	if p_status_code = 0 then
		vStatusMessage := ''Success'';
		vResponseCode := ''00'';
	elsif p_status_code = 1 then
		vStatusMessage := ''Failed'';
		vResponseCode := ''02'';
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
		RAISE ''Payment not found'';
	END IF;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
	partnerrefid := null;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
';


--
-- Name: payment_status_onupdate("text", character varying, character varying); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."payment_status_onupdate"("p_ref_id" "text", "p_responsecode" character varying, "p_responsemsg" character varying) RETURNS TABLE("partnerrefid" character varying, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
BEGIN

	--raise exception ''Test error'';

	UPDATE "payment".payment_responses
	SET response_code = p_responsecode,
	"message" = p_responsemsg
	WHERE payment_request_id = (
		SELECT id FROM "payment".payment_requests pq
		WHERE pq.ref_id = p_ref_id
		LIMIT 1
	) returning partner_ref_id into partnerrefid;
	
	IF partnerrefid IS NULL THEN
		RAISE ''Payment not found'';
	END IF;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
	partnerrefid := null;
    error := 1;
    message := SQLERRM;
    RETURN NEXT;
END;
';


--
-- Name: payment_status_onupdate_with_sales_price("text", character varying, character varying, numeric); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."payment_status_onupdate_with_sales_price"("p_ref_id" "text", "p_responsecode" character varying, "p_responsemsg" character varying, "p_sales_price" numeric) RETURNS TABLE("partnerrefid" character varying, "error" integer, "message" "text")
    LANGUAGE "plpgsql"
    AS '
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
        RAISE ''Payment not found'';
    END IF;

    error := 0;
    message := ''OK'';
    RETURN NEXT;
EXCEPTION WHEN others THEN
        partnerrefid := null;
        error := 1;
        message := SQLERRM;
        RETURN NEXT;
END;
';


--
-- Name: set_response_updated_at(); Type: FUNCTION; Schema: payment; Owner: -
--

CREATE FUNCTION "payment"."set_response_updated_at"() RETURNS "trigger"
    LANGUAGE "plpgsql"
    AS '
BEGIN
    IF (NEW.response_code IS DISTINCT FROM OLD.response_code)
        OR (NEW.message IS DISTINCT FROM OLD.message) THEN
        NEW.response_updated_at := now();
    END IF;
    RETURN NEW;
END;
';


SET default_tablespace = '';

SET default_table_access_method = "heap";

--
-- Name: http_callback_logs; Type: TABLE; Schema: callback; Owner: -
--

CREATE TABLE "callback"."http_callback_logs" (
    "id" bigint NOT NULL,
    "ref_id" character varying(50) NOT NULL,
    "method" character varying(10) NOT NULL,
    "path" character varying(500) NOT NULL,
    "query_params" "text",
    "request_headers" "jsonb",
    "request_body" "text",
    "status_code" integer NOT NULL,
    "response_headers" "jsonb",
    "response_body" "text",
    "client_ip" "inet",
    "user_agent" "text",
    "response_time_ms" bigint NOT NULL,
    "request_time" timestamp with time zone NOT NULL,
    "response_time" timestamp with time zone NOT NULL,
    "error_message" "text",
    "callback_type" character varying(50),
    "partner_ref_id" "text",
    "client_number" "text",
    "product_code" "text",
    "response_code" "text",
    "total_amount" numeric(18,2),
    "created_at" timestamp with time zone DEFAULT "now"(),
    "updated_at" timestamp with time zone DEFAULT "now"()
);


--
-- Name: TABLE "http_callback_logs"; Type: COMMENT; Schema: callback; Owner: -
--

COMMENT ON TABLE "callback"."http_callback_logs" IS 'Logs all HTTP callback requests and responses with business context';


--
-- Name: COLUMN "http_callback_logs"."ref_id"; Type: COMMENT; Schema: callback; Owner: -
--

COMMENT ON COLUMN "callback"."http_callback_logs"."ref_id" IS 'Transaction reference ID (not unique, allows multiple callbacks per transaction)';


--
-- Name: COLUMN "http_callback_logs"."callback_type"; Type: COMMENT; Schema: callback; Owner: -
--

COMMENT ON COLUMN "callback"."http_callback_logs"."callback_type" IS 'Type of callback: payment_status, inquiry_result, etc.';


--
-- Name: COLUMN "http_callback_logs"."partner_ref_id"; Type: COMMENT; Schema: callback; Owner: -
--

COMMENT ON COLUMN "callback"."http_callback_logs"."partner_ref_id" IS 'Partner system reference ID';


--
-- Name: COLUMN "http_callback_logs"."response_code"; Type: COMMENT; Schema: callback; Owner: -
--

COMMENT ON COLUMN "callback"."http_callback_logs"."response_code" IS 'Business logic response code (01, 12, etc.)';


--
-- Name: http_callback_logs_id_seq; Type: SEQUENCE; Schema: callback; Owner: -
--

CREATE SEQUENCE "callback"."http_callback_logs_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: http_callback_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: callback; Owner: -
--

ALTER SEQUENCE "callback"."http_callback_logs_id_seq" OWNED BY "callback"."http_callback_logs"."id";


--
-- Name: bill_details; Type: TABLE; Schema: inquiry; Owner: -
--

CREATE TABLE "inquiry"."bill_details" (
    "id" bigint NOT NULL,
    "pps_inquiry_id" "text" NOT NULL,
    "name" "text" NOT NULL,
    "value" "text",
    "is_pii" boolean DEFAULT false NOT NULL,
    "is_show" boolean DEFAULT true NOT NULL,
    "created_at" timestamp with time zone DEFAULT "now"() NOT NULL
);


--
-- Name: bill_details_id_seq; Type: SEQUENCE; Schema: inquiry; Owner: -
--

CREATE SEQUENCE "inquiry"."bill_details_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bill_details_id_seq; Type: SEQUENCE OWNED BY; Schema: inquiry; Owner: -
--

ALTER SEQUENCE "inquiry"."bill_details_id_seq" OWNED BY "inquiry"."bill_details"."id";


--
-- Name: inquiry_requests; Type: TABLE; Schema: inquiry; Owner: -
--

CREATE TABLE "inquiry"."inquiry_requests" (
    "id" bigint NOT NULL,
    "ref_id" "text",
    "client_number" "text",
    "category" "text" NOT NULL,
    "rsid" "text" NOT NULL,
    "product_code" "text" NOT NULL,
    "ts" timestamp with time zone NOT NULL,
    "created_at" timestamp with time zone DEFAULT "now"() NOT NULL
);


--
-- Name: inquiry_requests_id_seq; Type: SEQUENCE; Schema: inquiry; Owner: -
--

CREATE SEQUENCE "inquiry"."inquiry_requests_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: inquiry_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: inquiry; Owner: -
--

ALTER SEQUENCE "inquiry"."inquiry_requests_id_seq" OWNED BY "inquiry"."inquiry_requests"."id";


--
-- Name: inquiry_responses; Type: TABLE; Schema: inquiry; Owner: -
--

CREATE TABLE "inquiry"."inquiry_responses" (
    "id" bigint NOT NULL,
    "inquiry_request_id" bigint,
    "pps_inquiry_id" "text",
    "client_number" "text",
    "product_code" "text" NOT NULL,
    "response_code" "text" NOT NULL,
    "message" "text",
    "total_amount" numeric(18,2) DEFAULT 0 NOT NULL,
    "ts" timestamp with time zone NOT NULL,
    "bill_count" integer DEFAULT 0 NOT NULL,
    "created_at" timestamp with time zone DEFAULT "now"() NOT NULL
);


--
-- Name: inquiry_responses_id_seq; Type: SEQUENCE; Schema: inquiry; Owner: -
--

CREATE SEQUENCE "inquiry"."inquiry_responses_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: inquiry_responses_id_seq; Type: SEQUENCE OWNED BY; Schema: inquiry; Owner: -
--

ALTER SEQUENCE "inquiry"."inquiry_responses_id_seq" OWNED BY "inquiry"."inquiry_responses"."id";


--
-- Name: http_logs; Type: TABLE; Schema: log; Owner: -
--

CREATE TABLE "log"."http_logs" (
    "id" bigint NOT NULL,
    "request_id" character varying(50) NOT NULL,
    "method" character varying(10) NOT NULL,
    "path" character varying(500) NOT NULL,
    "query_params" "text",
    "request_headers" "jsonb",
    "request_body" "text",
    "status_code" integer NOT NULL,
    "response_headers" "jsonb",
    "response_body" "text",
    "client_ip" "inet",
    "user_agent" "text",
    "response_time_ms" bigint NOT NULL,
    "request_time" timestamp with time zone NOT NULL,
    "response_time" timestamp with time zone NOT NULL,
    "error_message" "text",
    "created_at" timestamp with time zone DEFAULT "now"(),
    "updated_at" timestamp with time zone DEFAULT "now"()
);


--
-- Name: http_logs_id_seq; Type: SEQUENCE; Schema: log; Owner: -
--

CREATE SEQUENCE "log"."http_logs_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: http_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: log; Owner: -
--

ALTER SEQUENCE "log"."http_logs_id_seq" OWNED BY "log"."http_logs"."id";


--
-- Name: error_message_mappings; Type: TABLE; Schema: mapping; Owner: -
--

CREATE TABLE "mapping"."error_message_mappings" (
    "id" bigint NOT NULL,
    "system_type" "text" NOT NULL,
    "error_pattern" "text" NOT NULL,
    "response_code" "text" NOT NULL,
    "is_exact_match" boolean DEFAULT false NOT NULL,
    "description" "text",
    "created_at" timestamp with time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp with time zone DEFAULT "now"() NOT NULL
);


--
-- Name: error_message_mappings_id_seq; Type: SEQUENCE; Schema: mapping; Owner: -
--

CREATE SEQUENCE "mapping"."error_message_mappings_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: error_message_mappings_id_seq; Type: SEQUENCE OWNED BY; Schema: mapping; Owner: -
--

ALTER SEQUENCE "mapping"."error_message_mappings_id_seq" OWNED BY "mapping"."error_message_mappings"."id";


--
-- Name: payment_bill_details; Type: TABLE; Schema: payment; Owner: -
--

CREATE TABLE "payment"."payment_bill_details" (
    "id" bigint NOT NULL,
    "partner_ref_id" "text" NOT NULL,
    "name" "text" NOT NULL,
    "value" "text",
    "is_pii" boolean DEFAULT false NOT NULL,
    "is_show" boolean DEFAULT true NOT NULL,
    "created_at" timestamp with time zone DEFAULT "now"() NOT NULL
);


--
-- Name: payment_bill_details_id_seq; Type: SEQUENCE; Schema: payment; Owner: -
--

CREATE SEQUENCE "payment"."payment_bill_details_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: payment_bill_details_id_seq; Type: SEQUENCE OWNED BY; Schema: payment; Owner: -
--

ALTER SEQUENCE "payment"."payment_bill_details_id_seq" OWNED BY "payment"."payment_bill_details"."id";


--
-- Name: payment_requests; Type: TABLE; Schema: payment; Owner: -
--

CREATE TABLE "payment"."payment_requests" (
    "id" bigint NOT NULL,
    "ref_id" "text" NOT NULL,
    "partner_inquiry_id" "text" NOT NULL,
    "client_number" "text",
    "category" "text" NOT NULL,
    "rsid" "text" NOT NULL,
    "product_code" "text" NOT NULL,
    "total_amount" numeric(18,2) DEFAULT 0 NOT NULL,
    "ts" timestamp with time zone NOT NULL,
    "created_at" timestamp with time zone DEFAULT "now"() NOT NULL
);


--
-- Name: payment_requests_id_seq; Type: SEQUENCE; Schema: payment; Owner: -
--

CREATE SEQUENCE "payment"."payment_requests_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: payment_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: payment; Owner: -
--

ALTER SEQUENCE "payment"."payment_requests_id_seq" OWNED BY "payment"."payment_requests"."id";


--
-- Name: payment_responses; Type: TABLE; Schema: payment; Owner: -
--

CREATE TABLE "payment"."payment_responses" (
    "id" bigint NOT NULL,
    "payment_request_id" bigint,
    "partner_ref_id" "text" NOT NULL,
    "client_number" "text",
    "product_code" "text" NOT NULL,
    "response_code" "text" NOT NULL,
    "message" "text",
    "admin_fee" numeric(18,2) DEFAULT 0,
    "total_amount" numeric(18,2) DEFAULT 0 NOT NULL,
    "ts" timestamp with time zone NOT NULL,
    "bill_count" integer DEFAULT 0 NOT NULL,
    "created_at" timestamp with time zone DEFAULT "now"() NOT NULL,
    "response_updated_at" timestamp with time zone DEFAULT "now"() NOT NULL,
    "sales_price" numeric(18,2) DEFAULT 0
);


--
-- Name: payment_responses_id_seq; Type: SEQUENCE; Schema: payment; Owner: -
--

CREATE SEQUENCE "payment"."payment_responses_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: payment_responses_id_seq; Type: SEQUENCE OWNED BY; Schema: payment; Owner: -
--

ALTER SEQUENCE "payment"."payment_responses_id_seq" OWNED BY "payment"."payment_responses"."id";


--
-- Name: http_callback_logs id; Type: DEFAULT; Schema: callback; Owner: -
--

ALTER TABLE ONLY "callback"."http_callback_logs" ALTER COLUMN "id" SET DEFAULT "nextval"('"callback"."http_callback_logs_id_seq"'::"regclass");


--
-- Name: bill_details id; Type: DEFAULT; Schema: inquiry; Owner: -
--

ALTER TABLE ONLY "inquiry"."bill_details" ALTER COLUMN "id" SET DEFAULT "nextval"('"inquiry"."bill_details_id_seq"'::"regclass");


--
-- Name: inquiry_requests id; Type: DEFAULT; Schema: inquiry; Owner: -
--

ALTER TABLE ONLY "inquiry"."inquiry_requests" ALTER COLUMN "id" SET DEFAULT "nextval"('"inquiry"."inquiry_requests_id_seq"'::"regclass");


--
-- Name: inquiry_responses id; Type: DEFAULT; Schema: inquiry; Owner: -
--

ALTER TABLE ONLY "inquiry"."inquiry_responses" ALTER COLUMN "id" SET DEFAULT "nextval"('"inquiry"."inquiry_responses_id_seq"'::"regclass");


--
-- Name: http_logs id; Type: DEFAULT; Schema: log; Owner: -
--

ALTER TABLE ONLY "log"."http_logs" ALTER COLUMN "id" SET DEFAULT "nextval"('"log"."http_logs_id_seq"'::"regclass");


--
-- Name: error_message_mappings id; Type: DEFAULT; Schema: mapping; Owner: -
--

ALTER TABLE ONLY "mapping"."error_message_mappings" ALTER COLUMN "id" SET DEFAULT "nextval"('"mapping"."error_message_mappings_id_seq"'::"regclass");


--
-- Name: payment_bill_details id; Type: DEFAULT; Schema: payment; Owner: -
--

ALTER TABLE ONLY "payment"."payment_bill_details" ALTER COLUMN "id" SET DEFAULT "nextval"('"payment"."payment_bill_details_id_seq"'::"regclass");


--
-- Name: payment_requests id; Type: DEFAULT; Schema: payment; Owner: -
--

ALTER TABLE ONLY "payment"."payment_requests" ALTER COLUMN "id" SET DEFAULT "nextval"('"payment"."payment_requests_id_seq"'::"regclass");


--
-- Name: payment_responses id; Type: DEFAULT; Schema: payment; Owner: -
--

ALTER TABLE ONLY "payment"."payment_responses" ALTER COLUMN "id" SET DEFAULT "nextval"('"payment"."payment_responses_id_seq"'::"regclass");


--
-- Name: http_callback_logs http_callback_logs_pkey; Type: CONSTRAINT; Schema: callback; Owner: -
--

ALTER TABLE ONLY "callback"."http_callback_logs"
    ADD CONSTRAINT "http_callback_logs_pkey" PRIMARY KEY ("id");


--
-- Name: bill_details bill_details_pkey; Type: CONSTRAINT; Schema: inquiry; Owner: -
--

ALTER TABLE ONLY "inquiry"."bill_details"
    ADD CONSTRAINT "bill_details_pkey" PRIMARY KEY ("id");


--
-- Name: inquiry_requests inquiry_requests_pkey; Type: CONSTRAINT; Schema: inquiry; Owner: -
--

ALTER TABLE ONLY "inquiry"."inquiry_requests"
    ADD CONSTRAINT "inquiry_requests_pkey" PRIMARY KEY ("id");


--
-- Name: inquiry_responses inquiry_responses_pkey; Type: CONSTRAINT; Schema: inquiry; Owner: -
--

ALTER TABLE ONLY "inquiry"."inquiry_responses"
    ADD CONSTRAINT "inquiry_responses_pkey" PRIMARY KEY ("id");


--
-- Name: http_logs http_logs_pkey; Type: CONSTRAINT; Schema: log; Owner: -
--

ALTER TABLE ONLY "log"."http_logs"
    ADD CONSTRAINT "http_logs_pkey" PRIMARY KEY ("id");


--
-- Name: http_logs http_logs_request_id_key; Type: CONSTRAINT; Schema: log; Owner: -
--

ALTER TABLE ONLY "log"."http_logs"
    ADD CONSTRAINT "http_logs_request_id_key" UNIQUE ("request_id");


--
-- Name: error_message_mappings error_message_mappings_pkey; Type: CONSTRAINT; Schema: mapping; Owner: -
--

ALTER TABLE ONLY "mapping"."error_message_mappings"
    ADD CONSTRAINT "error_message_mappings_pkey" PRIMARY KEY ("id");


--
-- Name: error_message_mappings error_message_mappings_system_type_error_pattern_key; Type: CONSTRAINT; Schema: mapping; Owner: -
--

ALTER TABLE ONLY "mapping"."error_message_mappings"
    ADD CONSTRAINT "error_message_mappings_system_type_error_pattern_key" UNIQUE ("system_type", "error_pattern");


--
-- Name: payment_bill_details payment_bill_details_pkey; Type: CONSTRAINT; Schema: payment; Owner: -
--

ALTER TABLE ONLY "payment"."payment_bill_details"
    ADD CONSTRAINT "payment_bill_details_pkey" PRIMARY KEY ("id");


--
-- Name: payment_requests payment_requests_pkey; Type: CONSTRAINT; Schema: payment; Owner: -
--

ALTER TABLE ONLY "payment"."payment_requests"
    ADD CONSTRAINT "payment_requests_pkey" PRIMARY KEY ("id");


--
-- Name: payment_requests payment_requests_ref_id_key; Type: CONSTRAINT; Schema: payment; Owner: -
--

ALTER TABLE ONLY "payment"."payment_requests"
    ADD CONSTRAINT "payment_requests_ref_id_key" UNIQUE ("ref_id");


--
-- Name: payment_responses payment_responses_pkey; Type: CONSTRAINT; Schema: payment; Owner: -
--

ALTER TABLE ONLY "payment"."payment_responses"
    ADD CONSTRAINT "payment_responses_pkey" PRIMARY KEY ("id");


--
-- Name: idx_callback_logs_callback_type; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_callback_type" ON "callback"."http_callback_logs" USING "btree" ("callback_type");


--
-- Name: idx_callback_logs_client_ip; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_client_ip" ON "callback"."http_callback_logs" USING "btree" ("client_ip");


--
-- Name: idx_callback_logs_client_number; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_client_number" ON "callback"."http_callback_logs" USING "btree" ("client_number");


--
-- Name: idx_callback_logs_created_at; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_created_at" ON "callback"."http_callback_logs" USING "btree" ("created_at");


--
-- Name: idx_callback_logs_created_at_for_cleanup; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_created_at_for_cleanup" ON "callback"."http_callback_logs" USING "btree" ("created_at");


--
-- Name: idx_callback_logs_method; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_method" ON "callback"."http_callback_logs" USING "btree" ("method");


--
-- Name: idx_callback_logs_method_path; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_method_path" ON "callback"."http_callback_logs" USING "btree" ("method", "path");


--
-- Name: idx_callback_logs_partner_ref_id; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_partner_ref_id" ON "callback"."http_callback_logs" USING "btree" ("partner_ref_id");


--
-- Name: idx_callback_logs_partner_ref_time; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_partner_ref_time" ON "callback"."http_callback_logs" USING "btree" ("partner_ref_id", "request_time");


--
-- Name: idx_callback_logs_path; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_path" ON "callback"."http_callback_logs" USING "btree" ("path");


--
-- Name: idx_callback_logs_product_code; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_product_code" ON "callback"."http_callback_logs" USING "btree" ("product_code");


--
-- Name: idx_callback_logs_ref_id; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_ref_id" ON "callback"."http_callback_logs" USING "btree" ("ref_id");


--
-- Name: idx_callback_logs_ref_time; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_ref_time" ON "callback"."http_callback_logs" USING "btree" ("ref_id", "request_time");


--
-- Name: idx_callback_logs_request_time; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_request_time" ON "callback"."http_callback_logs" USING "btree" ("request_time");


--
-- Name: idx_callback_logs_response_code; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_response_code" ON "callback"."http_callback_logs" USING "btree" ("response_code");


--
-- Name: idx_callback_logs_response_time; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_response_time" ON "callback"."http_callback_logs" USING "btree" ("response_time");


--
-- Name: idx_callback_logs_status_code; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_status_code" ON "callback"."http_callback_logs" USING "btree" ("status_code");


--
-- Name: idx_callback_logs_status_time; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_status_time" ON "callback"."http_callback_logs" USING "btree" ("status_code", "request_time");


--
-- Name: idx_callback_logs_type_time; Type: INDEX; Schema: callback; Owner: -
--

CREATE INDEX "idx_callback_logs_type_time" ON "callback"."http_callback_logs" USING "btree" ("callback_type", "request_time");


--
-- Name: idx_bill_details_pps_inquiry_id; Type: INDEX; Schema: inquiry; Owner: -
--

CREATE INDEX "idx_bill_details_pps_inquiry_id" ON "inquiry"."bill_details" USING "btree" ("pps_inquiry_id");


--
-- Name: idx_inquiry_requests_ref_id; Type: INDEX; Schema: inquiry; Owner: -
--

CREATE INDEX "idx_inquiry_requests_ref_id" ON "inquiry"."inquiry_requests" USING "btree" ("ref_id");


--
-- Name: idx_inquiry_responses_request_id; Type: INDEX; Schema: inquiry; Owner: -
--

CREATE INDEX "idx_inquiry_responses_request_id" ON "inquiry"."inquiry_responses" USING "btree" ("inquiry_request_id");


--
-- Name: idx_inquiry_responses_validation; Type: INDEX; Schema: inquiry; Owner: -
--

CREATE INDEX "idx_inquiry_responses_validation" ON "inquiry"."inquiry_responses" USING "btree" ("pps_inquiry_id", "product_code", "total_amount");


--
-- Name: idx_http_logs_client_ip; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_client_ip" ON "log"."http_logs" USING "btree" ("client_ip");


--
-- Name: idx_http_logs_created_at; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_created_at" ON "log"."http_logs" USING "btree" ("created_at");


--
-- Name: idx_http_logs_method; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_method" ON "log"."http_logs" USING "btree" ("method");


--
-- Name: idx_http_logs_method_path; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_method_path" ON "log"."http_logs" USING "btree" ("method", "path");


--
-- Name: idx_http_logs_path; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_path" ON "log"."http_logs" USING "btree" ("path");


--
-- Name: idx_http_logs_request_id; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_request_id" ON "log"."http_logs" USING "btree" ("request_id");


--
-- Name: idx_http_logs_request_time; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_request_time" ON "log"."http_logs" USING "btree" ("request_time");


--
-- Name: idx_http_logs_response_time; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_response_time" ON "log"."http_logs" USING "btree" ("response_time");


--
-- Name: idx_http_logs_status_code; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_status_code" ON "log"."http_logs" USING "btree" ("status_code");


--
-- Name: idx_http_logs_status_time; Type: INDEX; Schema: log; Owner: -
--

CREATE INDEX "idx_http_logs_status_time" ON "log"."http_logs" USING "btree" ("status_code", "request_time");


--
-- Name: idx_error_message_mappings_response_code; Type: INDEX; Schema: mapping; Owner: -
--

CREATE INDEX "idx_error_message_mappings_response_code" ON "mapping"."error_message_mappings" USING "btree" ("response_code");


--
-- Name: idx_error_message_mappings_system_type; Type: INDEX; Schema: mapping; Owner: -
--

CREATE INDEX "idx_error_message_mappings_system_type" ON "mapping"."error_message_mappings" USING "btree" ("system_type");


--
-- Name: idx_payment_bill_details_name; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_bill_details_name" ON "payment"."payment_bill_details" USING "btree" ("partner_ref_id", "name");


--
-- Name: idx_payment_bill_details_partner_ref_id; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_bill_details_partner_ref_id" ON "payment"."payment_bill_details" USING "btree" ("partner_ref_id");


--
-- Name: idx_payment_requests_created_at; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_requests_created_at" ON "payment"."payment_requests" USING "btree" ("created_at" DESC);


--
-- Name: idx_payment_requests_partner_inquiry_id; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_requests_partner_inquiry_id" ON "payment"."payment_requests" USING "btree" ("partner_inquiry_id");


--
-- Name: idx_payment_requests_ref_id; Type: INDEX; Schema: payment; Owner: -
--

CREATE UNIQUE INDEX "idx_payment_requests_ref_id" ON "payment"."payment_requests" USING "btree" ("ref_id");


--
-- Name: idx_payment_responses_code_partner_ref; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_responses_code_partner_ref" ON "payment"."payment_responses" USING "btree" ("response_code", "partner_ref_id");


--
-- Name: idx_payment_responses_created_at_partner_ref; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_responses_created_at_partner_ref" ON "payment"."payment_responses" USING "btree" ("created_at", "partner_ref_id");


--
-- Name: idx_payment_responses_partner_ref_id; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_responses_partner_ref_id" ON "payment"."payment_responses" USING "btree" ("partner_ref_id");


--
-- Name: idx_payment_responses_request_id; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_responses_request_id" ON "payment"."payment_responses" USING "btree" ("payment_request_id");


--
-- Name: idx_payment_responses_response_code; Type: INDEX; Schema: payment; Owner: -
--

CREATE INDEX "idx_payment_responses_response_code" ON "payment"."payment_responses" USING "btree" ("response_code");


--
-- Name: http_callback_logs trg_set_callback_log_updated_at; Type: TRIGGER; Schema: callback; Owner: -
--

CREATE TRIGGER "trg_set_callback_log_updated_at" BEFORE UPDATE ON "callback"."http_callback_logs" FOR EACH ROW EXECUTE FUNCTION "callback"."set_updated_at"();


--
-- Name: payment_responses trg_set_response_updated_at; Type: TRIGGER; Schema: payment; Owner: -
--

CREATE TRIGGER "trg_set_response_updated_at" BEFORE UPDATE OF "response_code", "message" ON "payment"."payment_responses" FOR EACH ROW EXECUTE FUNCTION "payment"."set_response_updated_at"();


--
-- Name: inquiry_responses inquiry_responses_inquiry_request_id_fkey; Type: FK CONSTRAINT; Schema: inquiry; Owner: -
--

ALTER TABLE ONLY "inquiry"."inquiry_responses"
    ADD CONSTRAINT "inquiry_responses_inquiry_request_id_fkey" FOREIGN KEY ("inquiry_request_id") REFERENCES "inquiry"."inquiry_requests"("id") ON DELETE CASCADE;


--
-- Name: payment_responses payment_responses_payment_request_id_fkey; Type: FK CONSTRAINT; Schema: payment; Owner: -
--

ALTER TABLE ONLY "payment"."payment_responses"
    ADD CONSTRAINT "payment_responses_payment_request_id_fkey" FOREIGN KEY ("payment_request_id") REFERENCES "payment"."payment_requests"("id") ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict G8FvpzaXQb5WMEUuKk6PqC6U33gdRI64njSUomArhMaUOdsckP7z7jxi3zEOSmi

