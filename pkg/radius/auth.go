package radius

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoginHandler(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم المستخدم وكلمة المرور مطلوبة"})
	}

	admin, hash, err := GetAdminByUsername(req.Username)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if admin == nil {
		return c.Status(401).JSON(fiber.Map{"error": "بيانات الدخول غير صحيحة"})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "بيانات الدخول غير صحيحة"})
	}

	token, err := newSessionToken()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	expires := time.Now().Add(adminSessionTTL)
	if _, err := DB.Exec(`INSERT INTO radius_admin_sessions (token, admin_id, expires_at) VALUES (?, ?, ?)`,
		token, admin.ID, expires.UTC().Format("2006-01-02 15:04:05")); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	c.Cookie(&fiber.Cookie{
		Name:     adminSessionCookie,
		Value:    token,
		Expires:  expires,
		HTTPOnly: true,
		SameSite: "Lax",
		Path:     "/",
	})
	return c.JSON(fiber.Map{"message": "تم تسجيل الدخول", "admin": admin})
}

func LogoutHandler(c *fiber.Ctx) error {
	token := c.Cookies(adminSessionCookie)
	if token != "" {
		_, _ = DB.Exec(`DELETE FROM radius_admin_sessions WHERE token=?`, token)
	}
	c.Cookie(&fiber.Cookie{
		Name:     adminSessionCookie,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
		Path:     "/",
	})
	return c.JSON(fiber.Map{"message": "تم تسجيل الخروج"})
}

func MeHandler(c *fiber.Ctx) error {
	id, ok := c.Locals("admin_id").(int64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "غير مسجل"})
	}
	admin, err := GetAdminByID(id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(admin)
}

func UpdateProfileHandler(c *fiber.Ctx) error {
	id, ok := c.Locals("admin_id").(int64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "غير مسجل"})
	}
	type req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}
	admin, err := UpdateAdminProfile(id, strings.TrimSpace(body.Name), strings.TrimSpace(body.Email), c.Locals("can_manage_profiles").(bool), c.Locals("can_manage_nas").(bool))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم تحديث البيانات", "admin": admin})
}

func ChangePasswordHandler(c *fiber.Ctx) error {
	id, ok := c.Locals("admin_id").(int64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "غير مسجل"})
	}
	type req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}
	if err := ChangeAdminPassword(id, body.Current, body.New); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم تغيير كلمة المرور"})
}

func RegisterAdminHandler(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(int64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "غير مسجل"})
	}
	requesterRole, _ := c.Locals("role").(string)

	type req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		CanManageProfiles bool `json:"can_manage_profiles"`
		CanManageNas      bool `json:"can_manage_nas"`
	}
	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	// Security: Only superadmin can assign roles other than 'agent'
	// and only superadmin can create a superadmin.
	if requesterRole != "superadmin" {
		body.Role = "agent" // Force 'agent' role for non-superadmins
	}
	
	admin, err := CreateAdminAccount(body.Username, body.Password, body.Name, body.Email, body.Role, &adminID, body.CanManageProfiles, body.CanManageNas)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم إنشاء الحساب", "admin": admin})
}

func ListAdminsHandler(c *fiber.Ctx) error {
	adminID, ok := c.Locals("admin_id").(int64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "غير مسجل"})
	}
	role, _ := c.Locals("role").(string)

	list, err := ListAdmins(adminID, role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(list)
}

func DeleteAdminHandler(c *fiber.Ctx) error {
	currentID, ok := c.Locals("admin_id").(int64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "غير مسجل"})
	}
	role, _ := c.Locals("role").(string)

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "معرّف غير صالح"})
	}
	if int64(id) == currentID {
		return c.Status(400).JSON(fiber.Map{"error": "لا يمكن حذف حسابك الحالي"})
	}
	if err := DeleteAdminByID(int64(id), currentID, role); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم حذف الحساب"})
}

func RechargeAdminHandler(c *fiber.Ctx) error {
	performerID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	type req struct {
		AdminID int64   `json:"admin_id"`
		Amount  float64 `json:"amount"`
		Notes   string  `json:"notes"`
	}
	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}
	if body.AdminID <= 0 || body.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "المعرّف أو المبلغ غير صالح"})
	}
	if err := RechargeSubAdmin(body.AdminID, performerID, role, body.Amount, body.Notes); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم شحن رصيد الوكيل بنجاح"})
}

func WithdrawAdminHandler(c *fiber.Ctx) error {
	performerID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	type req struct {
		AdminID int64   `json:"admin_id"`
		Amount  float64 `json:"amount"`
		Notes   string  `json:"notes"`
	}
	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}
	if body.AdminID <= 0 || body.Amount <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "المعرّف أو المبلغ غير صالح"})
	}
	if err := WithdrawSubAdmin(body.AdminID, performerID, role, body.Amount, body.Notes); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم سحب الرصيد من الوكيل بنجاح"})
}

func ListAdminTransactionsHandler(c *fiber.Ctx) error {
	requesterID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	list, err := ListAdminTransactions(requesterID, role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(list)
}
