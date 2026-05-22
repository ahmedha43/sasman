package radius

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/gofiber/fiber/v2"
)

const (
	adminSessionTTL    = 12 * time.Hour
	adminSessionCookie = "sasman_admin_session"
)

type Admin struct {
	ID                int64     `json:"id"`
	Username          string    `json:"username"`
	Name              string    `json:"name"`
	Email             string    `json:"email"`
	Role              string    `json:"role"`
	ParentID          *int64    `json:"parent_id"`
	Balance           float64   `json:"balance"`
	CanManageProfiles bool      `json:"can_manage_profiles"`
	CanManageNas      bool      `json:"can_manage_nas"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func EnsureDefaultAdmin() {
	if DB == nil {
		return
	}
	var count int
	if err := DB.QueryRow("SELECT COUNT(*) FROM radius_admins").Scan(&count); err != nil {
		log.Printf("[admins] count error: %v", err)
		return
	}
	if count > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[admins] hash error: %v", err)
		return
	}
	_, err = DB.Exec(`INSERT INTO radius_admins (username, password_hash, name, email) VALUES (?, ?, ?, ?)`,
		"admin", string(hash), "مدير النظام", "")
	if err != nil {
		log.Printf("[admins] seed error: %v", err)
		return
	}
	log.Println("[admins] default admin created: admin/admin (please change password)")
}

func RequireAdmin(c *fiber.Ctx) error {
	token := c.Cookies(adminSessionCookie)
	if token == "" {
		return c.Status(401).JSON(fiber.Map{"error": "غير مصرح", "auth_required": true})
	}
	var adminID int64
	var role string
	var parentID sql.NullInt64
	var expiresAt string
	var canProfiles, canNas int
	if err := DB.QueryRow(`SELECT a.id, a.role, a.parent_id, s.expires_at, a.can_manage_profiles, a.can_manage_nas
	                       FROM radius_admins a
	                       JOIN radius_admin_sessions s ON a.id = s.admin_id
	                       WHERE s.token=?`, token).
		Scan(&adminID, &role, &parentID, &expiresAt, &canProfiles, &canNas); err != nil {
		log.Printf("[auth] Session not found in DB: %v", err)
		return c.Status(401).JSON(fiber.Map{"error": "الجلسة غير صالحة", "auth_required": true})
	}

	// Use flexible parser for different DB time formats
	expiry := parseDBTime(expiresAt)
	if expiry.IsZero() {
		log.Printf("[auth] Expiry parse error for value: [%s]", expiresAt)
		return c.Status(401).JSON(fiber.Map{"error": "خطأ في تاريخ الجلسة", "auth_required": true})
	}

	now := time.Now().UTC()
	if now.After(expiry) {
		log.Printf("[auth] Session expired: %s < %s", expiresAt, now.Format("2006-01-02 15:04:05"))
		_, _ = DB.Exec(`DELETE FROM radius_admin_sessions WHERE token=?`, token)
		return c.Status(401).JSON(fiber.Map{"error": "انتهت صلاحية الجلسة", "auth_required": true})
	}

	c.Locals("admin_id", adminID)
	c.Locals("role", role)
	c.Locals("can_manage_profiles", canProfiles == 1)
	c.Locals("can_manage_nas", canNas == 1)
	if parentID.Valid {
		c.Locals("parent_id", parentID.Int64)
	} else {
		c.Locals("parent_id", nil)
	}
	return c.Next()
}

func GetAdminByUsername(username string) (*Admin, string, error) {
	row := DB.QueryRow(`SELECT id, username, password_hash, name, email, role, parent_id, balance, can_manage_profiles, can_manage_nas, created_at, updated_at
                        FROM radius_admins WHERE username = ? LIMIT 1`, username)
	var a Admin
	var hash string
	var createdAt, updatedAt string
	var canProfiles, canNas int
	if err := row.Scan(&a.ID, &a.Username, &hash, &a.Name, &a.Email, &a.Role, &a.ParentID, &a.Balance, &canProfiles, &canNas, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}
	a.CanManageProfiles = canProfiles == 1
	a.CanManageNas = canNas == 1
	a.CreatedAt = parseDBTime(createdAt)
	a.UpdatedAt = parseDBTime(updatedAt)
	return &a, hash, nil
}

func GetAdminByID(id int64) (*Admin, error) {
	row := DB.QueryRow(`SELECT id, username, name, email, role, parent_id, balance, can_manage_profiles, can_manage_nas, created_at, updated_at
                        FROM radius_admins WHERE id = ? LIMIT 1`, id)
	var a Admin
	var createdAt, updatedAt string
	var canProfiles, canNas int
	if err := row.Scan(&a.ID, &a.Username, &a.Name, &a.Email, &a.Role, &a.ParentID, &a.Balance, &canProfiles, &canNas, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	a.CanManageProfiles = canProfiles == 1
	a.CanManageNas = canNas == 1
	a.CreatedAt = parseDBTime(createdAt)
	a.UpdatedAt = parseDBTime(updatedAt)
	return &a, nil
}

func CreateAdminAccount(username, password, name, email, role string, parentID *int64, canProfiles, canNas bool) (*Admin, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, fmt.Errorf("اسم المستخدم وكلمة المرور مطلوبة")
	}
	if len(password) < 6 {
		return nil, fmt.Errorf("كلمة المرور يجب ألا تقل عن 6 أحرف")
	}
	if role == "" {
		role = "agent"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	
	cp, cn := 0, 0
	if canProfiles { cp = 1 }
	if canNas { cn = 1 }

	res, err := DB.Exec(`INSERT INTO radius_admins (username, password_hash, name, email, role, parent_id, can_manage_profiles, can_manage_nas) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		username, string(hash), name, email, role, parentID, cp, cn)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("اسم المستخدم مستخدم مسبقًا")
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetAdminByID(id)
}

func UpdateAdminProfile(id int64, name, email string, canProfiles, canNas bool) (*Admin, error) {
	cp, cn := 0, 0
	if canProfiles { cp = 1 }
	if canNas { cn = 1 }

	_, err := DB.Exec(`UPDATE radius_admins SET name=?, email=?, can_manage_profiles=?, can_manage_nas=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		name, email, cp, cn, id)
	if err != nil {
		return nil, err
	}
	return GetAdminByID(id)
}

func ListAdmins(requesterID int64, requesterRole string) ([]Admin, error) {
	var rows *sql.Rows
	var err error

	if requesterRole == "superadmin" {
		rows, err = DB.Query(`SELECT id, username, name, email, role, parent_id, balance, can_manage_profiles, can_manage_nas, created_at, updated_at
		                       FROM radius_admins ORDER BY id ASC`)
	} else {
		rows, err = DB.Query(`SELECT id, username, name, email, role, parent_id, balance, can_manage_profiles, can_manage_nas, created_at, updated_at
		                       FROM radius_admins WHERE parent_id = ? OR id = ? ORDER BY id ASC`, requesterID, requesterID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Admin{}
	for rows.Next() {
		var a Admin
		var createdAt, updatedAt string
		var canProfiles, canNas int
		if err := rows.Scan(&a.ID, &a.Username, &a.Name, &a.Email, &a.Role, &a.ParentID, &a.Balance, &canProfiles, &canNas, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		a.CanManageProfiles = canProfiles == 1
		a.CanManageNas = canNas == 1
		a.CreatedAt = parseDBTime(createdAt)
		a.UpdatedAt = parseDBTime(updatedAt)
		out = append(out, a)
	}
	return out, nil
}

func DeleteAdminByID(id int64, requesterID int64, requesterRole string) error {
	var count int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM radius_admins`).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return fmt.Errorf("لا يمكن حذف آخر حساب مدير")
	}

	// Permission check
	if requesterRole != "superadmin" {
		var parentID sql.NullInt64
		err := DB.QueryRow(`SELECT parent_id FROM radius_admins WHERE id = ?`, id).Scan(&parentID)
		if err != nil {
			return err
		}
		if !parentID.Valid || parentID.Int64 != requesterID {
			return fmt.Errorf("غير مصرح لك بحذف هذا الوكيل")
		}
	}

	_, err := DB.Exec(`DELETE FROM radius_admin_sessions WHERE admin_id=?`, id)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`DELETE FROM radius_admins WHERE id=?`, id)
	return err
}

func ChangeAdminPassword(id int64, currentPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return fmt.Errorf("كلمة المرور الجديدة يجب ألا تقل عن 6 أحرف")
	}
	var hash string
	if err := DB.QueryRow(`SELECT password_hash FROM radius_admins WHERE id=?`, id).Scan(&hash); err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("كلمة المرور الحالية غير صحيحة")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`UPDATE radius_admins SET password_hash=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		string(newHash), id)
	return err
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func parseDBTime(v string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

func RechargeAdmin(adminID int64, amount float64) error {
	if amount == 0 {
		return nil
	}
	_, err := DB.Exec(`UPDATE radius_admins SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, amount, adminID)
	return err
}

func DeductAdminBalance(adminID int64, amount float64, notes string) error {
	if amount <= 0 {
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var balance float64
	err = tx.QueryRow("SELECT balance FROM radius_admins WHERE id = ?", adminID).Scan(&balance)
	if err != nil {
		return err
	}

	if balance < amount {
		return fmt.Errorf("رصيدك غير كافٍ. الرصيد الحالي: %.2f، المبلغ المطلوب: %.2f", balance, amount)
	}

	_, err = tx.Exec("UPDATE radius_admins SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, adminID)
	if err != nil {
		return err
	}

	// Add transaction record for admin if we have a table for it. 
	// Currently radius_user_transactions is for users. 
	// I'll add it to radius_user_transactions but with username = 'admin:' + admin_username or similar,
	// or better, let's see if we have an admin transactions table.
	// Looking at schema... no admin transaction table. I'll just use radius_user_transactions with a prefix.
	
	var adminUser string
	_ = tx.QueryRow("SELECT username FROM radius_admins WHERE id = ?", adminID).Scan(&adminUser)
	
	_, _ = tx.Exec(`INSERT INTO radius_user_transactions (username, transaction_type, amount, notes, admin_id, created_at)
	                 VALUES (?, 'agent_payment', ?, ?, ?, CURRENT_TIMESTAMP)`, 
	                 "admin:"+adminUser, amount, notes, adminID)

	return tx.Commit()
}

type AdminTransaction struct {
	ID              int64     `json:"id"`
	AdminID         int64     `json:"admin_id"`
	AdminUsername   string    `json:"admin_username"`
	PerformedBy     int64     `json:"performed_by"`
	PerformerName   string    `json:"performer_name"`
	TransactionType string    `json:"transaction_type"`
	Amount          float64   `json:"amount"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
}

func RechargeSubAdmin(targetID, performerID int64, performerRole string, amount float64, notes string) error {
	if amount <= 0 {
		return fmt.Errorf("المبلغ يجب أن يكون أكبر من صفر")
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Get target details and check if performer has authority
	var targetParentID sql.NullInt64
	var targetRole string
	err = tx.QueryRow("SELECT parent_id, role FROM radius_admins WHERE id = ?", targetID).Scan(&targetParentID, &targetRole)
	if err != nil {
		return fmt.Errorf("الوكيل الفرعي غير موجود")
	}

	if performerRole != "superadmin" {
		if !targetParentID.Valid || targetParentID.Int64 != performerID {
			return fmt.Errorf("غير مصرح لك بشحن هذا الوكيل (ليس وكيلاً فرعياً لك)")
		}

		// Check performer's balance
		var performerBalance float64
		err = tx.QueryRow("SELECT balance FROM radius_admins WHERE id = ?", performerID).Scan(&performerBalance)
		if err != nil {
			return err
		}
		if performerBalance < amount {
			return fmt.Errorf("رصيدك غير كافٍ. الرصيد الحالي: %.2f، المبلغ المطلوب: %.2f", performerBalance, amount)
		}

		// Deduct from performer
		_, err = tx.Exec("UPDATE radius_admins SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, performerID)
		if err != nil {
			return err
		}
	}

	// 2. Add to target balance
	_, err = tx.Exec("UPDATE radius_admins SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, targetID)
	if err != nil {
		return err
	}

	// 3. Log transaction
	_, err = tx.Exec(`INSERT INTO radius_admin_transactions (admin_id, performed_by, transaction_type, amount, notes, created_at)
	                  VALUES (?, ?, 'recharge', ?, ?, CURRENT_TIMESTAMP)`, targetID, performerID, amount, notes)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func WithdrawSubAdmin(targetID, performerID int64, performerRole string, amount float64, notes string) error {
	if amount <= 0 {
		return fmt.Errorf("المبلغ يجب أن يكون أكبر من صفر")
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Get target details and check if performer has authority
	var targetParentID sql.NullInt64
	var targetBalance float64
	err = tx.QueryRow("SELECT parent_id, balance FROM radius_admins WHERE id = ?", targetID).Scan(&targetParentID, &targetBalance)
	if err != nil {
		return fmt.Errorf("الوكيل الفرعي غير موجود")
	}

	if performerRole != "superadmin" {
		if !targetParentID.Valid || targetParentID.Int64 != performerID {
			return fmt.Errorf("غير مصرح لك بالسحب من هذا الوكيل (ليس وكيلاً فرعياً لك)")
		}
	}

	// Check target's balance
	if targetBalance < amount {
		return fmt.Errorf("رصيد الوكيل الفرعي غير كافٍ للسحب. الرصيد الحالي: %.2f، المبلغ المطلوب: %.2f", targetBalance, amount)
	}

	// 2. Deduct from target balance
	_, err = tx.Exec("UPDATE radius_admins SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, targetID)
	if err != nil {
		return err
	}

	// 3. If performer is not superadmin, credit their balance
	if performerRole != "superadmin" {
		_, err = tx.Exec("UPDATE radius_admins SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, performerID)
		if err != nil {
			return err
		}
	}

	// 4. Log transaction
	_, err = tx.Exec(`INSERT INTO radius_admin_transactions (admin_id, performed_by, transaction_type, amount, notes, created_at)
	                  VALUES (?, ?, 'withdrawal', ?, ?, CURRENT_TIMESTAMP)`, targetID, performerID, amount, notes)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func ListAdminTransactions(requesterID int64, requesterRole string) ([]AdminTransaction, error) {
	var rows *sql.Rows
	var err error

	query := `SELECT t.id, t.admin_id, a.username, t.performed_by, p.username, t.transaction_type, t.amount, t.notes, t.created_at
	          FROM radius_admin_transactions t
	          JOIN radius_admins a ON t.admin_id = a.id
	          JOIN radius_admins p ON t.performed_by = p.id`

	if requesterRole == "superadmin" {
		rows, err = DB.Query(query + ` ORDER BY t.id DESC LIMIT 200`)
	} else {
		rows, err = DB.Query(query + ` WHERE t.admin_id = ? OR t.performed_by = ? OR a.parent_id = ? ORDER BY t.id DESC LIMIT 200`,
			requesterID, requesterID, requesterID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AdminTransaction{}
	for rows.Next() {
		var tx AdminTransaction
		var createdAt string
		if err := rows.Scan(&tx.ID, &tx.AdminID, &tx.AdminUsername, &tx.PerformedBy, &tx.PerformerName, &tx.TransactionType, &tx.Amount, &tx.Notes, &createdAt); err != nil {
			return nil, err
		}
		tx.CreatedAt = parseDBTime(createdAt)
		out = append(out, tx)
	}
	return out, nil
}
