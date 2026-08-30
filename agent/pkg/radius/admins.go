package radius

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	ID                    int64     `json:"id"`
	Username              string    `json:"username"`
	Name                  string    `json:"name"`
	Email                 string    `json:"email"`
	Role                  string    `json:"role"`
	ParentID              *int64    `json:"parent_id"`
	Balance               float64   `json:"balance"`
	CanManageProfiles     bool      `json:"can_manage_profiles"`
	CanManageNas          bool      `json:"can_manage_nas"`
	CanCreateUsers        bool      `json:"can_create_users"`
	CanEditUsers          bool      `json:"can_edit_users"`
	CanDeleteUsers        bool      `json:"can_delete_users"`
	CanToggleUsers        bool      `json:"can_toggle_users"`
	CanDisconnectUsers    bool      `json:"can_disconnect_users"`
	CanRenewUsers         bool      `json:"can_renew_users"`
	CanGenerateVouchers   bool      `json:"can_generate_vouchers"`
	CanDeleteVouchers     bool      `json:"can_delete_vouchers"`
	CanPrintVouchers      bool      `json:"can_print_vouchers"`
	CanManageDevices      bool      `json:"can_manage_devices"`
	CanManageTransactions bool      `json:"can_manage_transactions"`
	CanManageSubagents    bool      `json:"can_manage_subagents"`
	CanViewLogs           bool      `json:"can_view_logs"`
	CanClearLogs          bool      `json:"can_clear_logs"`
	CanManageWhatsapp     bool      `json:"can_manage_whatsapp"`
	CanManageStreams      bool      `json:"can_manage_streams"`
	Permissions           string    `json:"permissions"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

var OnAdminPasswordChanged func()

// HasPermission checks if the authenticated requester has a specific privilege or is superadmin
func HasPermission(c *fiber.Ctx, perm string) bool {
	role, _ := c.Locals("role").(string)
	if role == "superadmin" {
		return true
	}
	perms, ok := c.Locals("permissions").(map[string]bool)
	if !ok || perms == nil {
		return false
	}
	return perms[perm]
}

// GetSuperadminCredentials returns the primary admin credentials for cloud sync
func GetSuperadminCredentials() (username, password string, isDefault bool) {
	if DB == nil {
		return "admin", "admin", true
	}
	var u, hash, secret string
	err := DB.QueryRow("SELECT username, password_hash, COALESCE(plain_secret, '') FROM radius_admins WHERE role='superadmin' OR role='agent' ORDER BY id ASC LIMIT 1").Scan(&u, &hash, &secret)
	if err != nil || u == "" {
		return "admin", "admin", true
	}
	if secret != "" {
		return u, secret, secret == "admin"
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin")) == nil {
		return u, "admin", true
	}
	return u, "", false
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
	_, err = DB.Exec(`INSERT INTO radius_admins (username, password_hash, name, email, plain_secret, role) VALUES (?, ?, ?, ?, ?, 'superadmin')`,
		"admin", string(hash), "مدير النظام", "", "admin")
	if err != nil {
		log.Printf("[admins] seed error: %v", err)
		return
	}
	log.Println("[admins] default superadmin created: admin/admin (please change password)")
}

func RequireAdmin(c *fiber.Ctx) error {
	token := c.Cookies(adminSessionCookie)
	if token == "" {
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token == "" {
		token = c.Query("token")
	}
	if token == "" {
		return c.Status(401).JSON(fiber.Map{"error": "غير مصرح", "auth_required": true})
	}
	var adminID int64
	var role string
	var parentID sql.NullInt64
	var expiresAt string
	var canProfiles, canNas, canCreateU, canEditU, canDeleteU, canToggleU, canDiscU, canRenewU, canGenV, canDelV, canPrintV, canDev, canTrans, canSubA, canVLogs, canCLogs, canWA, canStreams int
	var customPerms sql.NullString

	if err := DB.QueryRow(`SELECT a.id, a.role, a.parent_id, s.expires_at,
	                              COALESCE(a.can_manage_profiles, 0), COALESCE(a.can_manage_nas, 0),
	                              COALESCE(a.can_create_users, 1), COALESCE(a.can_edit_users, 1), COALESCE(a.can_delete_users, 0),
	                              COALESCE(a.can_toggle_users, 1), COALESCE(a.can_disconnect_users, 1), COALESCE(a.can_renew_users, 1),
	                              COALESCE(a.can_generate_vouchers, 1), COALESCE(a.can_delete_vouchers, 0), COALESCE(a.can_print_vouchers, 1),
	                              COALESCE(a.can_manage_devices, 0), COALESCE(a.can_manage_transactions, 1), COALESCE(a.can_manage_subagents, 0),
	                              COALESCE(a.can_view_logs, 1), COALESCE(a.can_clear_logs, 0), COALESCE(a.can_manage_whatsapp, 0),
	                              COALESCE(a.can_manage_streams, 0), a.permissions
	                       FROM radius_admins a
	                       JOIN radius_admin_sessions s ON a.id = s.admin_id
	                       WHERE s.token=?`, token).
		Scan(&adminID, &role, &parentID, &expiresAt,
			&canProfiles, &canNas, &canCreateU, &canEditU, &canDeleteU, &canToggleU, &canDiscU, &canRenewU,
			&canGenV, &canDelV, &canPrintV, &canDev, &canTrans, &canSubA, &canVLogs, &canCLogs, &canWA, &canStreams, &customPerms); err != nil {
		log.Printf("[auth] Session not found in DB: %v", err)
		return c.Status(401).JSON(fiber.Map{"error": "الجلسة غير صالحة", "auth_required": true})
	}

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

	permsMap := map[string]bool{
		"can_manage_profiles":     canProfiles == 1,
		"can_manage_nas":          canNas == 1,
		"can_create_users":        canCreateU == 1,
		"can_edit_users":          canEditU == 1,
		"can_delete_users":        canDeleteU == 1,
		"can_toggle_users":        canToggleU == 1,
		"can_disconnect_users":    canDiscU == 1,
		"can_renew_users":         canRenewU == 1,
		"can_generate_vouchers":   canGenV == 1,
		"can_delete_vouchers":     canDelV == 1,
		"can_print_vouchers":      canPrintV == 1,
		"can_manage_devices":      canDev == 1,
		"can_manage_transactions": canTrans == 1,
		"can_manage_subagents":    canSubA == 1,
		"can_view_logs":           canVLogs == 1,
		"can_clear_logs":          canCLogs == 1,
		"can_manage_whatsapp":     canWA == 1,
		"can_manage_streams":      canStreams == 1,
	}

	c.Locals("admin_id", adminID)
	c.Locals("role", role)
	c.Locals("can_manage_profiles", canProfiles == 1)
	c.Locals("can_manage_nas", canNas == 1)
	c.Locals("permissions", permsMap)

	if parentID.Valid {
		c.Locals("parent_id", parentID.Int64)
	} else {
		c.Locals("parent_id", nil)
	}
	return c.Next()
}

func scanAdminRow(scanner interface {
	Scan(dest ...interface{}) error
}) (*Admin, string, error) {
	var a Admin
	var hash, createdAt, updatedAt string
	var customPerms sql.NullString
	var canProfiles, canNas, canCreateU, canEditU, canDeleteU, canToggleU, canDiscU, canRenewU, canGenV, canDelV, canPrintV, canDev, canTrans, canSubA, canVLogs, canCLogs, canWA, canStreams int

	err := scanner.Scan(
		&a.ID, &a.Username, &hash, &a.Name, &a.Email, &a.Role, &a.ParentID, &a.Balance,
		&canProfiles, &canNas, &canCreateU, &canEditU, &canDeleteU, &canToggleU, &canDiscU, &canRenewU,
		&canGenV, &canDelV, &canPrintV, &canDev, &canTrans, &canSubA, &canVLogs, &canCLogs, &canWA, &canStreams,
		&customPerms, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, "", err
	}

	a.CanManageProfiles = canProfiles == 1
	a.CanManageNas = canNas == 1
	a.CanCreateUsers = canCreateU == 1
	a.CanEditUsers = canEditU == 1
	a.CanDeleteUsers = canDeleteU == 1
	a.CanToggleUsers = canToggleU == 1
	a.CanDisconnectUsers = canDiscU == 1
	a.CanRenewUsers = canRenewU == 1
	a.CanGenerateVouchers = canGenV == 1
	a.CanDeleteVouchers = canDelV == 1
	a.CanPrintVouchers = canPrintV == 1
	a.CanManageDevices = canDev == 1
	a.CanManageTransactions = canTrans == 1
	a.CanManageSubagents = canSubA == 1
	a.CanViewLogs = canVLogs == 1
	a.CanClearLogs = canCLogs == 1
	a.CanManageWhatsapp = canWA == 1
	a.CanManageStreams = canStreams == 1
	a.Permissions = customPerms.String
	a.CreatedAt = parseDBTime(createdAt)
	a.UpdatedAt = parseDBTime(updatedAt)
	return &a, hash, nil
}

const adminSelectFields = `id, username, password_hash, name, email, role, parent_id, balance,
COALESCE(can_manage_profiles, 0), COALESCE(can_manage_nas, 0),
COALESCE(can_create_users, 1), COALESCE(can_edit_users, 1), COALESCE(can_delete_users, 0),
COALESCE(can_toggle_users, 1), COALESCE(can_disconnect_users, 1), COALESCE(can_renew_users, 1),
COALESCE(can_generate_vouchers, 1), COALESCE(can_delete_vouchers, 0), COALESCE(can_print_vouchers, 1),
COALESCE(can_manage_devices, 0), COALESCE(can_manage_transactions, 1), COALESCE(can_manage_subagents, 0),
COALESCE(can_view_logs, 1), COALESCE(can_clear_logs, 0), COALESCE(can_manage_whatsapp, 0),
COALESCE(can_manage_streams, 0), COALESCE(permissions, ''), created_at, updated_at`

func GetAdminByUsername(username string) (*Admin, string, error) {
	row := DB.QueryRow(fmt.Sprintf(`SELECT %s FROM radius_admins WHERE username = ? LIMIT 1`, adminSelectFields), username)
	a, hash, err := scanAdminRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}
	return a, hash, nil
}

func GetAdminByID(id int64) (*Admin, error) {
	row := DB.QueryRow(fmt.Sprintf(`SELECT %s FROM radius_admins WHERE id = ? LIMIT 1`, adminSelectFields), id)
	a, _, err := scanAdminRow(row)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func CreateAdminAccount(username, password, name, email, role string, parentID *int64, perms map[string]bool) (*Admin, error) {
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

	b2i := func(k string, def bool) int {
		if v, ok := perms[k]; ok {
			if v { return 1 }
			return 0
		}
		if def { return 1 }
		return 0
	}

	jsonBytes, _ := json.Marshal(perms)

	res, err := DB.Exec(`INSERT INTO radius_admins (
		username, password_hash, name, email, role, parent_id,
		can_manage_profiles, can_manage_nas,
		can_create_users, can_edit_users, can_delete_users, can_toggle_users, can_disconnect_users, can_renew_users,
		can_generate_vouchers, can_delete_vouchers, can_print_vouchers,
		can_manage_devices, can_manage_transactions, can_manage_subagents,
		can_view_logs, can_clear_logs, can_manage_whatsapp, can_manage_streams, permissions, plain_secret
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		username, string(hash), name, email, role, parentID,
		b2i("can_manage_profiles", false), b2i("can_manage_nas", false),
		b2i("can_create_users", true), b2i("can_edit_users", true), b2i("can_delete_users", false),
		b2i("can_toggle_users", true), b2i("can_disconnect_users", true), b2i("can_renew_users", true),
		b2i("can_generate_vouchers", true), b2i("can_delete_vouchers", false), b2i("can_print_vouchers", true),
		b2i("can_manage_devices", false), b2i("can_manage_transactions", true), b2i("can_manage_subagents", false),
		b2i("can_view_logs", true), b2i("can_clear_logs", false), b2i("can_manage_whatsapp", false),
		b2i("can_manage_streams", false), string(jsonBytes), password)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("اسم المستخدم مستخدم مسبقًا")
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetAdminByID(id)
}

func UpdateAdminPermissions(id int64, perms map[string]bool) (*Admin, error) {
	b2i := func(k string) int {
		if v, ok := perms[k]; ok && v {
			return 1
		}
		return 0
	}

	jsonBytes, _ := json.Marshal(perms)

	_, err := DB.Exec(`UPDATE radius_admins SET
		can_manage_profiles=?, can_manage_nas=?,
		can_create_users=?, can_edit_users=?, can_delete_users=?, can_toggle_users=?, can_disconnect_users=?, can_renew_users=?,
		can_generate_vouchers=?, can_delete_vouchers=?, can_print_vouchers=?,
		can_manage_devices=?, can_manage_transactions=?, can_manage_subagents=?,
		can_view_logs=?, can_clear_logs=?, can_manage_whatsapp=?, can_manage_streams=?,
		permissions=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		b2i("can_manage_profiles"), b2i("can_manage_nas"),
		b2i("can_create_users"), b2i("can_edit_users"), b2i("can_delete_users"), b2i("can_toggle_users"), b2i("can_disconnect_users"), b2i("can_renew_users"),
		b2i("can_generate_vouchers"), b2i("can_delete_vouchers"), b2i("can_print_vouchers"),
		b2i("can_manage_devices"), b2i("can_manage_transactions"), b2i("can_manage_subagents"),
		b2i("can_view_logs"), b2i("can_clear_logs"), b2i("can_manage_whatsapp"), b2i("can_manage_streams"),
		string(jsonBytes), id)

	if err != nil {
		return nil, err
	}
	return GetAdminByID(id)
}

func UpdateAdminProfile(id int64, name, email string) (*Admin, error) {
	_, err := DB.Exec(`UPDATE radius_admins SET name=?, email=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		name, email, id)
	if err != nil {
		return nil, err
	}
	return GetAdminByID(id)
}

func ListAdmins(requesterID int64, requesterRole string) ([]Admin, error) {
	var rows *sql.Rows
	var err error

	if requesterRole == "superadmin" {
		rows, err = DB.Query(fmt.Sprintf(`SELECT %s FROM radius_admins ORDER BY id ASC`, adminSelectFields))
	} else {
		rows, err = DB.Query(fmt.Sprintf(`SELECT %s FROM radius_admins WHERE parent_id = ? OR id = ? ORDER BY id ASC`, adminSelectFields), requesterID, requesterID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Admin{}
	for rows.Next() {
		a, _, err := scanAdminRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
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
	_, err = DB.Exec(`UPDATE radius_admins SET password_hash=?, plain_secret=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		string(newHash), newPassword, id)
	if err == nil && OnAdminPasswordChanged != nil {
		go OnAdminPasswordChanged()
	}
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

		var performerBalance float64
		err = tx.QueryRow("SELECT balance FROM radius_admins WHERE id = ?", performerID).Scan(&performerBalance)
		if err != nil {
			return err
		}
		if performerBalance < amount {
			return fmt.Errorf("رصيدك غير كافٍ. الرصيد الحالي: %.2f، المبلغ المطلوب: %.2f", performerBalance, amount)
		}

		_, err = tx.Exec("UPDATE radius_admins SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, performerID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec("UPDATE radius_admins SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, targetID)
	if err != nil {
		return err
	}

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

	if targetBalance < amount {
		return fmt.Errorf("رصيد الوكيل الفرعي غير كافٍ للسحب. الرصيد الحالي: %.2f، المبلغ المطلوب: %.2f", targetBalance, amount)
	}

	_, err = tx.Exec("UPDATE radius_admins SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, targetID)
	if err != nil {
		return err
	}

	if performerRole != "superadmin" {
		_, err = tx.Exec("UPDATE radius_admins SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", amount, performerID)
		if err != nil {
			return err
		}
	}

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
