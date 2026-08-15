package radius

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type AuditLog struct {
	ID            int64     `json:"id"`
	AdminID       *int64    `json:"admin_id"`
	AdminUsername string    `json:"admin_username"`
	ActionType    string    `json:"action_type"`
	Target        string    `json:"target"`
	Details       string    `json:"details"`
	IPAddress     string    `json:"ip_address"`
	CreatedAt     time.Time `json:"created_at"`
}

// LogActivity inserts a new audit log record asynchronously
func LogActivity(adminID *int64, adminUsername, actionType, target, details, ipAddress string) {
	if DB == nil {
		return
	}
	go func() {
		var aID interface{}
		if adminID != nil && *adminID > 0 {
			aID = *adminID
		} else {
			aID = nil
		}
		if adminUsername == "" {
			adminUsername = "النظام"
		}
		_, err := DB.Exec(`INSERT INTO radius_audit_logs (admin_id, admin_username, action_type, target, details, ip_address, created_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			aID, adminUsername, actionType, target, details, ipAddress)
		if err != nil {
			log.Printf("[audit] Failed to log activity: %v", err)
		}
	}()
}

// LogActivityFromCtx extracts context information (admin_id, username, IP) and logs activity
func LogActivityFromCtx(c *fiber.Ctx, actionType, target, details string) {
	if c == nil {
		LogActivity(nil, "النظام", actionType, target, details, "127.0.0.1")
		return
	}

	var adminIDPtr *int64
	if val := c.Locals("admin_id"); val != nil {
		switch v := val.(type) {
		case int64:
			if v > 0 {
				adminIDPtr = &v
			}
		case int:
			if v > 0 {
				id64 := int64(v)
				adminIDPtr = &id64
			}
		case float64:
			if v > 0 {
				id64 := int64(v)
				adminIDPtr = &id64
			}
		}
	}

	username, _ := c.Locals("username").(string)
	if username == "" {
		username, _ = c.Locals("role").(string)
	}
	if username == "" {
		username = "مدير النظام"
	}

	ip := c.IP()
	if ip == "" || ip == "::1" {
		ip = "127.0.0.1"
	}

	LogActivity(adminIDPtr, username, actionType, target, details, ip)
}

// GetAuditLogsHandler fetches logs with filtering, searching, and pagination
func GetAuditLogsHandler(c *fiber.Ctx) error {
	if DB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "قاعدة البيانات غير متاحة"})
	}

	role, _ := c.Locals("role").(string)
	currentAdminID, _ := c.Locals("admin_id").(int64)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 50
	}
	offset := (page - 1) * limit

	actionType := strings.TrimSpace(c.Query("action_type", ""))
	search := strings.TrimSpace(c.Query("search", ""))
	startDate := strings.TrimSpace(c.Query("start_date", ""))
	endDate := strings.TrimSpace(c.Query("end_date", ""))

	whereClauses := []string{"1=1"}
	args := []interface{}{}

	// Role-based scoping: Non-superadmins see actions performed by themselves or their child admins
	if role != "superadmin" && currentAdminID > 0 {
		whereClauses = append(whereClauses, "(admin_id = ? OR admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?))")
		args = append(args, currentAdminID, currentAdminID)
	}

	if actionType != "" {
		whereClauses = append(whereClauses, "action_type = ?")
		args = append(args, actionType)
	}

	if search != "" {
		whereClauses = append(whereClauses, "(admin_username LIKE ? OR target LIKE ? OR details LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if startDate != "" {
		whereClauses = append(whereClauses, "created_at >= ?")
		args = append(args, startDate+" 00:00:00")
	}

	if endDate != "" {
		whereClauses = append(whereClauses, "created_at <= ?")
		args = append(args, endDate+" 23:59:59")
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Count total records
	var total int
	countQuery := "SELECT COUNT(*) FROM radius_audit_logs WHERE " + whereSQL
	if err := DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		log.Printf("[audit] Count error: %v", err)
	}

	// Query paginated logs
	query := fmt.Sprintf("SELECT id, admin_id, admin_username, action_type, target, details, ip_address, created_at FROM radius_audit_logs WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?", whereSQL)
	queryArgs := append(args, limit, offset)

	rows, err := DB.Query(query, queryArgs...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل في قراءة سجل العمليات: " + err.Error()})
	}
	defer rows.Close()

	logs := []AuditLog{}
	for rows.Next() {
		var l AuditLog
		var createdAtStr string
		var adminIDVal *int64

		if err := rows.Scan(&l.ID, &adminIDVal, &l.AdminUsername, &l.ActionType, &l.Target, &l.Details, &l.IPAddress, &createdAtStr); err != nil {
			continue
		}
		l.AdminID = adminIDVal
		l.CreatedAt = parseDBTime(createdAtStr)
		logs = append(logs, l)
	}

	return c.JSON(fiber.Map{
		"logs":        logs,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": (total + limit - 1) / limit,
	})
}

// ClearAuditLogsHandler clears old or all audit logs (superadmin only)
func ClearAuditLogsHandler(c *fiber.Ctx) error {
	if DB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "قاعدة البيانات غير متاحة"})
	}

	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "غير مصرح لغير المدراء الأساسيين"})
	}

	days := c.Query("days", "")
	if days != "" {
		d, err := strconv.Atoi(days)
		if err == nil && d > 0 {
			res, err := DB.Exec(`DELETE FROM radius_audit_logs WHERE created_at < datetime('now', ?)` /* e.g. -30 days */, fmt.Sprintf("-%d days", d))
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "فشل تصفير السجل: " + err.Error()})
			}
			rowsAffected, _ := res.RowsAffected()
			LogActivityFromCtx(c, "تصفير السجل", "سجل العمليات", fmt.Sprintf("تم حذف العمليات الأقدم من %d يوم (عدد: %d)", d, rowsAffected))
			return c.JSON(fiber.Map{"message": fmt.Sprintf("تم حذف %d سجل قديم بنجاح", rowsAffected)})
		}
	}

	// Full clear
	_, err := DB.Exec(`DELETE FROM radius_audit_logs`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل تصفير سجل العمليات: " + err.Error()})
	}

	LogActivityFromCtx(c, "تصفير السجل", "سجل العمليات", "تم تصفير سجل العمليات بالكامل")
	return c.JSON(fiber.Map{"message": "تم تصفير سجل العمليات بالكامل بنجاح"})
}

// ExportAuditLogsCSVHandler exports logs to CSV file for Excel
func ExportAuditLogsCSVHandler(c *fiber.Ctx) error {
	if DB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "قاعدة البيانات غير متاحة"})
	}

	role, _ := c.Locals("role").(string)
	currentAdminID, _ := c.Locals("admin_id").(int64)

	whereClauses := []string{"1=1"}
	args := []interface{}{}

	if role != "superadmin" && currentAdminID > 0 {
		whereClauses = append(whereClauses, "(admin_id = ? OR admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?))")
		args = append(args, currentAdminID, currentAdminID)
	}

	actionType := strings.TrimSpace(c.Query("action_type", ""))
	search := strings.TrimSpace(c.Query("search", ""))
	if actionType != "" {
		whereClauses = append(whereClauses, "action_type = ?")
		args = append(args, actionType)
	}
	if search != "" {
		searchTerm := "%" + search + "%"
		whereClauses = append(whereClauses, "(admin_username LIKE ? OR target LIKE ? OR details LIKE ?)")
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	whereSQL := strings.Join(whereClauses, " AND ")
	query := fmt.Sprintf("SELECT id, admin_username, action_type, target, details, ip_address, created_at FROM radius_audit_logs WHERE %s ORDER BY id DESC LIMIT 5000", whereSQL)

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل في التصدير: " + err.Error()})
	}
	defer rows.Close()

	buf := new(bytes.Buffer)
	// Write UTF-8 BOM so Excel opens Arabic correctly
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(buf)
	_ = writer.Write([]string{"#", "المنفذ", "نوع العملية", "الهدف / العناصر", "التفاصيل", "عنوان IP", "التاريخ والوقت"})

	for rows.Next() {
		var id int64
		var adminName, actType, target, details, ip, createdAtStr string
		if err := rows.Scan(&id, &adminName, &actType, &target, &details, &ip, &createdAtStr); err == nil {
			_ = writer.Write([]string{
				fmt.Sprintf("%d", id),
				adminName,
				actType,
				target,
				details,
				ip,
				createdAtStr,
			})
		}
	}
	writer.Flush()

	filename := fmt.Sprintf("audit_logs_%s.csv", time.Now().Format("20060102_150405"))
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Set("Content-Type", "text/csv; charset=utf-8")
	return c.Send(buf.Bytes())
}
