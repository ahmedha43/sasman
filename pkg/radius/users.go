package radius

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

var baghdadLocation = time.FixedZone("Asia/Baghdad", 3*60*60)
var currentBaghdadTime = func() time.Time {
	return time.Now().In(baghdadLocation)
}

var errProfileNotFound = errors.New("profile not found")

type userPayload struct {
	User      string `json:"user"`
	OldUser   string `json:"old_user"`
	Pass      string `json:"pass"`
	Profile   string `json:"profile"`
	ExpiresAt string `json:"expires_at"`
	FullName  string `json:"full_name"`
	Phone     string `json:"phone"`
	AdminID   int64  `json:"admin_id"` // For superadmin to assign/change owner
}

type renewUserPayload struct {
	Profile string `json:"profile"`
	Paid    bool   `json:"paid"`
}
type UserMeta struct {
	Username       string          `json:"username"`
	ExpirationUnix sql.NullInt64   `json:"expiration_unix"`
	FullName       string          `json:"full_name"`
	Phone          string          `json:"phone"`
	Balance        float64         `json:"balance"`
	AdminID        sql.NullInt64   `json:"admin_id"`
}

func GetUsers(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	var query string
	var args []interface{}

	if role == "superadmin" {
		query = `SELECT c.username, c.value, COALESCE(m.admin_id, 0), COALESCE(a.username, 'System') as admin_name
                 FROM radcheck c
                 LEFT JOIN radius_user_meta m ON c.username = m.username
                 LEFT JOIN radius_admins a ON m.admin_id = a.id
                 WHERE c.attribute='Cleartext-Password' 
                 ORDER BY c.username`
	} else {
		query = `SELECT c.username, c.value, m.admin_id, COALESCE(a.username, 'Sub-Agent') as admin_name 
                 FROM radcheck c 
                 JOIN radius_user_meta m ON c.username = m.username 
                 LEFT JOIN radius_admins a ON m.admin_id = a.id
                 WHERE c.attribute='Cleartext-Password' 
                   AND (m.admin_id = ? OR m.admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?))
                 ORDER BY c.username`
		args = append(args, adminID, adminID)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	users := make([]map[string]interface{}, 0)
	for rows.Next() {
		var username, password, adminName string
		var uAdminID int64
		if err := rows.Scan(&username, &password, &uAdminID, &adminName); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		var profile string
		_ = DB.QueryRow("SELECT groupname FROM radusergroup WHERE username=? ORDER BY priority LIMIT 1", username).Scan(&profile)

		expirationUnix, err := loadUserExpiration(username)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		session := GetSessionForUser(username)
		expired := expirationUnix.Valid && expirationUnix.Int64 > 0 && currentBaghdadTime().Unix() >= expirationUnix.Int64
		if expired && !session.Online {
			session.Status = "expired"
		} else if expired && session.Online {
			session.Status = "expired_online"
		}

		fullName, phone, balance, createdAt, enabled := loadUserExtraInfo(username)

		users = append(users, map[string]interface{}{
			"user":            username,
			"pass":            password,
			"profile":         profile,
			"expires_at":      formatUnixDateTime(expirationUnix),
			"expires_at_unix": nullableIntToJSON(expirationUnix),
			"expired":         expired,
			"session":         session,
			"full_name":       fullName,
			"phone":           phone,
			"balance":         balance,
			"created_at":      createdAt,
			"enabled":         enabled,
			"admin_id":        uAdminID,
			"admin_name":      adminName,
		})
	}

	return c.JSON(users)
}

func CreateUser(c *fiber.Ctx) error {
	var req userPayload
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	req.User = strings.TrimSpace(req.User)
	req.OldUser = strings.TrimSpace(req.OldUser)
	req.Profile = strings.TrimSpace(req.Profile)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.User == "" || req.Pass == "" || req.Profile == "" {
		return c.Status(400).JSON(fiber.Map{"error": "يرجى تعبئة كافة الحقول المطلوبة"})
	}

	if _, err := loadProfileValidityDays(req.Profile); err != nil {
		if errors.Is(err, errProfileNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "الباقة المحددة غير موجودة"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	// Check if new username already exists
	var newUsernameExists bool
	_ = DB.QueryRow("SELECT 1 FROM radcheck WHERE username=? AND attribute='Cleartext-Password'", req.User).Scan(&newUsernameExists)

	// Determine if this is a username change operation
	var usernameChanged bool
	var oldUsername string
	var currentExpiration sql.NullInt64

	if req.OldUser != "" && req.OldUser != req.User {
		// Username is being changed
		usernameChanged = true
		oldUsername = req.OldUser

		// Verify old user exists
		var oldUserExists bool
		_ = DB.QueryRow("SELECT 1 FROM radcheck WHERE username=? AND attribute='Cleartext-Password'", oldUsername).Scan(&oldUserExists)
		if !oldUserExists {
			return c.Status(404).JSON(fiber.Map{"error": "المستخدم الأصلي غير موجود"})
		}

		// Check if new username already exists
		if newUsernameExists {
			return c.Status(400).JSON(fiber.Map{"error": "اسم المستخدم الجديد موجود مسبقاً"})
		}

		// Get current expiration from old user
		_ = DB.QueryRow("SELECT expiration_unix FROM radius_user_meta WHERE username=?", oldUsername).Scan(&currentExpiration)
	} else if newUsernameExists {
		// Editing existing user without changing username
		oldUsername = req.User
		_ = DB.QueryRow("SELECT expiration_unix FROM radius_user_meta WHERE username=?", req.User).Scan(&currentExpiration)
	}

	expirationUnix, err := parseExpiryDate(req.ExpiresAt)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "صيغة تاريخ الانتهاء غير صحيحة"})
	}

	// Ownership check for editing existing user (not username change)
	if !usernameChanged && newUsernameExists && role != "superadmin" {
		var userAdminID *int64
		_ = DB.QueryRow("SELECT admin_id FROM radius_user_meta WHERE username=?", req.User).Scan(&userAdminID)
		if userAdminID == nil || *userAdminID != adminID {
			return c.Status(403).JSON(fiber.Map{"error": "لا تملك صلاحية تعديل هذا المشترك"})
		}
	}

	// Ownership check for username change
	if usernameChanged && role != "superadmin" {
		var userAdminID *int64
		_ = DB.QueryRow("SELECT admin_id FROM radius_user_meta WHERE username=?", oldUsername).Scan(&userAdminID)
		if userAdminID == nil || *userAdminID != adminID {
			return c.Status(403).JSON(fiber.Map{"error": "لا تملك صلاحية تعديل هذا المشترك"})
		}
	}

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer tx.Rollback()

	if usernameChanged {
		// Transfer financial data from old username to new username
		if err := transferUserFinancialDataTx(tx, oldUsername, req.User); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل نقل البيانات المالية: " + err.Error()})
		}

		// Delete old user records
		if _, err := tx.Exec("DELETE FROM radcheck WHERE username=?", oldUsername); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if _, err := tx.Exec("DELETE FROM radreply WHERE username=?", oldUsername); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if _, err := tx.Exec("DELETE FROM radusergroup WHERE username=?", oldUsername); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if _, err := tx.Exec("DELETE FROM radpostauth WHERE username=?", oldUsername); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		if _, err := tx.Exec("DELETE FROM radcheck WHERE username=? AND attribute='Cleartext-Password'", req.User); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if _, err := tx.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Cleartext-Password', ':=', ?)", req.User, req.Pass); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := saveUserExpirationTx(tx, req.User, expirationUnix); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := replaceUserProfileTx(tx, req.User, req.Profile); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	targetAdminID := adminID
	if role == "superadmin" && req.AdminID > 0 {
		targetAdminID = req.AdminID
	}

	if err := saveUserMetaWithInfoTx(tx, req.User, expirationUnix, req.FullName, req.Phone, targetAdminID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Sync to LMDB for FreeRADIUS
	QueueUserSync(req.User)
	if usernameChanged {
		QueueUserSync(oldUsername)
	}

	// Automatically kick if updated so they reconnect and take the new settings/expiration
	go KickUserIfOnline(req.User)
	if usernameChanged {
		go KickUserIfOnline(oldUsername)
	}

	message := "تم حفظ المشترك بنجاح"
	if usernameChanged {
		message = "تم تحديث المشترك ونقل البيانات المالية بنجاح"
	}

	return c.JSON(fiber.Map{"message": message})
}

func RenewUser(c *fiber.Ctx) error {
	rawUser := c.Params("user")
	username, _ := url.PathUnescape(rawUser)
	log.Printf("[radius] Renew request: raw=[%s], decoded=[%s]", rawUser, username)
	username = strings.TrimSpace(username)
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم المستخدم غير صالح"})
	}

	var req renewUserPayload
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	req.Profile = strings.TrimSpace(req.Profile)
	if req.Profile == "" {
		return c.Status(400).JSON(fiber.Map{"error": "يرجى اختيار باقة التجديد"})
	}

	validityDays, err := loadProfileValidityDays(req.Profile)
	if err != nil {
		if errors.Is(err, errProfileNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "الباقة المحددة غير موجودة"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	profilePrice, _ := loadProfilePrice(req.Profile)

	currentExpiration, err := loadUserExpiration(username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	newExpiration := calculateRenewedExpiration(currentExpiration, validityDays)

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer tx.Rollback()

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	exists, err := userExistsTx(tx, username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if !exists {
		return c.Status(404).JSON(fiber.Map{"error": "المشترك غير موجود"})
	}

	// Ownership check
	if role != "superadmin" {
		var userAdminID *int64
		err := tx.QueryRow("SELECT admin_id FROM radius_user_meta WHERE username=?", username).Scan(&userAdminID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "User metadata not found"})
		}
		if userAdminID == nil || *userAdminID != adminID {
			return c.Status(403).JSON(fiber.Map{"error": "You do not own this user"})
		}
	}

	if err := replaceUserProfileTx(tx, username, req.Profile); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := saveUserExpirationTx(tx, username, newExpiration); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := saveUserMetaTx(tx, username, newExpiration, adminID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// NEW LOGIC: Deduct from agent balance if the performing admin is an agent
	if role != "superadmin" {
		agentPrice, _ := loadProfileAgentPrice(req.Profile)
		if agentPrice > 0 {
			if err := DeductAdminBalance(adminID, agentPrice, fmt.Sprintf("تجديد المشترك %s باقة %s", username, req.Profile)); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}

	// Sync to LMDB for FreeRADIUS (Queued)
	QueueUserSync(username)

	// Automatically kick if renewed so they reconnect and take the new package/expiration
	go KickUserIfOnline(username)

	_, _, balance, _, _ := loadUserExtraInfo(username)

	if profilePrice > 0 && !req.Paid {
		if err := addUserTransaction(username, "debt", profilePrice, fmt.Sprintf("تجديد باقة %s (%d يوم)", req.Profile, validityDays), adminID); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		_, _, balance, _, _ = loadUserExtraInfo(username)
		go SendWhatsappNotification(username, "renew_debt", map[string]string{
			"username":      username,
			"profile":       req.Profile,
			"price":         fmt.Sprintf("%.0f", profilePrice),
			"validity_days": fmt.Sprintf("%d", validityDays),
			"balance":       fmt.Sprintf("%.0f", balance),
		})
	} else if profilePrice > 0 && req.Paid {
		go SendWhatsappNotification(username, "renew_paid", map[string]string{
			"username":      username,
			"profile":       req.Profile,
			"price":         fmt.Sprintf("%.0f", profilePrice),
			"validity_days": fmt.Sprintf("%d", validityDays),
			"balance":       fmt.Sprintf("%.0f", balance),
		})
	} else {
		go SendWhatsappNotification(username, "renew_paid", map[string]string{
			"username":      username,
			"profile":       req.Profile,
			"price":         "0",
			"validity_days": fmt.Sprintf("%d", validityDays),
			"balance":       fmt.Sprintf("%.0f", balance),
		})
	}

	return c.JSON(fiber.Map{
		"message":    "تم تجديد المشترك بنجاح",
		"expires_at": formatUnixDateTime(newExpiration),
		"price":      profilePrice,
	})
}

func DeleteUser(c *fiber.Ctx) error {
	rawUser := c.Params("user")
	username, _ := url.PathUnescape(rawUser)
	log.Printf("[radius] Delete request: raw=[%s], decoded=[%s]", rawUser, username)
	username = strings.TrimSpace(username)

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	// Ownership check
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

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer tx.Rollback()

	rowsAffected, err := deleteUserRecordsTx(tx, username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	log.Printf("[radius] Deleted user records for [%s], rows affected: %d", username, rowsAffected)

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Remove from LMDB
	QueueUserSync(username)

	return c.JSON(fiber.Map{"message": "تم حذف المستخدم بنجاح"})
}

func DisconnectUser(c *fiber.Ctx) error {
	rawUser := c.Params("user")
	username, _ := url.PathUnescape(rawUser)
	username = strings.TrimSpace(username)
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم المستخدم مطلوب"})
	}

	sessions, _ := LoadSessionsFromDB()
	key := strings.ToLower(strings.TrimSpace(username))
	info, ok := sessions[key]

	if !ok || (!info.Online && !info.Stale) {
		return c.Status(400).JSON(fiber.Map{"error": "لا توجد جلسة نشطة لهذا المشترك"})
	}

	targetName := info.Username
	if targetName == "" {
		targetName = username
	}

	if err := DisconnectUserSession(targetName, info); err != nil {
		log.Printf("[radius] Router disconnect failed for %s: %v. Falling back to DB closure.", targetName, err)
		if info.SessionID != "" {
			_, _ = DB.Exec(`UPDATE radacct SET acctstoptime = CURRENT_TIMESTAMP, acctterminatecause = 'Admin-Reset' 
			               WHERE acctsessionid = ? AND acctstoptime IS NULL`, info.SessionID)
		} else {
			_, _ = DB.Exec(`UPDATE radacct SET acctstoptime = CURRENT_TIMESTAMP, acctterminatecause = 'Admin-Reset' 
			               WHERE username = ? AND acctstoptime IS NULL`, targetName)
		}
	}

	InvalidateSessionCache()
	return c.JSON(fiber.Map{"message": "تمت معالجة طلب الفصل"})
}

func ToggleUserStatus(c *fiber.Ctx) error {
	rawUser := c.Params("user")
	username, _ := url.PathUnescape(rawUser)
	username = strings.TrimSpace(username)
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم المستخدم مطلوب"})
	}

	var currentEnabled int
	err := DB.QueryRow("SELECT enabled FROM radius_user_meta WHERE username=?", username).Scan(&currentEnabled)
	if err != nil && err != sql.ErrNoRows {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if err == sql.ErrNoRows {
		currentEnabled = 1 // Default
	}

	newStatus := 0
	if currentEnabled == 0 {
		newStatus = 1
	}

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO radius_user_meta (username, enabled, updated_at) 
	                 VALUES (?, ?, CURRENT_TIMESTAMP)
	                 ON CONFLICT(username) DO UPDATE SET enabled=excluded.enabled, updated_at=CURRENT_TIMESTAMP`,
		username, newStatus)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Update radcheck to block/unblock authentication in FreeRADIUS
	if newStatus == 0 {
		// Disable: Add Auth-Type := Reject
		_, _ = tx.Exec("DELETE FROM radcheck WHERE username=? AND attribute='Auth-Type'", username)
		_, err = tx.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Auth-Type', ':=', 'Reject')", username)
	} else {
		// Enable: Remove Auth-Type := Reject
		_, err = tx.Exec("DELETE FROM radcheck WHERE username=? AND attribute='Auth-Type'", username)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Sync to LMDB
	QueueUserSync(username)

	// If disabled, also kick the user if they are online
	if newStatus == 0 {
		sessions, _ := LoadSessionsFromDB()
		key := strings.ToLower(strings.TrimSpace(username))
		if info, ok := sessions[key]; ok && (info.Online || info.Stale) {
			go func() {
				targetName := info.Username
				if targetName == "" {
					targetName = username
				}
				_ = DisconnectUserSession(targetName, info)
				InvalidateSessionCache()
			}()
		}
	}

	msg := "تم تنشيط المستخدم"
	if newStatus == 0 {
		msg = "تم إيقاف المستخدم وفصله"
	}
	fmt.Printf("[radius] User %s status toggled to: %d\n", username, newStatus)
	return c.JSON(fiber.Map{"message": msg, "enabled": newStatus == 1})
}

func transferUserFinancialDataTx(tx *sql.Tx, oldUsername, newUsername string) error {
	// 1. Rename username in radius_user_meta to preserve balance, created_at, enabled, and all metadata!
	_, err := tx.Exec("UPDATE radius_user_meta SET username = ? WHERE username = ?", newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("فشل نقل بيانات المستخدم الأساسية: %v", err)
	}

	// 2. Transfer transactions from radius_user_transactions to new username
	_, err = tx.Exec("UPDATE radius_user_transactions SET username = ? WHERE username = ?", newUsername, oldUsername)
	if err != nil {
		return fmt.Errorf("فشل نقل المعاملات: %v", err)
	}

	// 3. Transfer accounting history (radacct)
	_, _ = tx.Exec("UPDATE radacct SET username = ? WHERE username = ?", newUsername, oldUsername)

	// 4. Transfer postauth history (radpostauth)
	_, _ = tx.Exec("UPDATE radpostauth SET username = ? WHERE username = ?", newUsername, oldUsername)

	return nil
}

func deleteUserRecordsTx(tx *sql.Tx, username string) (int64, error) {
	var totalAffected int64
	for _, query := range []string{
		"DELETE FROM radcheck WHERE username=?",
		"DELETE FROM radreply WHERE username=?",
		"DELETE FROM radusergroup WHERE username=?",
		"DELETE FROM radius_user_meta WHERE username=?",
		"DELETE FROM radius_user_transactions WHERE username=?",
		"DELETE FROM radacct WHERE username=?",
		"DELETE FROM radpostauth WHERE username=?",
	} {
		res, err := tx.Exec(query, username)
		if err != nil {
			return 0, err
		}
		affected, _ := res.RowsAffected()
		totalAffected += affected
	}
	return totalAffected, nil
}

func userExistsTx(tx *sql.Tx, username string) (bool, error) {
	var count int
	if err := tx.QueryRow(
		"SELECT COUNT(1) FROM radcheck WHERE username=? AND attribute='Cleartext-Password'",
		username,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func replaceUserProfileTx(tx *sql.Tx, username, profile string) error {
	if _, err := tx.Exec("DELETE FROM radusergroup WHERE username=?", username); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES (?, ?, 1)", username, profile); err != nil {
		return err
	}
	return nil
}

func saveUserExpirationTx(tx *sql.Tx, username string, expirationUnix sql.NullInt64) error {
	if _, err := tx.Exec("DELETE FROM radcheck WHERE username=? AND attribute='Expiration'", username); err != nil {
		return err
	}
	if !expirationUnix.Valid {
		return nil
	}
	_, err := tx.Exec(
		"INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Expiration', ':=', ?)",
		username,
		strconv.FormatInt(expirationUnix.Int64, 10),
	)
	return err
}

func saveUserMetaTx(tx *sql.Tx, username string, expirationUnix sql.NullInt64, adminID int64) error {
	_, err := tx.Exec(
		`INSERT INTO radius_user_meta (username, expiration_unix, renewal_enabled, renewal_profile, admin_id, updated_at)
		 VALUES (?, ?, 0, '', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(username) DO UPDATE SET
		 	expiration_unix=excluded.expiration_unix,
		 	renewal_enabled=0,
		 	renewal_profile='',
		 	admin_id=excluded.admin_id,
		 	updated_at=CURRENT_TIMESTAMP`,
		username,
		nullIntValue(expirationUnix),
		adminID,
	)
	return err
}

func saveUserMetaWithInfoTx(tx *sql.Tx, username string, expirationUnix sql.NullInt64, fullName, phone string, adminID int64) error {
	_, err := tx.Exec(
		`INSERT INTO radius_user_meta (username, expiration_unix, renewal_enabled, renewal_profile, full_name, phone, admin_id, updated_at)
		 VALUES (?, ?, 0, '', ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(username) DO UPDATE SET
		 	expiration_unix=excluded.expiration_unix,
		 	renewal_enabled=0,
		 	renewal_profile='',
		 	full_name=excluded.full_name,
		 	phone=excluded.phone,
		 	admin_id=excluded.admin_id,
		 	updated_at=CURRENT_TIMESTAMP`,
		username,
		nullIntValue(expirationUnix),
		fullName,
		phone,
		adminID,
	)
	return err
}

func loadUserExpiration(username string) (sql.NullInt64, error) {
	var expirationUnix sql.NullInt64

	err := DB.QueryRow(
		"SELECT expiration_unix FROM radius_user_meta WHERE username=?",
		username,
	).Scan(&expirationUnix)
	if err != nil && err != sql.ErrNoRows {
		return sql.NullInt64{}, err
	}

	if err == sql.ErrNoRows || !expirationUnix.Valid {
		legacyExpiration, legacyErr := loadLegacyExpiration(username)
		if legacyErr != nil {
			return sql.NullInt64{}, legacyErr
		}
		expirationUnix = legacyExpiration
	}

	return expirationUnix, nil
}

func loadProfileValidityDays(profile string) (int, error) {
	var validityDays int
	err := DB.QueryRow("SELECT validity_days FROM radius_profile_meta WHERE groupname=?", profile).Scan(&validityDays)
	if err == sql.ErrNoRows {
		return 0, errProfileNotFound
	} else if err != nil {
		return 0, err
	}

	return validityDays, nil
}

func loadProfilePrice(profile string) (float64, error) {
	var price float64
	err := DB.QueryRow("SELECT price FROM radius_profile_meta WHERE groupname=?", profile).Scan(&price)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return price, nil
}

func loadProfileAgentPrice(profile string) (float64, error) {
	var price float64
	err := DB.QueryRow("SELECT agent_price FROM radius_profile_meta WHERE groupname=?", profile).Scan(&price)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return price, nil
}

func loadUserExtraInfo(username string) (string, string, float64, string, bool) {
	var fullName, phone, createdAt string
	var balance float64
	var enabled int
	DB.QueryRow(
		"SELECT COALESCE(full_name,''), COALESCE(phone,''), COALESCE(balance,0), COALESCE(created_at,''), COALESCE(enabled,1) FROM radius_user_meta WHERE username=?",
		username,
	).Scan(&fullName, &phone, &balance, &createdAt, &enabled)
	return fullName, phone, balance, createdAt, enabled == 1
}

func calculateRenewedExpiration(current sql.NullInt64, validityDays int) sql.NullInt64 {
	if validityDays <= 0 {
		return sql.NullInt64{}
	}

	base := currentBaghdadTime()
	base = time.Date(base.Year(), base.Month(), base.Day(), base.Hour(), base.Minute(), 0, 0, baghdadLocation)
	if current.Valid && current.Int64 > base.Unix() {
		base = time.Unix(current.Int64, 0).In(baghdadLocation)
	}

	expiration := base.AddDate(0, 0, validityDays)

	return sql.NullInt64{Int64: expiration.Unix(), Valid: true}
}

func loadLegacyExpiration(username string) (sql.NullInt64, error) {
	var raw string
	err := DB.QueryRow(
		"SELECT value FROM radcheck WHERE username=? AND attribute='Expiration' LIMIT 1",
		username,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return sql.NullInt64{}, nil
	}
	if err != nil {
		return sql.NullInt64{}, err
	}

	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return sql.NullInt64{}, nil
	}

	return sql.NullInt64{Int64: value, Valid: true}, nil
}

func parseExpiryDate(raw string) (sql.NullInt64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sql.NullInt64{}, nil
	}

	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	} {
		dateValue, err := time.ParseInLocation(layout, raw, baghdadLocation)
		if err == nil {
			return sql.NullInt64{Int64: dateValue.Unix(), Valid: true}, nil
		}
	}

	dateValue, err := time.ParseInLocation("2006-01-02", raw, baghdadLocation)
	if err != nil {
		return sql.NullInt64{}, err
	}

	endOfDay := time.Date(dateValue.Year(), dateValue.Month(), dateValue.Day(), 23, 59, 59, 0, baghdadLocation)
	return sql.NullInt64{Int64: endOfDay.Unix(), Valid: true}, nil
}

func formatUnixDateTime(value sql.NullInt64) string {
	if !value.Valid || value.Int64 <= 0 {
		return ""
	}
	return time.Unix(value.Int64, 0).In(baghdadLocation).Format("2006-01-02 15:04")
}

func nullableIntToJSON(value sql.NullInt64) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullIntValue(value sql.NullInt64) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

// --- User Portal Handlers ---

func PortalLoginHandler(c *fiber.Ctx) error {
	type LoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	var storedPass string
	err := DB.QueryRow("SELECT value FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", req.Username).Scan(&storedPass)
	if err == sql.ErrNoRows || storedPass != req.Password {
		return c.Status(401).JSON(fiber.Map{"error": "اسم المستخدم أو كلمة المرور غير صحيحة"})
	}

	return c.JSON(fiber.Map{
		"message":  "تم تسجيل الدخول بنجاح",
		"username": req.Username,
		"token":    req.Username, // Simple token for portal
	})
}

func PortalStatusHandler(c *fiber.Ctx) error {
	username := c.Query("username")
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing username"})
	}

	var m UserMeta
	var expirationUnix sql.NullInt64
	var groupName sql.NullString
	
	err := DB.QueryRow(`
		SELECT m.full_name, m.phone, m.balance, m.expiration_unix, COALESCE(g.groupname, '')
		FROM radius_user_meta m
		LEFT JOIN radusergroup g ON m.username = g.username
		WHERE m.username = ?`, username).Scan(&m.FullName, &m.Phone, &m.Balance, &expirationUnix, &groupName)

	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	status := "منتهي"
	expiryStr := "غير محدد"
	if expirationUnix.Valid {
		if expirationUnix.Int64 > time.Now().Unix() {
			status = "نشط"
		}
		expiryStr = time.Unix(expirationUnix.Int64, 0).In(baghdadLocation).Format("2006-01-02 15:04")
	}

	var totalIn, totalOut int64
	_ = DB.QueryRow("SELECT SUM(acctinputoctets), SUM(acctoutputoctets) FROM radacct WHERE username = ?", username).Scan(&totalIn, &totalOut)

	return c.JSON(fiber.Map{
		"username":   username,
		"full_name":  m.FullName,
		"status":     status,
		"expiry":     expiryStr,
		"balance":    m.Balance,
		"profile":    groupName.String,
		"usage_in":   totalIn,
		"usage_out":  totalOut,
		"usage_total": totalIn + totalOut,
	})
}

func PortalChangePasswordHandler(c *fiber.Ctx) error {
	type Req struct {
		Username string `json:"username"`
		OldPass  string `json:"old_password"`
		NewPass  string `json:"new_password"`
	}
	var req Req
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	var storedPass string
	err := DB.QueryRow("SELECT value FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", req.Username).Scan(&storedPass)
	if err != nil || storedPass != req.OldPass {
		return c.Status(401).JSON(fiber.Map{"error": "كلمة المرور الحالية غير صحيحة"})
	}

	_, err = DB.Exec("UPDATE radcheck SET value = ? WHERE username = ? AND attribute = 'Cleartext-Password'", req.NewPass, req.Username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update password"})
	}

	return c.JSON(fiber.Map{"message": "تم تغيير كلمة المرور بنجاح"})
}

// KickUserIfOnline checks if the user is online and sends a RADIUS disconnect request
func KickUserIfOnline(username string) {
	sessions := loadSessionsCached()
	if info, ok := sessions[strings.ToLower(username)]; ok && info.Online {
		targetName := info.Username
		if targetName == "" {
			targetName = username
		}
		log.Printf("[radius-kick] Automatically disconnecting online user %s after renewal/edit...", targetName)
		if err := DisconnectUserSession(targetName, info); err != nil {
			log.Printf("[radius-kick] Auto disconnect failed for %s: %v", targetName, err)
		} else {
			log.Printf("[radius-kick] Disconnected online user %s successfully", targetName)
		}
	}
}
