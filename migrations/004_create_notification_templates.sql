-- Migration: 004_create_notification_templates.sql
-- Stores localised notification templates that replace hardcoded messages.

CREATE TABLE IF NOT EXISTS notification_templates (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    template_key    VARCHAR(100) NOT NULL,  -- e.g. "bpm.task.assigned"
    locale          VARCHAR(10)  NOT NULL DEFAULT 'vi',
    title_template  TEXT         NOT NULL,  -- "Bạn có nhiệm vụ mới"
    body_template   TEXT         NOT NULL,  -- "Bạn được giao '{{taskName}}' trong '{{processName}}'."
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(template_key, locale)
);

-- ─── Vietnamese (vi) ──────────────────────────────────────────────────────────
INSERT INTO notification_templates (template_key, locale, title_template, body_template) VALUES
('tenant.created',          'vi', 'Tenant mới đã được khởi tạo',       'Tenant ''{{displayName}}'' đã được tạo thành công.'),
('tenant.updated',          'vi', 'Tenant đã được cập nhật',            'Cấu hình của tenant ''{{displayName}}'' đã được cập nhật thành công.'),
('tenant.status_updated',   'vi', 'Trạng thái tenant thay đổi',         'Trạng thái của tenant ''{{tenantKey}}'' đã đổi thành {{status}}.'),
('tenant.deleted',          'vi', 'Đã xóa tenant',                      'Tenant ''{{tenantKey}}'' đã bị xóa khỏi hệ thống.'),
('bpm.task_assigned',       'vi', 'Bạn có nhiệm vụ mới',                'Bạn được giao nhiệm vụ ''{{taskName}}'' trong quy trình ''{{processName}}''.'),
('bpm.task_completed',      'vi', 'Nhiệm vụ hoàn thành',                'Nhiệm vụ ''{{taskName}}'' đã được hoàn thành.'),
('bpm.approval_required',   'vi', 'Yêu cầu phê duyệt',                  'Bạn cần phê duyệt ''{{taskName}}'' trong quy trình ''{{processName}}''.'),
('crm.lead_status_changed','vi', 'Trạng thái lead thay đổi',           'Trạng thái của lead ''{{entityName}}'' đã được cập nhật.'),
('crm.deal_updated',       'vi', 'Deal đã được cập nhật',              'Deal ''{{entityName}}'' vừa được cập nhật.'),
('iam.login_new_device',   'vi', 'Đăng nhập từ thiết bị mới',          'Tài khoản của bạn vừa được truy cập từ thiết bị mới (IP: {{ip}}). Nếu không phải bạn, hãy đổi mật khẩu ngay.'),
('iam.password_changed',   'vi', 'Mật khẩu đã thay đổi',               'Mật khẩu tài khoản của bạn vừa được đổi. Hãy liên hệ quản trị viên nếu bạn không thực hiện thao tác này.');

-- ─── English (en) ──────────────────────────────────────────────────────────────
INSERT INTO notification_templates (template_key, locale, title_template, body_template) VALUES
('tenant.created',          'en', 'New tenant created',                 'Tenant ''{{displayName}}'' has been created successfully.'),
('tenant.updated',          'en', 'Tenant updated',                     'Configuration for tenant ''{{displayName}}'' has been updated.'),
('tenant.status_updated',   'en', 'Tenant status changed',              'Status of tenant ''{{tenantKey}}'' changed to {{status}}.'),
('tenant.deleted',          'en', 'Tenant deleted',                     'Tenant ''{{tenantKey}}'' has been removed from the system.'),
('bpm.task_assigned',       'en', 'New task assigned',                  'You have been assigned task ''{{taskName}}'' in process ''{{processName}}''.'),
('bpm.task_completed',      'en', 'Task completed',                     'Task ''{{taskName}}'' has been completed.'),
('bpm.approval_required',   'en', 'Approval required',                  'You need to approve ''{{taskName}}'' in process ''{{processName}}''.'),
('crm.lead_status_changed','en', 'Lead status changed',                'Status of lead ''{{entityName}}'' has been updated.'),
('crm.deal_updated',       'en', 'Deal updated',                       'Deal ''{{entityName}}'' has been updated.'),
('iam.login_new_device',   'en', 'Login from new device',              'Your account was accessed from a new device (IP: {{ip}}). If this wasn''t you, change your password immediately.'),
('iam.password_changed',   'en', 'Password changed',                   'Your account password has been changed. Contact your administrator if you did not make this change.');
