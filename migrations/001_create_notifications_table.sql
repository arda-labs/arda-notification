-- Migration: 001_create_notifications_table.sql
-- Creates the central notifications table for arda-notification service.

CREATE TABLE IF NOT EXISTS notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_key      VARCHAR(100) NOT NULL,
    user_id         VARCHAR(255) NOT NULL,
    type            VARCHAR(50)  NOT NULL CHECK (type IN ('SYSTEM', 'WORKFLOW', 'CRM', 'IAM', 'CUSTOM')),
    title           VARCHAR(255) NOT NULL,
    body            TEXT         NOT NULL DEFAULT '',
    metadata        JSONB,
    is_read         BOOLEAN      NOT NULL DEFAULT FALSE,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    source_event_id VARCHAR(255)
);

-- Composite index for the most common query: list by user within tenant, unread first
CREATE INDEX IF NOT EXISTS idx_notif_user_tenant
    ON notifications (tenant_key, user_id, is_read, created_at DESC);

-- Index for TTL purge job
CREATE INDEX IF NOT EXISTS idx_notif_created_at
    ON notifications (created_at);

-- Unique constraint to prevent duplicate events from Kafka at-least-once delivery
CREATE UNIQUE INDEX IF NOT EXISTS idx_notif_source_event
    ON notifications (source_event_id)
    WHERE source_event_id IS NOT NULL;
