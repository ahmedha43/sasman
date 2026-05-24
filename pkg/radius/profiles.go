package radius

import (
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v2"
)

func GetProfiles(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	var query string
	var args []interface{}

	if role == "superadmin" {
		query = `SELECT m.groupname, COALESCE(m.admin_id, 0), COALESCE(a.username, 'System') as admin_name
                 FROM radius_profile_meta m
                 LEFT JOIN radius_admins a ON m.admin_id = a.id`
	} else {
		query = `SELECT m.groupname, COALESCE(m.admin_id, 0), COALESCE(a.username, 'Sub-Agent') as admin_name 
                 FROM radius_profile_meta m 
                 LEFT JOIN radius_admins a ON m.admin_id = a.id
                 WHERE m.admin_id = ? 
                    OR m.admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?)
                    OR m.admin_id IS NULL`
		args = append(args, adminID, adminID)
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	profiles := make([]map[string]interface{}, 0)
	for rows.Next() {
		var name, adminName string
		var uAdminID int64
		rows.Scan(&name, &uAdminID, &adminName)

		var limit string
		DB.QueryRow("SELECT value FROM radgroupreply WHERE groupname=? AND attribute='MikroTik-Rate-Limit'", name).Scan(&limit)

		var pool string
		DB.QueryRow("SELECT value FROM radgroupreply WHERE groupname=? AND attribute='Framed-Pool'", name).Scan(&pool)

		var mikrotikGroup string
		DB.QueryRow("SELECT value FROM radgroupreply WHERE groupname=? AND attribute='Mikrotik-Group'", name).Scan(&mikrotikGroup)

		var validityDays int
		var price, agentPrice float64
		var expiredPool, expiredProfile string
		_ = DB.QueryRow("SELECT validity_days, price, agent_price, COALESCE(expired_pool, ''), COALESCE(expired_profile, '') FROM radius_profile_meta WHERE groupname=?", name).Scan(&validityDays, &price, &agentPrice, &expiredPool, &expiredProfile)

		var nasIp string
		DB.QueryRow("SELECT value FROM radgroupcheck WHERE groupname=? AND attribute='NAS-IP-Address'", name).Scan(&nasIp)
		if nasIp == "" {
			nasIp = "ALL"
		}

		var simultaneous string
		DB.QueryRow("SELECT value FROM radgroupcheck WHERE groupname=? AND attribute='Simultaneous-Use'", name).Scan(&simultaneous)
		if simultaneous == "" {
			simultaneous = "1"
		}

		profiles = append(profiles, map[string]interface{}{
			"name":               name,
			"limit":              limit,
			"pool":               pool,
			"mikrotik_group":     mikrotikGroup,
			"validity_days":      validityDays,
			"validity_sec":       validityDays * 86400,
			"nas_ip":             nasIp,
			"simultaneous":       simultaneous,
			"price":              price,
			"agent_price":        agentPrice,
			"expired_pool":       expiredPool,
			"expired_profile":    expiredProfile,
			"admin_id":           uAdminID,
			"admin_name":         adminName,
		})
	}

	return c.JSON(profiles)
}

func CreateProfile(c *fiber.Ctx) error {
	type Request struct {
		Name           string  `json:"name"`
		OriginalName   string  `json:"original_name"`
		Download       string  `json:"download"` // M
		Upload         string  `json:"upload"`   // M
		Pool           string  `json:"pool"`
		MikrotikGroup  string  `json:"mikrotik_group"`
		Validity       string  `json:"validity"` // days
		NasIP          string  `json:"nas_ip"`   // Optional NAS IP binding
		Price          float64 `json:"price"`
		AgentPrice     float64 `json:"agent_price"`
		Simultaneous   string  `json:"simultaneous"`
		ExpiredPool    string  `json:"expired_pool"`
		ExpiredProfile string  `json:"expired_profile"`
		AdminID        int64   `json:"admin_id"` // For superadmin
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Speed check (allows legacy format if already combined)
	speedLimit := req.Upload + "M/" + req.Download + "M"
	if req.Download == "" && req.Upload == "" {
		// Perhaps we are updating a profile that only has speed in metadata or something?
		// But usually we expect download/upload.
	}

	// ---------- 1. Handle renames: rewrite old → new everywhere ----------
	if req.OriginalName != "" && req.OriginalName != req.Name {
		// Update users whose profile is still the old name
		_, _ = DB.Exec("UPDATE radius_users SET groupname=? WHERE groupname=?", req.Name, req.OriginalName)

		// Drop old profile rows (reply / check / meta)
		_, _ = DB.Exec("DELETE FROM radgroupreply WHERE groupname=?", req.OriginalName)
		_, _ = DB.Exec("DELETE FROM radgroupcheck WHERE groupname=?", req.OriginalName)
		_, _ = DB.Exec("DELETE FROM radius_profile_meta WHERE groupname=?", req.OriginalName)
	}

	// ---------- 2. Create / update the (new) profile ----------
	DB.Exec("DELETE FROM radgroupreply WHERE groupname=?", req.Name)
	DB.Exec("DELETE FROM radgroupcheck WHERE groupname=?", req.Name)

	if req.Download != "" || req.Upload != "" {
		_, _ = DB.Exec("INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES (?, 'MikroTik-Rate-Limit', '=', ?)", req.Name, speedLimit)
	}

	// Only save Framed-Pool in radgroupreply if using MikroTik pool
	if req.Pool != "" {
		_, _ = DB.Exec("INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES (?, 'Framed-Pool', '=', ?)", req.Name, req.Pool)
	}

	// Save Mikrotik-Group
	if req.MikrotikGroup != "" {
		_, _ = DB.Exec("INSERT INTO radgroupreply (groupname, attribute, op, value) VALUES (?, 'Mikrotik-Group', '=', ?)", req.Name, req.MikrotikGroup)
	}

	if req.Simultaneous != "0" && req.Simultaneous != "" {
		_, _ = DB.Exec("INSERT INTO radgroupcheck (groupname, attribute, op, value) VALUES (?, 'Simultaneous-Use', ':=', ?)", req.Name, req.Simultaneous)
	}

	if req.NasIP != "" && req.NasIP != "ALL" {
		_, _ = DB.Exec("INSERT INTO radgroupcheck (groupname, attribute, op, value) VALUES (?, 'NAS-IP-Address', '==', ?)", req.Name, req.NasIP)
	}

	var days int
	fmt.Sscanf(req.Validity, "%d", &days)


	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	canManage, _ := c.Locals("can_manage_profiles").(bool)

	if role != "superadmin" && !canManage {
		return c.Status(403).JSON(fiber.Map{"error": "لا تملك صلاحية إدارة الباقات"})
	}

	targetAdminID := adminID
	if role == "superadmin" && req.AdminID > 0 {
		targetAdminID = req.AdminID
	}

	if _, err := DB.Exec(
		`INSERT INTO radius_profile_meta (groupname, validity_days, price, agent_price, expired_pool, expired_profile, admin_id, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(groupname) DO UPDATE SET
		 	validity_days=excluded.validity_days,
		 	price=excluded.price,
		 	agent_price=excluded.agent_price,
		 	expired_pool=excluded.expired_pool,
		 	expired_profile=excluded.expired_profile,
		 	admin_id=excluded.admin_id,
		 	updated_at=CURRENT_TIMESTAMP`,
		req.Name,
		days,
		req.Price,
		req.AgentPrice,
		req.ExpiredPool,
		req.ExpiredProfile,
		targetAdminID,
	); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Rebuild the in-memory profile redirects cache instantly (zero-overhead lookup)
	UpdateProfileRedirectsCache()


	// Trigger sync for all users in this profile
	go SyncUsersInGroup(req.Name)

	return c.JSON(fiber.Map{"message": "تم حفظ الباقة بنجاح"})
}

func DeleteProfile(c *fiber.Ctx) error {
	name, _ := url.PathUnescape(c.Params("name"))
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	canManage, _ := c.Locals("can_manage_profiles").(bool)

	// Ownership check
	if role != "superadmin" {
		if !canManage {
			return c.Status(403).JSON(fiber.Map{"error": "لا تملك صلاحية إدارة الباقات"})
		}
		var ownerID *int64
		err := DB.QueryRow("SELECT admin_id FROM radius_profile_meta WHERE groupname=?", name).Scan(&ownerID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Profile not found"})
		}
		if ownerID == nil || *ownerID != adminID {
			return c.Status(403).JSON(fiber.Map{"error": "You do not own this profile"})
		}
	}

	DB.Exec("DELETE FROM radgroupreply WHERE groupname=?", name)
	DB.Exec("DELETE FROM radgroupcheck WHERE groupname=?", name)
	DB.Exec("DELETE FROM radius_profile_meta WHERE groupname=?", name)
	return c.JSON(fiber.Map{"message": "تم حذف الباقة بنجاح"})
}
