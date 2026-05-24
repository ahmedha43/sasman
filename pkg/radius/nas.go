package radius

import (
	"mikrotik-manager/pkg/core"

	"github.com/gofiber/fiber/v2"
)

func GetNAS(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	var query string
	var args []interface{}

	if role == "superadmin" {
		query = `SELECT n.id, n.nasname, n.shortname, n.secret, COALESCE(n.profile_nas_ip, ''), COALESCE(n.admin_id, 0), COALESCE(a.username, 'System') as admin_name
                 FROM nas n
                 LEFT JOIN radius_admins a ON n.admin_id = a.id`
	} else {
		query = `SELECT n.id, n.nasname, n.shortname, n.secret, COALESCE(n.profile_nas_ip, ''), COALESCE(n.admin_id, 0), COALESCE(a.username, 'Global') as admin_name 
                 FROM nas n 
                 LEFT JOIN radius_admins a ON n.admin_id = a.id
                 WHERE n.admin_id = ? 
                    OR n.admin_id = 0 
                    OR n.admin_id IS NULL
                    OR n.admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?)`
		args = append(args, adminID, adminID)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	nasList := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var ip, name, secret, profileNASIP, adminName string
		var uAdminID int64
		rows.Scan(&id, &ip, &name, &secret, &profileNASIP, &uAdminID, &adminName)
		nasList = append(nasList, map[string]interface{}{
			"id":             id,
			"ip":             ip,
			"name":           name,
			"secret":         secret,
			"profile_nas_ip": profileNASIP,
			"admin_id":       uAdminID,
			"admin_name":     adminName,
		})
	}
	return c.JSON(nasList)
}

func UpdateNAS(c *fiber.Ctx) error {
	id := c.Params("id")
	role, _ := c.Locals("role").(string)

	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "صلاحية التعديل محصورة بمدير النظام فقط"})
	}

	type Request struct {
		IP           string `json:"ip"`
		Name         string `json:"name"`
		Secret       string `json:"secret"`
		ProfileNASIP string `json:"profile_nas_ip"`
		AdminID      int64  `json:"admin_id"`
		IsGlobal     bool   `json:"is_global"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	targetAdminID := req.AdminID
	if req.IsGlobal {
		targetAdminID = 0
	}

	_, err := DB.Exec(
		"UPDATE nas SET nasname=?, shortname=?, secret=?, profile_nas_ip=?, admin_id=? WHERE id=?",
		req.IP, req.Name, req.Secret, req.ProfileNASIP, targetAdminID, id,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	reloadFreeRADIUS()
	return c.JSON(fiber.Map{"message": "تم تعديل الراوتر بنجاح"})
}

func CreateNAS(c *fiber.Ctx) error {
	type Request struct {
		IP           string `json:"ip"`
		Name         string `json:"name"`
		Secret       string `json:"secret"`
		ProfileNASIP string `json:"profile_nas_ip"`
		AdminID      int64  `json:"admin_id"` // For superadmin
		IsGlobal     bool   `json:"is_global"` // Make visible to everyone
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// DB.Exec("DELETE FROM nas WHERE nasname=?", req.IP) // Removed to allow same IP for different names/admins

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	canManage, _ := c.Locals("can_manage_nas").(bool)

	if role != "superadmin" && !canManage {
		return c.Status(403).JSON(fiber.Map{"error": "لا تملك صلاحية إدارة أجهزة الراوتر"})
	}

	targetAdminID := adminID
	if role == "superadmin" {
		if req.IsGlobal {
			targetAdminID = 0 // 0 means visible to all
		} else if req.AdminID > 0 {
			targetAdminID = req.AdminID
		}
	}

	_, err := DB.Exec(
		"INSERT INTO nas (nasname, shortname, secret, profile_nas_ip, admin_id) VALUES (?, ?, ?, ?, ?)",
		req.IP,
		req.Name,
		req.Secret,
		req.ProfileNASIP,
		targetAdminID,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	reloadFreeRADIUS()
	return c.JSON(fiber.Map{"message": "تم إضافة راوتر NAS بنجاح"})
}

func DeleteNAS(c *fiber.Ctx) error {
	ip := c.Params("ip")
	role, _ := c.Locals("role").(string)

	// Only superadmin can delete NAS
	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "صلاحية الحذف محصورة بمدير النظام فقط"})
	}

	DB.Exec("DELETE FROM nas WHERE nasname=?", ip)
	reloadFreeRADIUS()
	return c.JSON(fiber.Map{"message": "تم حذف الراوتر بنجاح"})
}

func QuickSetupNAS(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)

	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "صلاحية التعديل محصورة بمدير النظام فقط"})
	}

	ip := "172.17.0.1"
	name := "SASMAN"
	secret := "123456"
	profileNASIP := "172.17.0.1"
	targetAdminID := int64(0) // is_global = true

	var count int
	DB.QueryRow("SELECT COUNT(*) FROM nas WHERE nasname=?", ip).Scan(&count)
	if count == 0 {
		_, err := DB.Exec(
			"INSERT INTO nas (nasname, shortname, secret, profile_nas_ip, admin_id) VALUES (?, ?, ?, ?, ?)",
			ip, name, secret, profileNASIP, targetAdminID,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل الإضافة في قاعدة البيانات: " + err.Error()})
		}
	} else {
		DB.Exec("UPDATE nas SET shortname=?, secret=?, profile_nas_ip=?, admin_id=? WHERE nasname=?", name, secret, profileNASIP, targetAdminID, ip)
	}

	reloadFreeRADIUS()

	// Add to MikroTik
	client, err := core.GetSharedClient()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "تم الإضافة بالنظام، لكن تعذر الاتصال بالمايكروتك: " + err.Error()})
	}

	// Check if already exists in MikroTik
	res, _ := core.SafeRun(client, "/radius/print", "?address="+ip)
	if res != nil && len(res.Re) == 0 {
		_, err = core.SafeRun(client, "/radius/add", "=address="+ip, "=secret="+secret, "=service=ppp,hotspot,wireless", "=timeout=3000ms")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "تم الإضافة بالنظام، لكن فشل في مايكروتك: " + err.Error()})
		}
	} else if res != nil && len(res.Re) > 0 {
		// Update existing
		id := res.Re[0].Map[".id"]
		core.SafeRun(client, "/radius/set", "=.id="+id, "=secret="+secret, "=service=ppp,hotspot,wireless")
	}

	// Enable incoming radius
	core.SafeRun(client, "/radius/incoming/set", "=accept=yes", "=port=3799")

	return c.JSON(fiber.Map{"message": "تم إعداد الراديوس السريع في النظام والمايكروتك بنجاح"})
}

