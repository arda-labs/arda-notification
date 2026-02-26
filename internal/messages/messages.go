package messages

import "fmt"

// ─── Tenant builders ─────────────────────────────────────────────────────────

func TenantCreated(displayName, dbType string) (string, string) {
	return TenantCreatedTitle, fmt.Sprintf(TenantCreatedBody, displayName, dbType)
}

func TenantUpdated(displayName string) (string, string) {
	return TenantUpdatedTitle, fmt.Sprintf(TenantUpdatedBody, displayName)
}

func TenantStatusUpdated(tenantKey, status string) (string, string) {
	return TenantStatusUpdatedTitle, fmt.Sprintf(TenantStatusUpdatedBody, tenantKey, status)
}

func TenantDeleted(tenantKey string) (string, string) {
	return TenantDeletedTitle, fmt.Sprintf(TenantDeletedBody, tenantKey)
}

// ─── BPM builders ────────────────────────────────────────────────────────────

func TaskAssigned(taskName, processName string) (string, string) {
	return TaskAssignedTitle, fmt.Sprintf(TaskAssignedBody, taskName, processName)
}

func TaskCompleted(taskName string) (string, string) {
	return TaskCompletedTitle, fmt.Sprintf(TaskCompletedBody, taskName)
}

func ApprovalRequired(taskName, processName string) (string, string) {
	return ApprovalRequiredTitle, fmt.Sprintf(ApprovalRequiredBody, taskName, processName)
}

// ─── CRM builders ────────────────────────────────────────────────────────────

func LeadStatusChanged(entityName string) (string, string) {
	return LeadStatusChangedTitle, fmt.Sprintf(LeadStatusChangedBody, entityName)
}

func DealUpdated(entityName string) (string, string) {
	return DealUpdatedTitle, fmt.Sprintf(DealUpdatedBody, entityName)
}

// ─── IAM builders ────────────────────────────────────────────────────────────

func LoginNewDevice(ip string) (string, string) {
	return LoginNewDeviceTitle, fmt.Sprintf(LoginNewDeviceBody, ip)
}

func PasswordChanged() (string, string) {
	return PasswordChangedTitle, PasswordChangedBody
}
