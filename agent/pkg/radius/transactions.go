package radius

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

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
	if !HasPermission(c, "can_manage_transactions") {
		return c.Status(403).JSON(fiber.Map{"error": "🚫 ليس لديك صلاحية لإضافة حركات مالية أو تسديد ديون"})
	}

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
	actName := "إضافة دين لمشترك"
	if req.Type == "payment" {
		msg = "تم تسديد الديون بنجاح"
		actName = "تسديد دين مشترك"
	}

	LogActivityFromCtx(c, actName, username, fmt.Sprintf("%s بمبلغ %.2f للمشترك %s (ملاحظات: %s)", msg, req.Amount, username, req.Notes))

	_, _, balance, _, _, _, _, _, _ := loadUserExtraInfo(username)
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

	fullName, phone, balance, createdAt, _, _, _, _, _ := loadUserExtraInfo(username)

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

	_, _, balance, _, _, _, _, _, _ := loadUserExtraInfo(username)
	return c.JSON(fiber.Map{"balance": balance})
}

func ListAllTransactionsHandler(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	var query string
	var args []interface{}

	if role == "superadmin" {
		query = `SELECT t.id, t.username, t.transaction_type, t.amount, t.notes, t.created_at, COALESCE(a.username, 'admin') as admin_name
		         FROM radius_user_transactions t
		         LEFT JOIN radius_admins a ON t.admin_id = a.id
		         ORDER BY t.created_at DESC LIMIT 500`
	} else {
		query = `SELECT t.id, t.username, t.transaction_type, t.amount, t.notes, t.created_at, COALESCE(a.username, 'admin') as admin_name
		         FROM radius_user_transactions t
		         LEFT JOIN radius_admins a ON t.admin_id = a.id
		         WHERE t.admin_id = ?
		         ORDER BY t.created_at DESC LIMIT 500`
		args = append(args, adminID)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	transactions := make([]map[string]interface{}, 0)
	userTransMap := make(map[string][]map[string]interface{})
	var globalPayments, globalDebts float64

	for rows.Next() {
		var id int
		var username, tType, notes, createdAt, adminName string
		var amount float64
		if err := rows.Scan(&id, &username, &tType, &amount, &notes, &createdAt, &adminName); err != nil {
			continue
		}
		item := map[string]interface{}{
			"id":         id,
			"username":   username,
			"type":       tType,
			"amount":     amount,
			"notes":      notes,
			"created_at": createdAt,
			"admin":      adminName,
		}
		transactions = append(transactions, item)
		userTransMap[username] = append(userTransMap[username], item)

		if tType == "payment" || tType == "تجديد اشتراك" {
			globalPayments += amount
		} else if tType == "debt" {
			globalDebts += amount
		}
	}

	// Fetch all users with their balance and info
	var uQuery string
	var uArgs []interface{}
	if role == "superadmin" {
		uQuery = `SELECT m.username, COALESCE(m.full_name, ''), COALESCE(m.phone, ''), COALESCE(m.balance, 0)
		          FROM radius_user_meta m
		          ORDER BY m.balance DESC, m.username ASC`
	} else {
		uQuery = `SELECT m.username, COALESCE(m.full_name, ''), COALESCE(m.phone, ''), COALESCE(m.balance, 0)
		          FROM radius_user_meta m
		          WHERE m.admin_id = ? OR m.admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?)
		          ORDER BY m.balance DESC, m.username ASC`
		uArgs = append(uArgs, adminID, adminID)
	}

	uRows, err := DB.Query(uQuery, uArgs...)
	userSummaries := make([]map[string]interface{}, 0)
	if err == nil {
		defer uRows.Close()
		for uRows.Next() {
			var uName, fullName, phone string
			var balance float64
			if err := uRows.Scan(&uName, &fullName, &phone, &balance); err != nil {
				continue
			}

			// Calculate total paid & debt for this specific user
			var userPaid, userDebt float64
			for _, t := range userTransMap[uName] {
				amt, _ := t["amount"].(float64)
				tType, _ := t["type"].(string)
				if tType == "payment" || tType == "تجديد اشتراك" {
					userPaid += amt
				} else if tType == "debt" {
					userDebt += amt
				}
			}

			userSummaries = append(userSummaries, map[string]interface{}{
				"username":     uName,
				"full_name":    fullName,
				"phone":        phone,
				"balance":      balance,
				"total_paid":   userPaid,
				"total_debt":   userDebt,
				"transactions": userTransMap[uName],
			})
		}
	}

	return c.JSON(fiber.Map{
		"transactions":    transactions,
		"user_summaries":  userSummaries,
		"global_payments": globalPayments,
		"global_debts":    globalDebts,
	})
}

func AddGlobalTransactionHandler(c *fiber.Ctx) error {
	type reqPayload struct {
		Username string  `json:"username"`
		Type     string  `json:"type"`
		Amount   float64 `json:"amount"`
		Notes    string  `json:"notes"`
	}
	var req reqPayload
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "يرجى إدخال اسم المشترك ومبلغ صالح"})
	}

	if req.Type != "debt" && req.Type != "payment" {
		req.Type = "payment"
	}

	adminID, _ := c.Locals("admin_id").(int64)
	if err := addUserTransaction(req.Username, req.Type, req.Amount, req.Notes, adminID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	LogActivityFromCtx(c, "سجل مالي", req.Username, fmt.Sprintf("تمت إضافة حركة مالية (%s) بمبلغ %.0f د.ع", req.Type, req.Amount))

	return c.JSON(fiber.Map{"message": "تمت إضافة الحركة المالية بنجاح", "success": true})
}
