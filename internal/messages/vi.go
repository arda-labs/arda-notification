package messages

// ─── Tenant ──────────────────────────────────────────────────────────────────

const (
	TenantCreatedTitle = "Tenant mới đã được khởi tạo"
	TenantCreatedBody  = "Tenant '%s' (DB: %s) đã được tạo thành công."

	TenantUpdatedTitle = "Tenant đã được cập nhật"
	TenantUpdatedBody  = "Cấu hình của tenant '%s' đã được cập nhật thành công."

	TenantStatusUpdatedTitle = "Trạng thái tenant thay đổi"
	TenantStatusUpdatedBody  = "Trạng thái của tenant '%s' đã được đổi thành %s."

	TenantDeletedTitle = "Đã xóa tenant"
	TenantDeletedBody  = "Tenant '%s' đã bị xóa khỏi hệ thống."
)

// ─── BPM ─────────────────────────────────────────────────────────────────────

const (
	TaskAssignedTitle = "Bạn có nhiệm vụ mới"
	TaskAssignedBody  = "Bạn được giao nhiệm vụ '%s' trong quy trình '%s'."

	TaskCompletedTitle = "Nhiệm vụ hoàn thành"
	TaskCompletedBody  = "Nhiệm vụ '%s' đã được hoàn thành."

	ApprovalRequiredTitle = "Yêu cầu phê duyệt"
	ApprovalRequiredBody  = "Bạn cần phê duyệt '%s' trong quy trình '%s'."
)

// ─── CRM ─────────────────────────────────────────────────────────────────────

const (
	LeadStatusChangedTitle = "Trạng thái lead thay đổi"
	LeadStatusChangedBody  = "Trạng thái của lead '%s' đã được cập nhật."

	DealUpdatedTitle = "Deal đã được cập nhật"
	DealUpdatedBody  = "Deal '%s' vừa được cập nhật."
)

// ─── IAM ─────────────────────────────────────────────────────────────────────

const (
	LoginNewDeviceTitle = "Đăng nhập từ thiết bị mới"
	LoginNewDeviceBody  = "Tài khoản của bạn vừa được truy cập từ thiết bị mới (IP: %s). Nếu không phải bạn, hãy đổi mật khẩu ngay."

	PasswordChangedTitle = "Mật khẩu đã thay đổi"
	PasswordChangedBody  = "Mật khẩu tài khoản của bạn vừa được đổi. Hãy liên hệ quản trị viên nếu bạn không thực hiện thao tác này."
)
