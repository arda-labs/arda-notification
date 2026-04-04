package email

import (
	"fmt"
	"strings"
)

// BuildHTML constructs a simple HTML email body for a notification.
func BuildHTML(title, body string, actions []map[string]string) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"></head><body style="font-family:system-ui,sans-serif;margin:0;padding:20px;background:#f9fafb;">`)
	sb.WriteString(`<div style="max-width:560px;margin:0 auto;background:#fff;border-radius:8px;padding:24px;border:1px solid #e5e7eb;">`)
	sb.WriteString(`<h2 style="margin:0 0 12px;font-size:18px;color:#111827;">`)
	sb.WriteString(escapeHTML(title))
	sb.WriteString(`</h2>`)
	sb.WriteString(`<p style="margin:0 0 16px;font-size:14px;color:#374151;line-height:1.6;">`)
	sb.WriteString(escapeHTML(body))
	sb.WriteString(`</p>`)

	if len(actions) > 0 {
		sb.WriteString(`<div style="display:flex;gap:8px;margin-top:16px;">`)
		for _, a := range actions {
			variant := a["variant"]
			style := `display:inline-block;padding:8px 16px;border-radius:6px;text-decoration:none;font-size:13px;font-weight:500;`
			switch variant {
			case "primary":
				style += `background:#2563eb;color:#fff;`
			case "destructive":
				style += `background:#dc2626;color:#fff;`
			default:
				style += `background:#f3f4f6;color:#374151;border:1px solid #d1d5db;`
			}
			url := a["url"]
			if !strings.HasPrefix(url, "http") {
				url = "#" // Relative URLs don't work in email
			}
			sb.WriteString(fmt.Sprintf(`<a href="%s" style="%s">%s</a> `, url, style, escapeHTML(a["label"])))
		}
		sb.WriteString(`</div>`)
	}

	sb.WriteString(`<hr style="margin:20px 0;border:none;border-top:1px solid #e5e7eb;">`)
	sb.WriteString(`<p style="margin:0;font-size:11px;color:#9ca3af;">Arda Notification &mdash; arda.io.vn</p>`)
	sb.WriteString(`</div></body></html>`)
	return sb.String()
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
