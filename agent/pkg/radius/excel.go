package radius

import (
	"bytes"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// parseExcelDate parses dates in various formats from Excel sheet
func parseExcelDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Try standard date/time formats
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04",
		"2006-01-02",
		"02-01-2006",
		"01-02-2006",
		"2/5/2006 15:04:05",
		"2/5/2006 15:04",
		"2/5/2006",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}

	// Try parsing Excel float serial date (e.g. 46056.7427)
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		// Excel date epoch starts 1899-12-30
		anchor := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		days := int(f)
		frac := f - float64(days)
		t := anchor.AddDate(0, 0, days).Add(time.Duration(frac * 24 * float64(time.Hour)))
		return t.Format("2006-01-02 15:04:05")
	}

	return raw
}

// ImportFromExcel handles POST requests to upload an Excel file of users
func ImportFromExcel(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)

	// Get file from multipart form
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "يرجى إرفاق ملف الاستيراد"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل فتح الملف: " + err.Error()})
	}
	defer file.Close()

	// Load workbook using excelize
	f, err := excelize.OpenReader(file)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "الملف المرفوع ليس ملف Excel صالح: " + err.Error()})
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "الملف فارغ ولا يحتوي على أي صفحات (Sheets)"})
	}

	sheetName := sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل قراءة صفوف الصفحة: " + err.Error()})
	}

	if len(rows) < 2 {
		return c.Status(400).JSON(fiber.Map{"error": "الملف لا يحتوي على بيانات للمشتركين (فقط سطر العنوان أو فارغ)"})
	}

	// Map header names to column index
	headerMap := make(map[string]int)
	for idx, col := range rows[0] {
		headerMap[strings.ToLower(strings.TrimSpace(col))] = idx
	}

	getValue := func(row []string, colName string) string {
		idx, ok := headerMap[colName]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	importCount := 0
	skipCount := 0
	profilesSeen := make(map[string]bool)

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		username := getValue(row, "username")
		password := getValue(row, "ct_password")
		if password == "" {
			password = getValue(row, "password")
		}

		if username == "" || password == "" {
			skipCount++
			continue
		}

		profile := getValue(row, "profile_name")
		if profile == "" {
			profile = getValue(row, "group_name")
		}
		if profile == "" {
			profile = getValue(row, "profile")
		}

		expiration := parseExcelDate(getValue(row, "expiration"))
		
		balanceStr := getValue(row, "balance")
		balance, _ := strconv.ParseFloat(balanceStr, 64)

		firstname := getValue(row, "firstname")
		lastname := getValue(row, "lastname")
		fullName := strings.TrimSpace(firstname + " " + lastname)

		phone := getValue(row, "phone")

		// Ensure profile group exists in DB
		if profile != "" && !profilesSeen[profile] {
			err := ensureProfileExists(profile)
			if err == nil {
				profilesSeen[profile] = true
			}
		}

		// Import user record using the existing function in import_sas4.go
		err = importUser(username, password, profile, expiration, balance, fullName, phone)
		if err == nil {
			// If enabled field is explicitly set, we enforce it
			enabledStr := getValue(row, "enabled")
			if enabledStr != "" {
				enabledVal := 1
				if enabledStr == "0" || strings.ToLower(enabledStr) == "false" {
					enabledVal = 0
				}
				_, _ = DB.Exec("UPDATE radius_user_meta SET enabled = ?, admin_id = ? WHERE username = ?", enabledVal, adminID, username)
				if enabledVal == 0 {
					_, _ = DB.Exec("DELETE FROM radcheck WHERE username=? AND attribute='Auth-Type'", username)
					_, _ = DB.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Auth-Type', ':=', 'Reject')", username)
				}
			} else {
				// Default to current admin ownership
				_, _ = DB.Exec("UPDATE radius_user_meta SET admin_id = ? WHERE username = ?", adminID, username)
			}
			
			QueueUserSync(username)
			importCount++
		} else {
			skipCount++
		}
	}

	return c.JSON(fiber.Map{
		"message":        fmt.Sprintf("تم استيراد %d مشترك بنجاح وتخطي %d مشترك.", importCount, skipCount),
		"users_imported": importCount,
		"users_skipped":  skipCount,
	})
}

// ExportToExcel queries all subscribers and streams an Excel file back to the admin
func ExportToExcel(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	adminID, _ := c.Locals("admin_id").(int64)

	var query string
	var args []interface{}

	if role == "superadmin" {
		query = `SELECT 
					c.username, 
					c.value AS password,
					COALESCE(m.full_name, '') AS full_name,
					COALESCE(m.phone, '') AS phone,
					COALESCE(m.balance, 0.0) AS balance,
					COALESCE(m.enabled, 1) AS enabled,
					m.expiration_unix,
					COALESCE(m.created_at, '') AS created_at,
					COALESCE(a.username, 'System') AS admin_name,
					COALESCE(g.groupname, '') AS profile_name,
					COALESCE(r.value, '') AS static_ip
				 FROM radcheck c
				 LEFT JOIN radius_user_meta m ON c.username = m.username
				 LEFT JOIN radius_admins a ON m.admin_id = a.id
				 LEFT JOIN radusergroup g ON c.username = g.username
				 LEFT JOIN radreply r ON c.username = r.username AND r.attribute = 'Framed-IP-Address'
				 WHERE c.attribute = 'Cleartext-Password'
				 ORDER BY c.username`
	} else {
		query = `SELECT 
					c.username, 
					c.value AS password,
					COALESCE(m.full_name, '') AS full_name,
					COALESCE(m.phone, '') AS phone,
					COALESCE(m.balance, 0.0) AS balance,
					COALESCE(m.enabled, 1) AS enabled,
					m.expiration_unix,
					COALESCE(m.created_at, '') AS created_at,
					COALESCE(a.username, 'Sub-Agent') AS admin_name,
					COALESCE(g.groupname, '') AS profile_name,
					COALESCE(r.value, '') AS static_ip
				 FROM radcheck c
				 JOIN radius_user_meta m ON c.username = m.username
				 LEFT JOIN radius_admins a ON m.admin_id = a.id
				 LEFT JOIN radusergroup g ON c.username = g.username
				 LEFT JOIN radreply r ON c.username = r.username AND r.attribute = 'Framed-IP-Address'
				 WHERE c.attribute = 'Cleartext-Password'
				   AND (m.admin_id = ? OR m.admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?))
				 ORDER BY c.username`
		args = append(args, adminID, adminID)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch users: " + err.Error()})
	}
	defer rows.Close()

	// Create excel workbook
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"
	// Ensure Sheet1 is the active sheet
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)

	// Column Headers matching users_20260205111310_3.xlsx template
	headers := []string{
		"id", "username", "firstname", "lastname", "city", "phone", "balance", 
		"expiration", "email", "static_ip", "enabled", "notes", "profile_name", 
		"mac", "parent_name", "ct_password", "address", "contract_id", "created_at", 
		"national_id", "last_online", "group_name", "company", "gps_lat", "gps_lng", "street",
	}

	// Write headers
	for colIdx, header := range headers {
		cellName, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cellName, header)
	}

	rowNum := 2
	for rows.Next() {
		var username, password, fullName, phone, createdAt, adminName, profileName, staticIP string
		var balance float64
		var enabled int
		var expirationUnix sql.NullInt64

		err = rows.Scan(&username, &password, &fullName, &phone, &balance, &enabled, &expirationUnix, &createdAt, &adminName, &profileName, &staticIP)
		if err != nil {
			continue
		}

		// Split full_name into firstname and lastname
		fullName = strings.TrimSpace(fullName)
		parts := strings.SplitN(fullName, " ", 2)
		firstname := ""
		lastname := ""
		if len(parts) > 0 {
			firstname = parts[0]
		}
		if len(parts) > 1 {
			lastname = parts[1]
		}

		// Format Expiration Date
		expirationStr := ""
		if expirationUnix.Valid && expirationUnix.Int64 > 0 {
			t := time.Unix(expirationUnix.Int64, 0).In(baghdadLocation)
			expirationStr = t.Format("2006-01-02 15:04:05")
		}

		// Format CreatedAt Date
		if createdAt != "" {
			// Clean up RFC3339 timestamps if they exist
			for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, createdAt); err == nil {
					createdAt = t.Format("2006-01-02 15:04:05")
					break
				}
			}
		}

		// Row values mapping
		rowValues := map[string]interface{}{
			"id":           rowNum - 1,
			"username":     username,
			"firstname":    firstname,
			"lastname":     lastname,
			"city":         "",
			"phone":        phone,
			"balance":      fmt.Sprintf("%.2f", balance),
			"expiration":   expirationStr,
			"email":        "",
			"static_ip":    staticIP,
			"enabled":      enabled,
			"notes":        "",
			"profile_name": profileName,
			"mac":          "N/A",
			"parent_name":  adminName,
			"ct_password":  password,
			"address":      "",
			"contract_id":  "",
			"created_at":   createdAt,
			"national_id":  "",
			"last_online":  "",
			"group_name":   profileName,
			"company":      "",
			"gps_lat":      "",
			"gps_lng":      "",
			"street":       "",
		}

		for colIdx, header := range headers {
			cellName, _ := excelize.CoordinatesToCellName(colIdx+1, rowNum)
			_ = f.SetCellValue(sheetName, cellName, rowValues[header])
		}
		rowNum++
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate Excel file: " + err.Error()})
	}

	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", "attachment; filename=users_export.xlsx")
	return c.Send(buf.Bytes())
}
