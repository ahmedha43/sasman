package radius

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Telegram API Types ───────────────────────────────────────────────────────

type tgUpdateResponse struct {
	Ok     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

type tgUpdate struct {
	UpdateID      int64           `json:"update_id"`
	Message       *tgMessage      `json:"message"`
	CallbackQuery *tgCallbackQuery `json:"callback_query"`
}

type tgMessage struct {
	MessageID int64  `json:"message_id"`
	Chat      tgChat `json:"chat"`
	From      tgUser `json:"from"`
	Text      string `json:"text"`
}

type tgCallbackQuery struct {
	ID      string     `json:"id"`
	From    tgUser     `json:"from"`
	Message *tgMessage `json:"message"`
	Data    string     `json:"data"`
}

type tgChat struct{ ID int64 `json:"id"` }
type tgUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// ─── User Session State ───────────────────────────────────────────────────────

type botState int

const (
	stateIdle          botState = iota
	stateAwaitSearch            // waiting for search text
	stateAwaitUsername          // waiting for username to view
	stateAwaitRenewUser         // waiting for username to renew
	stateAwaitRenewProfile      // waiting for profile selection (username stored)
)

type userSession struct {
	State    botState
	Username string // temp storage for renew flow
}

var (
	sessionsMu   sync.Mutex
	userSessions = map[int64]*userSession{}
)

func getSession(chatID int64) *userSession {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	if s, ok := userSessions[chatID]; ok {
		return s
	}
	s := &userSession{State: stateIdle}
	userSessions[chatID] = s
	return s
}

func setSessionState(chatID int64, state botState, username string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	userSessions[chatID] = &userSession{State: state, Username: username}
}

// ─── Bot Startup ─────────────────────────────────────────────────────────────

func StartTelegramBot() {
	go func() {
		time.Sleep(15 * time.Second)
		log.Println("[telegram-bot] Starting Interactive Bot Worker...")

		var offset int64
		registered := false
		for {
			cfg := loadTelegramBackupConfig()
			if !cfg.BotEnabled || cfg.BotToken == "" || cfg.ChatID == "" {
				time.Sleep(30 * time.Second)
				continue
			}
			if !registered {
				registerBotCommands(cfg.BotToken)
				registered = true
			}

			updates, err := fetchUpdates(cfg.BotToken, offset)
			if err != nil {
				log.Printf("[telegram-bot] fetch error: %v", err)
				time.Sleep(15 * time.Second)
				continue
			}
			for _, u := range updates {
				if u.UpdateID >= offset {
					offset = u.UpdateID + 1
				}
				allowedID, _ := strconv.ParseInt(cfg.ChatID, 10, 64)
				if u.Message != nil && u.Message.Chat.ID == allowedID {
					handleMessage(cfg, u.Message)
				}
				if u.CallbackQuery != nil && u.CallbackQuery.Message != nil &&
					u.CallbackQuery.Message.Chat.ID == allowedID {
					handleCallback(cfg, u.CallbackQuery)
				}
			}
			if len(updates) == 0 {
				time.Sleep(2 * time.Second)
			}
		}
	}()
}

// ─── Fetch Updates ────────────────────────────────────────────────────────────

func fetchUpdates(token string, offset int64) ([]tgUpdate, error) {
	body := map[string]interface{}{"timeout": 20, "allowed_updates": []string{"message", "callback_query"}}
	if offset > 0 {
		body["offset"] = offset
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", token)
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result tgUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Ok {
		return nil, fmt.Errorf("telegram API error")
	}
	return result.Result, nil
}

// ─── Message Handler ──────────────────────────────────────────────────────────

func handleMessage(cfg telegramBackupConfig, msg *tgMessage) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}
	chatID := msg.Chat.ID
	sess := getSession(chatID)

	// Handle state-based text input first
	switch sess.State {
	case stateAwaitSearch:
		setSessionState(chatID, stateIdle, "")
		doSearch(cfg.BotToken, chatID, text)
		return
	case stateAwaitUsername:
		setSessionState(chatID, stateIdle, "")
		showUserDetails(cfg.BotToken, chatID, text)
		return
	case stateAwaitRenewUser:
		setSessionState(chatID, stateAwaitRenewProfile, text)
		showProfilesForRenew(cfg.BotToken, chatID, text)
		return
	}

	// Command routing
	lower := strings.ToLower(strings.Fields(text)[0])
	switch lower {
	case "/start", "/help":
		sendMainMenu(cfg.BotToken, chatID, msg.From.FirstName)
	case "/online":
		showOnlineStats(cfg.BotToken, chatID)
	case "/profiles":
		showProfilesList(cfg.BotToken, chatID)
	case "/users", "/subscribers":
		showUsersList(cfg.BotToken, chatID, 0)
	case "/search":
		parts := strings.Fields(text)
		if len(parts) < 2 {
			setSessionState(chatID, stateAwaitSearch, "")
			sendMessage(cfg.BotToken, chatID, "🔍 أرسل نص البحث (اسم، يوزر، أو هاتف):", nil, false)
		} else {
			doSearch(cfg.BotToken, chatID, strings.Join(parts[1:], " "))
		}
	case "/user":
		parts := strings.Fields(text)
		if len(parts) < 2 {
			setSessionState(chatID, stateAwaitUsername, "")
			sendMessage(cfg.BotToken, chatID, "👤 أرسل اسم المشترك (يوزر):", nil, false)
		} else {
			showUserDetails(cfg.BotToken, chatID, parts[1])
		}
	case "/renew":
		parts := strings.Fields(text)
		if len(parts) < 3 {
			setSessionState(chatID, stateAwaitRenewUser, "")
			sendMessage(cfg.BotToken, chatID, "🔄 أرسل اسم المشترك المراد تجديده:", nil, false)
		} else {
			doRenew(cfg.BotToken, chatID, parts[1], parts[2])
		}
	default:
		// Plain text: try as username or search
		if !strings.HasPrefix(lower, "/") {
			var exists bool
			_ = DB.QueryRow("SELECT 1 FROM radcheck WHERE username=? LIMIT 1", text).Scan(&exists)
			if exists {
				showUserDetails(cfg.BotToken, chatID, text)
			} else {
				doSearch(cfg.BotToken, chatID, text)
			}
		}
	}
}

// ─── Callback Handler ─────────────────────────────────────────────────────────

func handleCallback(cfg telegramBackupConfig, cb *tgCallbackQuery) {
	chatID := cb.Message.Chat.ID
	data := cb.Data
	answerCallback(cfg.BotToken, cb.ID)

	switch {
	case data == "menu":
		sendMainMenu(cfg.BotToken, chatID, cb.From.FirstName)
	case data == "online":
		showOnlineStats(cfg.BotToken, chatID)
	case data == "profiles":
		showProfilesList(cfg.BotToken, chatID)
	case data == "users_list":
		showUsersList(cfg.BotToken, chatID, 0)
	case strings.HasPrefix(data, "subscribers_page:"):
		pageStr := strings.TrimPrefix(data, "subscribers_page:")
		page, _ := strconv.Atoi(pageStr)
		showUsersList(cfg.BotToken, chatID, page)
	case data == "search":
		setSessionState(chatID, stateAwaitSearch, "")
		sendMessage(cfg.BotToken, chatID, "🔍 أرسل نص البحث (اسم، يوزر، أو هاتف):", nil, false)
	case data == "view_user":
		setSessionState(chatID, stateAwaitUsername, "")
		sendMessage(cfg.BotToken, chatID, "👤 أرسل اسم المشترك (يوزر):", nil, false)
	case data == "renew_start":
		setSessionState(chatID, stateAwaitRenewUser, "")
		sendMessage(cfg.BotToken, chatID, "🔄 أرسل اسم المشترك المراد تجديده:", nil, false)
	case strings.HasPrefix(data, "user:"):
		username := strings.TrimPrefix(data, "user:")
		showUserDetails(cfg.BotToken, chatID, username)
	case strings.HasPrefix(data, "renew_pick:"):
		username := strings.TrimPrefix(data, "renew_pick:")
		setSessionState(chatID, stateAwaitRenewProfile, username)
		showProfilesForRenew(cfg.BotToken, chatID, username)
	case strings.HasPrefix(data, "do_renew:"):
		// format: do_renew:username:profile
		parts := strings.SplitN(strings.TrimPrefix(data, "do_renew:"), ":", 2)
		if len(parts) == 2 {
			doRenew(cfg.BotToken, chatID, parts[0], parts[1])
		}
	case strings.HasPrefix(data, "confirm_renew:"):
		// same as do_renew alias
		parts := strings.SplitN(strings.TrimPrefix(data, "confirm_renew:"), ":", 2)
		if len(parts) == 2 {
			doRenew(cfg.BotToken, chatID, parts[0], parts[1])
		}
	case data == "cancel":
		setSessionState(chatID, stateIdle, "")
		sendMainMenu(cfg.BotToken, chatID, cb.From.FirstName)
	}
}

// ─── UI Functions ─────────────────────────────────────────────────────────────

func sendMainMenu(token string, chatID int64, name string) {
	text := fmt.Sprintf("🤖 *أهلاً %s! مرحباً بك في SASMAN* \n\nاختر من القائمة التفاعلية:", name)
	keyboard := inlineKeyboard([][]inlineBtn{
		{{Label: "🟢 المتصلون الآن", Data: "online"}, {Label: "📋 الباقات", Data: "profiles"}},
		{{Label: "🔍 بحث عن مشترك", Data: "search"}, {Label: "👤 عرض مشترك", Data: "view_user"}},
		{{Label: "👥 جميع المشتركين", Data: "users_list"}, {Label: "🔄 تجديد مشترك", Data: "renew_start"}},
	})
	sendMessage(token, chatID, text, keyboard, false)
}

func showUsersList(token string, chatID int64, page int) {
	if DB == nil {
		sendMessage(token, chatID, "❌ قاعدة البيانات غير متاحة.", backBtn(), false)
		return
	}
	limit := 10
	offset := page * limit

	// Get total count
	var total int
	err := DB.QueryRow("SELECT COUNT(*) FROM radius_user_meta").Scan(&total)
	if err != nil {
		sendMessage(token, chatID, "❌ فشل حساب عدد المشتركين: "+err.Error(), backBtn(), false)
		return
	}

	rows, err := DB.Query("SELECT username, full_name FROM radius_user_meta ORDER BY username ASC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		sendMessage(token, chatID, "❌ فشل جلب المشتركين: "+err.Error(), backBtn(), false)
		return
	}
	defer rows.Close()

	type userItem struct{ username, fullName string }
	var users []userItem
	for rows.Next() {
		var u userItem
		if rows.Scan(&u.username, &u.fullName) == nil {
			users = append(users, u)
		}
	}

	if len(users) == 0 {
		sendMessage(token, chatID, "⚠️ لا يوجد مشتركين في النظام حالياً.", backBtn(), false)
		return
	}

	totalPages := (total + limit - 1) / limit
	text := fmt.Sprintf("👥 *قائمة المشتركين (صفحة %d/%d):*\nاجمالي المشتركين: *%d*\nاضغط على المشترك لعرض تفاصيله والتحكم به:", page+1, totalPages, total)
	
	var keyboardRows [][]inlineBtn
	for _, u := range users {
		label := fmt.Sprintf("👤 %s (%s)", u.username, u.fullName)
		keyboardRows = append(keyboardRows, []inlineBtn{{Label: label, Data: "user:" + u.username}})
	}

	// Pagination buttons
	var navRow []inlineBtn
	if page > 0 {
		navRow = append(navRow, inlineBtn{Label: "◀️ السابق", Data: fmt.Sprintf("subscribers_page:%d", page-1)})
	}
	if offset+limit < total {
		navRow = append(navRow, inlineBtn{Label: "التالي ▶️", Data: fmt.Sprintf("subscribers_page:%d", page+1)})
	}
	if len(navRow) > 0 {
		keyboardRows = append(keyboardRows, navRow)
	}

	keyboardRows = append(keyboardRows, []inlineBtn{{Label: "🏠 القائمة الرئيسية", Data: "menu"}})
	sendMessage(token, chatID, text, inlineKeyboard(keyboardRows), false)
}

func showOnlineStats(token string, chatID int64) {
	sessions, err := LoadSessionsFromDB()
	if err != nil {
		sendMessage(token, chatID, "❌ فشل جلب الجلسات: "+err.Error(), backBtn(), false)
		return
	}
	online, stale := 0, 0
	for _, s := range sessions {
		if s.Online {
			online++
		} else if s.Stale {
			stale++
		}
	}
	text := fmt.Sprintf("🟢 *إحصائيات الاتصال الحالية:*\n\n🟢 متصلون الآن: *%d*\n⚠️ جلسات معلقة: *%d*\n👥 الإجمالي: *%d*",
		online, stale, online+stale)
	sendMessage(token, chatID, text, backBtn(), false)
}

func showProfilesList(token string, chatID int64) {
	if DB == nil {
		sendMessage(token, chatID, "❌ قاعدة البيانات غير متاحة.", backBtn(), false)
		return
	}
	rows, err := DB.Query("SELECT groupname, validity_days, price FROM radius_profile_meta ORDER BY price ASC")
	if err != nil {
		sendMessage(token, chatID, "❌ فشل جلب الباقات: "+err.Error(), backBtn(), false)
		return
	}
	defer rows.Close()
	var sb strings.Builder
	sb.WriteString("📋 *الباقات المتوفرة:*\n\n")
	count := 0
	for rows.Next() {
		var name string
		var days int
		var price float64
		if rows.Scan(&name, &days, &price) == nil {
			sb.WriteString(fmt.Sprintf("📦 *%s* | ⏱️ %d يوم | 💵 %.0f دينار\n", name, days, price))
			count++
		}
	}
	if count == 0 {
		sb.WriteString("⚠️ لا توجد باقات.")
	}
	sendMessage(token, chatID, sb.String(), backBtn(), false)
}

func doSearch(token string, chatID int64, query string) {
	if DB == nil {
		sendMessage(token, chatID, "❌ قاعدة البيانات غير متاحة.", backBtn(), false)
		return
	}
	like := "%" + query + "%"
	rows, err := DB.Query(`SELECT username, full_name, phone FROM radius_user_meta
		WHERE username LIKE ? OR full_name LIKE ? OR phone LIKE ? LIMIT 10`, like, like, like)
	if err != nil {
		sendMessage(token, chatID, "❌ فشل البحث: "+err.Error(), backBtn(), false)
		return
	}
	defer rows.Close()

	type result struct{ username, fullName, phone string }
	var results []result
	for rows.Next() {
		var r result
		if rows.Scan(&r.username, &r.fullName, &r.phone) == nil {
			results = append(results, r)
		}
	}

	if len(results) == 0 {
		sendMessage(token, chatID, fmt.Sprintf("⚠️ لا نتائج للبحث: *%s*", query), backBtn(), false)
		return
	}

	text := fmt.Sprintf("🔍 *نتائج البحث (%s):*\nاضغط لعرض تفاصيل المشترك:", query)
	var rows2 [][]inlineBtn
	for _, r := range results {
		label := fmt.Sprintf("👤 %s (%s)", r.username, r.fullName)
		rows2 = append(rows2, []inlineBtn{{Label: label, Data: "user:" + r.username}})
	}
	rows2 = append(rows2, []inlineBtn{{Label: "🏠 القائمة الرئيسية", Data: "menu"}})
	sendMessage(token, chatID, text, inlineKeyboard(rows2), false)
}

func showUserDetails(token string, chatID int64, username string) {
	if DB == nil {
		sendMessage(token, chatID, "❌ قاعدة البيانات غير متاحة.", backBtn(), false)
		return
	}
	var fullName, phone, profile string
	var balance float64
	var expirationUnix sql.NullInt64
	var enabled int

	err := DB.QueryRow(`
		SELECT m.full_name, m.phone, m.balance, m.expiration_unix, m.enabled, COALESCE(g.groupname,'')
		FROM radius_user_meta m
		LEFT JOIN radusergroup g ON m.username=g.username
		WHERE m.username=?`, username).Scan(&fullName, &phone, &balance, &expirationUnix, &enabled, &profile)

	if err == sql.ErrNoRows {
		sendMessage(token, chatID, fmt.Sprintf("❌ المشترك *%s* غير موجود.", username), backBtn(), false)
		return
	}
	if err != nil {
		sendMessage(token, chatID, "❌ خطأ: "+err.Error(), backBtn(), false)
		return
	}

	statusIcon := "🔴 منتهي"
	if enabled == 0 {
		statusIcon = "🚫 معطل"
	} else if expirationUnix.Valid && expirationUnix.Int64 > time.Now().Unix() {
		statusIcon = "🟢 نشط"
	}

	expiryStr := "غير محدد"
	if expirationUnix.Valid {
		expiryStr = time.Unix(expirationUnix.Int64, 0).In(baghdadLocation).Format("2006-01-02 15:04")
	}

	connStatus := "🔴 أوفلاين"
	sessions, _ := LoadSessionsFromDB()
	if info, ok := sessions[strings.ToLower(username)]; ok && info.Online {
		connStatus = fmt.Sprintf("🟢 أونلاين | IP: `%s`", info.IP)
	}

	text := fmt.Sprintf(
		"👤 *المشترك:* `%s`\n\n"+
			"📛 الاسم: %s\n📞 الهاتف: %s\n💰 الرصيد: %.0f دينار\n"+
			"📦 الباقة: %s\n⚡ الحالة: %s\n📅 الانتهاء: `%s`\n📡 الاتصال: %s",
		username, fullName, phone, balance, profile, statusIcon, expiryStr, connStatus)

	keyboard := inlineKeyboard([][]inlineBtn{
		{{Label: "🔄 تجديد هذا المشترك", Data: "renew_pick:" + username}},
		{{Label: "🔍 بحث آخر", Data: "search"}, {Label: "🏠 الرئيسية", Data: "menu"}},
	})
	sendMessage(token, chatID, text, keyboard, false)
}

func showProfilesForRenew(token string, chatID int64, username string) {
	if DB == nil {
		sendMessage(token, chatID, "❌ قاعدة البيانات غير متاحة.", backBtn(), false)
		return
	}
	rows, err := DB.Query("SELECT groupname, validity_days, price FROM radius_profile_meta ORDER BY price ASC")
	if err != nil {
		sendMessage(token, chatID, "❌ فشل جلب الباقات: "+err.Error(), backBtn(), false)
		return
	}
	defer rows.Close()

	text := fmt.Sprintf("🔄 *تجديد المشترك:* `%s`\nاختر الباقة:", username)
	var btns [][]inlineBtn
	for rows.Next() {
		var name string
		var days int
		var price float64
		if rows.Scan(&name, &days, &price) == nil {
			label := fmt.Sprintf("📦 %s | %d يوم | %.0f دينار", name, days, price)
			btns = append(btns, []inlineBtn{{Label: label, Data: fmt.Sprintf("do_renew:%s:%s", username, name)}})
		}
	}
	btns = append(btns, []inlineBtn{{Label: "❌ إلغاء", Data: "cancel"}})
	sendMessage(token, chatID, text, inlineKeyboard(btns), false)
}

func doRenew(token string, chatID int64, username, profile string) {
	if DB == nil {
		sendMessage(token, chatID, "❌ قاعدة البيانات غير متاحة.", backBtn(), false)
		return
	}
	setSessionState(chatID, stateIdle, "")

	var exists bool
	_ = DB.QueryRow("SELECT 1 FROM radcheck WHERE username=? AND attribute='Cleartext-Password' LIMIT 1", username).Scan(&exists)
	if !exists {
		sendMessage(token, chatID, fmt.Sprintf("❌ المشترك *%s* غير موجود.", username), backBtn(), false)
		return
	}

	validityDays, err := loadProfileValidityDays(profile)
	if err != nil {
		sendMessage(token, chatID, fmt.Sprintf("❌ الباقة *%s* غير موجودة.", profile), backBtn(), false)
		return
	}

	currentExpiry, err := loadUserExpiration(username)
	if err != nil {
		sendMessage(token, chatID, "❌ فشل جلب تاريخ الانتهاء: "+err.Error(), backBtn(), false)
		return
	}

	newExpiry := calculateRenewedExpiration(currentExpiry, validityDays)

	tx, err := DB.Begin()
	if err != nil {
		sendMessage(token, chatID, "❌ فشل بدء المعاملة: "+err.Error(), backBtn(), false)
		return
	}
	defer tx.Rollback()

	if err = replaceUserProfileTx(tx, username, profile); err != nil {
		sendMessage(token, chatID, "❌ فشل تغيير الباقة: "+err.Error(), backBtn(), false)
		return
	}
	if err = saveUserExpirationTx(tx, username, newExpiry); err != nil {
		sendMessage(token, chatID, "❌ فشل حفظ الانتهاء: "+err.Error(), backBtn(), false)
		return
	}
	if err = saveUserMetaTx(tx, username, newExpiry, 1); err != nil {
		sendMessage(token, chatID, "❌ فشل حفظ البيانات: "+err.Error(), backBtn(), false)
		return
	}
	if err = tx.Commit(); err != nil {
		sendMessage(token, chatID, "❌ فشل تأكيد التجديد: "+err.Error(), backBtn(), false)
		return
	}

	QueueUserSync(username)
	go KickUserIfOnline(username)

	expiryStr := formatUnixDateTime(newExpiry)
	if expiryStr == "" {
		expiryStr = "غير محدد"
	}

	text := fmt.Sprintf("✅ *تم تجديد المشترك بنجاح!*\n\n"+
		"👤 المشترك: `%s`\n📦 الباقة: *%s* (%d يوم)\n📅 الانتهاء الجديد: `%s`\n\n"+
		"⚡ تمت المزامنة وفصل الجلسة الحالية لتطبيق التغييرات فوراً.",
		username, profile, validityDays, expiryStr)

	keyboard := inlineKeyboard([][]inlineBtn{
		{{Label: "👤 عرض بيانات المشترك", Data: "user:" + username}},
		{{Label: "🔄 تجديد آخر", Data: "renew_start"}, {Label: "🏠 الرئيسية", Data: "menu"}},
	})
	sendMessage(token, chatID, text, keyboard, false)
}

// ─── Keyboard Builders ────────────────────────────────────────────────────────

type inlineBtn struct {
	Label string
	Data  string
}

func inlineKeyboard(rows [][]inlineBtn) map[string]interface{} {
	var kb [][]map[string]string
	for _, row := range rows {
		var r []map[string]string
		for _, b := range row {
			r = append(r, map[string]string{"text": b.Label, "callback_data": b.Data})
		}
		kb = append(kb, r)
	}
	return map[string]interface{}{"inline_keyboard": kb}
}

func backBtn() map[string]interface{} {
	return inlineKeyboard([][]inlineBtn{{{Label: "🏠 القائمة الرئيسية", Data: "menu"}}})
}

// ─── API Helpers ──────────────────────────────────────────────────────────────

func sendMessage(token string, chatID int64, text string, replyMarkup interface{}, disablePreview bool) {
	body := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	if replyMarkup != nil {
		body["reply_markup"] = replyMarkup
	}
	if disablePreview {
		body["disable_web_page_preview"] = true
	}
	if _, err := tgPost[map[string]interface{}](token, "sendMessage", body); err != nil {
		log.Printf("[telegram-bot] sendMessage error: %v", err)
	}
}

func answerCallback(token, callbackID string) {
	tgPost[bool](token, "answerCallbackQuery", map[string]interface{}{ //nolint
		"callback_query_id": callbackID,
	})
}

func tgPost[T any](token, method string, body interface{}) (T, error) {
	var zero T
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method)
	data, err := json.Marshal(body)
	if err != nil {
		return zero, err
	}
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	var result struct {
		Ok     bool `json:"ok"`
		Result T    `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, err
	}
	return result.Result, nil
}

// ─── Command Registration ─────────────────────────────────────────────────────

func registerBotCommands(token string) {
	type cmd struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	cmds := []cmd{
		{"start", "القائمة الرئيسية"},
		{"help", "المساعدة"},
		{"online", "المتصلون الآن"},
		{"profiles", "الباقات المتوفرة"},
		{"users", "عرض جميع المشتركين"},
		{"search", "بحث عن مشترك"},
		{"user", "عرض تفاصيل مشترك"},
		{"renew", "تجديد مشترك"},
	}
	if _, err := tgPost[bool](token, "setMyCommands", map[string]interface{}{"commands": cmds}); err != nil {
		log.Printf("[telegram-bot] setMyCommands error: %v", err)
	} else {
		log.Println("[telegram-bot] Commands registered successfully.")
	}
}
