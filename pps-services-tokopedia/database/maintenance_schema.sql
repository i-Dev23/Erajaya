-- Partitioning utilities for monthly partitions by created date
-- This script provides helper functions to create monthly partitions
-- for tables in schemas: log, callback, inquiry, payment.

CREATE SCHEMA IF NOT EXISTS maintenance;

-- Helper: create one monthly partition for a partitioned parent table
CREATE OR REPLACE FUNCTION maintenance.create_month_partition(
    p_parent_table REGCLASS,
    p_from DATE
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_parent TEXT := p_parent_table::TEXT; -- e.g., 'log.http_logs'
    v_to DATE := (date_trunc('month', p_from) + INTERVAL '1 month')::DATE;
    v_suffix TEXT := to_char(p_from, 'YYYYMM');
    v_child TEXT;
    v_created BOOLEAN := FALSE;
BEGIN
    v_child := v_parent || '_' || v_suffix;

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
        SELECT 1 FROM pg_class WHERE relname = split_part(v_child, '.', 2) AND relnamespace = (split_part(v_child, '.', 1))::regnamespace
    ) THEN
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%L) TO (%L)',
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
        RAISE NOTICE 'Created partition % for range [% - %)', v_child, p_from, v_to;
    END IF;
END;
$$;

-- Ensure partitions exist for current month and the next N months
CREATE OR REPLACE FUNCTION maintenance.ensure_monthly_partitions(
    p_months_ahead INTEGER DEFAULT 2
)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    v_month DATE := date_trunc('month', now())::DATE;
    i INT := 0;
BEGIN
    -- Create for current + ahead months for each parent table
    FOR i IN 0..p_months_ahead LOOP
        PERFORM maintenance.create_month_partition('log.http_logs'::regclass, (v_month + (i || ' months')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition('callback.http_callback_logs'::regclass, (v_month + (i || ' months')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition('inquiry.inquiry_requests'::regclass, (v_month + (i || ' months')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition('inquiry.inquiry_responses'::regclass, (v_month + (i || ' months')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition('payment.payment_requests'::regclass, (v_month + (i || ' months')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition('payment.payment_responses'::regclass, (v_month + (i || ' months')::INTERVAL)::DATE);
        -- Additional bill detail tables (safe no-op if parents are not partitioned yet)
        PERFORM maintenance.create_month_partition('inquiry.bill_details'::regclass, (v_month + (i || ' months')::INTERVAL)::DATE);
        PERFORM maintenance.create_month_partition('payment.payment_bill_details'::regclass, (v_month + (i || ' months')::INTERVAL)::DATE);
    END LOOP;
END;
$$;

-- Notes:
-- 1) You must first convert parent tables to be partitioned by RANGE on created_at (or appropriate timestamp)
--    Example (new deployment only):
--    ALTER TABLE log.http_logs DROP CONSTRAINT IF EXISTS http_logs_pkey; -- adjust as needed
--    ALTER TABLE log.http_logs PARTITION BY RANGE (created_at);
--    Then create an initial default partition or past partitions.
-- 2) For existing large tables, plan a migration: create new partitioned parents,
--    copy data, swap names. This is beyond the scope of this helper.


