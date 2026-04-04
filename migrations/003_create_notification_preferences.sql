-- Migration: 003_create_notification_preferences.sql
-- Stores per-user notification channel preferences and quiet hours.

CREATE TABLE IF NOT EXISTS notification_preferences (
    id                UUID PRIMARY KEY DEFAULT uuidv7(),
    tenant_key        VARCHAR(100) NOT NULL,
    user_id           VARCHAR(255) NOT NULL,
    type              VARCHAR(50)  NOT NULL CHECK (type IN ('SYSTEM', 'WORKFLOW', 'CRM', 'IAM', 'CUSTOM')),
    channel_in_app    BOOLEAN      NOT NULL DEFAULT TRUE,
    channel_email     BOOLEAN      NOT NULL DEFAULT FALSE,
    quiet_hours_start TIME,                    -- NULL = quiet hours disabled
    quiet_hours_end   TIME,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE(tenant_key, user_id, type)
);

CREATE INDEX IF NOT EXISTS idx_pref_user
    ON notification_preferences (tenant_key, user_id);
