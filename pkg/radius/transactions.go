package radius

import (
	"database/sql"
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v2"
)

type transactionPayload struct {
	Type   string  `json:"type"`
	Amount float64 `json:"amount"`
	Notes  string  `json:"notes"`
}

func addUserTransaction(username, tType string, amount float64, notes string, adminID int64) error {
	if amount <= 0 {
		return nil
	}
	_, err := DB.Exec(
		`INSERT INTO radius_user_transactions (username, transaction_type, amount, notes, admin_id, created_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		username, tType, amount, notes, adminID,
	)
	if err != nil {
		return err
	}

	var balanceChange float64
	switch tType {
	case "payment":
		balanceChange = -amount
	case "debt":
		balanceChange = amount
	}

	if balanceChange != 0 {
		_, err = DB.Exec(
			`UPDATE radius_user_meta SET balance = balance + ? WHERE username = ?`,
			balanceChange, username,
		)
	}
	return err
}

func GetUserTransactions(c *fiber.Ctx) error {
	rawUser := c.Params("user")
	username, _ := url.PathUnescape(rawUser)

	rows, err := DB.Query(
		`SELECT id, transaction_type, amount, notes, created_at
		 FROM radius_user_transactions
		 WHERE username = ?
		 ORDER BY created_at DESC`,
		username,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	transactions := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int
		var tType, notes, createdAt string
		var amount float64
		if err := rows.Scan(&id, &tType, &amount, &notes, &createdAt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		transactions = append(transactions, map[string]interface{}{
			"id":         id,
			"type":       tType,
			"amount":     amount,
			"notes":      notes,
			"created_at": createdAt,
		})
	}

	return c.JSON(transactions)
}

func AddTransaction(c *fiber.Ctx) error {
	rawUser := c.Params("user")
	username, _ := url.PathUnescape(rawUser)

	var req transactionPayload
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	if req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "المبلغ يجب أن يكون أكبر من صفر"})
	}

	if req.Type != "debt" && req.Type != "payment" {
		return c.Status(400).JSON(fiber.Map{"error": "نوع العملية غير صالح (debt أو payment)"})
	}

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	// Ownership check for the user
	if role != "superadmin" {
		var userAdminID *int64
		err := DB.QueryRow("SELECT admin_id FROM radius_user_meta WHERE username=?", username).Scan(&userAdminID)
		if err != nil && err != sql.ErrNoRows {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if userAdminID == nil || *userAdminID != adminID {
			return c.Status(403).JSON(fiber.Map{"error": "You do not own this user"})
		}
	}

	err := addUserTransaction(username, req.Type, req.Amount, req.Notes, adminID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	msg := "تم إضافة الديون بنجاح"
	if req.Type == "payment" {
		msg = "تم تسديد الديون بنجاح"
	}

	_, _, balance, _, _ := loadUserExtraInfo(username)
	templateKey := "add_debt"
	if req.Type == "payment" {
		templateKey = "payment"
	}
	go SendWhatsappNotification(username, templateKey, map[string]string{
		"username": username,
		"amount":   fmt.Sprintf("%.0f", req.Amount),
		"notes":    req.Notes,
		"balance":  fmt.Sprintf("%.0f", balance),
	})

	return c.JSON(fiber.Map{"message": msg})
}

func GetUserDetails(c *fiber.Ctx) error {
	rawUser := c.Params("user")
	username, _ := url.PathUnescape(rawUser)

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	var password string
	var profile string
	var userAdminID *int64
	err := DB.QueryRow(
		"SELECT r.value, m.admin_id FROM radcheck r JOIN radius_user_meta m ON r.username = m.username WHERE r.username=? AND r.attribute='Cleartext-Password'",
		username,
	).Scan(&password, &userAdminID)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "المشترك غير موجود"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Ownership check
	if role != "superadmin" {
		if userAdminID == nil || *userAdminID != adminID {
			return c.Status(403).JSON(fiber.Map{"error": "You do not own this user"})
		}
	}

	_ = DB.QueryRow("SELECT groupname FROM radusergroup WHERE username=? ORDER BY priority LIMIT 1", username).Scan(&profile)

	expirationUnix, err := loadUserExpiration(username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	fullName, phone, balance, createdAt, _ := loadUserExtraInfo(username)

	session := GetSessionForUser(username)
	expired := expirationUnix.Valid && expirationUnix.Int64 > 0 && currentBaghdadTime().Unix() >= expirationUnix.Int64
	if expired && !session.Online {
		session.Status = "expired"
	} else if expired && session.Online {
		session.Status = "expired_online"
	}

	// Get transactions
	transRows, err := DB.Query(
		`SELECT id, transaction_type, amount, notes, created_at
		 FROM radius_user_transactions
		 WHERE username = ?
		 ORDER BY created_at DESC LIMIT 50`,
		username,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer transRows.Close()

	transactions := make([]map[string]interface{}, 0)
	for transRows.Next() {
		var id int
		var tType, notes, tCreated string
		var amount float64
		if err := transRows.Scan(&id, &tType, &amount, &notes, &tCreated); err != nil {
			continue
		}
		transactions = append(transactions, map[string]interface{}{
			"id":         id,
			"type":       tType,
			"amount":     amount,
			"notes":      notes,
			"created_at": tCreated,
		})
	}

	// Get session history
	sessRows, err := DB.Query(
		`SELECT acctstarttime, acctstoptime, framedipaddress, acctinputoctets, acctoutputoctets, acctsessiontime, nasipaddress, callingstationid
		 FROM radacct
		 WHERE username = ?
		 ORDER BY acctstarttime DESC LIMIT 50`,
		username,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer sessRows.Close()

	sessions := make([]map[string]interface{}, 0)
	for sessRows.Next() {
		var start, stop, ip, nasIP, callingStation sql.NullString
		var inputOctets, outputOctets, sessionTime sql.NullInt64
		if err := sessRows.Scan(&start, &stop, &ip, &inputOctets, &outputOctets, &sessionTime, &nasIP, &callingStation); err != nil {
			continue
		}
		sessions = append(sessions, map[string]interface{}{
			"started_at":      normalizeAcctTime(start),
			"stopped_at":      normalizeAcctTime(stop),
			"ip":              ip.String,
			"download":        humanBytes(inputOctets.Int64),
			"upload":          humanBytes(outputOctets.Int64),
			"session_time":    sessionTime.Int64,
			"nas_ip":          nasIP.String,
			"calling_station": callingStation.String,
		})
	}

	return c.JSON(fiber.Map{
		"user":            username,
		"pass":            password,
		"profile":         profile,
		"expires_at":      formatUnixDateTime(expirationUnix),
		"expired":         expired,
		"session":         session,
		"full_name":       fullName,
		"phone":           phone,
		"balance":         balance,
		"created_at":      createdAt,
		"transactions":    transactions,
		"session_history": sessions,
	})
}

func GetUserBalance(c *fiber.Ctx) error {
	rawUser := c.Params("user")
	username, _ := url.PathUnescape(rawUser)

	_, _, balance, _, _ := loadUserExtraInfo(username)
	return c.JSON(fiber.Map{"balance": balance})
}
