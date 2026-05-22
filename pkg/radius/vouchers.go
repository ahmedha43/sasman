package radius

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Voucher struct {
	ID           int        `json:"id"`
	BatchID      string     `json:"batch_id"`
	Code         string     `json:"code"`
	ProfileName  string     `json:"profile_name"`
	ValidityDays int        `json:"validity_days"`
	Price        float64    `json:"price"`
	CreatedBy    int64      `json:"created_by"`
	UsedBy       string     `json:"used_by"`
	UsedAt       *time.Time `json:"used_at"`
	IsUsed       int        `json:"is_used"`
	CreatedAt    time.Time  `json:"created_at"`
}

func GenerateVouchers(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	ensureVoucherBatchSchema()

	type Request struct {
		ProfileName string  `json:"profile_name"`
		Count       int     `json:"count"`
		Price       float64 `json:"price"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.ProfileName == "" || req.Count <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Profile name and count are required"})
	}

	// Validate profile exists and get validity_days
	var validityDays int
	err := DB.QueryRow("SELECT validity_days FROM radius_profile_meta WHERE groupname = ?", req.ProfileName).Scan(&validityDays)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "الباقة المحددة غير موجودة"})
	}

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer tx.Rollback()

	vouchers := make([]string, 0)
	batchID := fmt.Sprintf("VCH-%d-%s", time.Now().Unix(), generateRandomCode(6))
	for i := 0; i < req.Count; i++ {
		code := generateRandomCode(10)
		_, err = tx.Exec(`INSERT INTO radius_vouchers (batch_id, code, profile_name, validity_days, price, created_by) 
						 VALUES (?, ?, ?, ?, ?, ?)`, batchID, code, req.ProfileName, validityDays, req.Price, adminID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to generate voucher: " + err.Error()})
		}
		vouchers = append(vouchers, code)
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": fmt.Sprintf("تم إنشاء %d كرت بنجاح", req.Count), "vouchers": vouchers})
}

func GetVouchers(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	ensureVoucherBatchSchema()

	var query string
	var args []interface{}

	if role == "superadmin" {
		query = "SELECT id, COALESCE(batch_id, ''), code, profile_name, validity_days, price, created_by, COALESCE(used_by, ''), used_at, is_used, created_at FROM radius_vouchers ORDER BY created_at DESC, id DESC"
	} else {
		query = "SELECT id, COALESCE(batch_id, ''), code, profile_name, validity_days, price, created_by, COALESCE(used_by, ''), used_at, is_used, created_at FROM radius_vouchers WHERE created_by = ? ORDER BY created_at DESC, id DESC"
		args = append(args, adminID)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	vouchers := make([]map[string]interface{}, 0)
	for rows.Next() {
		var v Voucher
		var usedBy string
		var usedAt sql.NullTime
		if err := rows.Scan(&v.ID, &v.BatchID, &v.Code, &v.ProfileName, &v.ValidityDays, &v.Price, &v.CreatedBy, &usedBy, &usedAt, &v.IsUsed, &v.CreatedAt); err != nil {
			continue
		}

		res := map[string]interface{}{
			"id":            v.ID,
			"batch_id":      v.BatchID,
			"code":          v.Code,
			"profile_name":  v.ProfileName,
			"validity_days": v.ValidityDays,
			"price":         v.Price,
			"created_by":    v.CreatedBy,
			"used_by":       usedBy,
			"is_used":       v.IsUsed,
			"created_at":    v.CreatedAt,
		}
		if usedAt.Valid {
			res["used_at"] = usedAt.Time
		} else {
			res["used_at"] = nil
		}
		vouchers = append(vouchers, res)
	}

	return c.JSON(vouchers)
}

func ensureVoucherBatchSchema() {
	rows, err := DB.Query("PRAGMA table_info(radius_vouchers)")
	if err != nil {
		return
	}
	defer rows.Close()

	hasBatchID := false
	for rows.Next() {
		var cid int
		var name, dtype string
		var notnull, pk int
		var dflt interface{}
		rows.Scan(&cid, &name, &dtype, &notnull, &dflt, &pk)
		if name == "batch_id" {
			hasBatchID = true
		}
	}

	if !hasBatchID {
		_, _ = DB.Exec("ALTER TABLE radius_vouchers ADD COLUMN batch_id TEXT NOT NULL DEFAULT ''")
		_, _ = DB.Exec("CREATE INDEX IF NOT EXISTS idx_radius_vouchers_batch ON radius_vouchers(batch_id)")
	}
}

func RedeemVoucher(c *fiber.Ctx) error {
	type Request struct {
		Username string `json:"username"`
		Code     string `json:"code"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Code = strings.TrimSpace(req.Code)

	if req.Username == "" || req.Code == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم المستخدم وكود الكرت مطلوبان"})
	}

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer tx.Rollback()

	// 1. Check voucher
	var v Voucher
	err = tx.QueryRow("SELECT id, profile_name, validity_days, is_used FROM radius_vouchers WHERE code = ?", req.Code).Scan(&v.ID, &v.ProfileName, &v.ValidityDays, &v.IsUsed)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "كود الكرت غير صحيح"})
	}
	if v.IsUsed == 1 {
		return c.Status(400).JSON(fiber.Map{"error": "هذا الكرت مستخدم مسبقاً"})
	}

	// 2. Check user exists
	var exists int
	_ = tx.QueryRow("SELECT 1 FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", req.Username).Scan(&exists)
	if exists == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "اسم المستخدم غير موجود"})
	}

	// 3. Perform Renewal Logic (Simplified version of RenewUser)
	currentExpiration, _ := loadUserExpiration(req.Username)
	newExpiration := calculateRenewedExpiration(currentExpiration, v.ValidityDays)

	if err := replaceUserProfileTx(tx, req.Username, v.ProfileName); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update profile: " + err.Error()})
	}
	if err := saveUserExpirationTx(tx, req.Username, newExpiration); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update expiration: " + err.Error()})
	}

	// Get owner admin_id to keep metadata consistent
	var adminID int64
	_ = tx.QueryRow("SELECT admin_id FROM radius_user_meta WHERE username = ?", req.Username).Scan(&adminID)

	if err := saveUserMetaTx(tx, req.Username, newExpiration, adminID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update metadata: " + err.Error()})
	}

	// 4. Mark voucher as used
	_, err = tx.Exec("UPDATE radius_vouchers SET is_used = 1, used_by = ?, used_at = CURRENT_TIMESTAMP WHERE id = ?", req.Username, v.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update voucher status: " + err.Error()})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Async Notification
	go SendWhatsappNotification(req.Username, "renew_paid", map[string]string{
		"username":      req.Username,
		"profile":       v.ProfileName,
		"price":         "0", // Voucher is already paid
		"validity_days": fmt.Sprintf("%d", v.ValidityDays),
	})

	return c.JSON(fiber.Map{"message": "تم تفعيل الكرت بنجاح! تم تجديد اشتراكك."})
}

func generateRandomCode(n int) string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // No I, O, 0, 1 for clarity
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		ret[i] = letters[num.Int64()]
	}
	return string(ret)
}

func DeleteVoucher(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	id := c.Params("id")

	var err error
	if role == "superadmin" {
		_, err = DB.Exec("DELETE FROM radius_vouchers WHERE id = ?", id)
	} else {
		_, err = DB.Exec("DELETE FROM radius_vouchers WHERE id = ? AND created_by = ?", id, adminID)
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل في حذف الكرت: " + err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم حذف الكرت بنجاح"})
}

func DeleteVoucherBatch(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	batchID := c.Params("batch_id")

	var err error
	var res sql.Result
	if role == "superadmin" {
		res, err = DB.Exec("DELETE FROM radius_vouchers WHERE batch_id = ?", batchID)
	} else {
		res, err = DB.Exec("DELETE FROM radius_vouchers WHERE batch_id = ? AND created_by = ?", batchID, adminID)
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل في حذف المجموعة: " + err.Error()})
	}
	
	rowsAffected, _ := res.RowsAffected()
	return c.JSON(fiber.Map{"message": fmt.Sprintf("تم حذف %d كرت من المجموعة بنجاح", rowsAffected)})
}

func ClearAllVouchers(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "غير مصرح لك بحذف جميع الكروت"})
	}

	_, err := DB.Exec("DELETE FROM radius_vouchers")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل في مسح جميع الكروت: " + err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم حذف جميع الكروت بنجاح (ضبط المصنع للكروت)"})
}
