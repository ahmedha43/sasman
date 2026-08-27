package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"mikrotik-manager/pkg/tunnel"
	"mikrotik-manager/server/internal/storage"
)

type Engine struct {
	repo      *storage.SQLiteRepository
	tunnelSvc *tunnel.Service
	llm       *LLMClient
	mcpBridge *MCPBridge
}

func NewEngine(repo *storage.SQLiteRepository, tunnelSvc *tunnel.Service) *Engine {
	return &Engine{
		repo:      repo,
		tunnelSvc: tunnelSvc,
		llm:       NewLLMClient(),
		mcpBridge: NewMCPBridge(),
	}
}

// getAgentRouterAuth retrieves the stored router credentials for the target agent
func (e *Engine) getAgentRouterAuth(subdomain string) map[string]string {
	sub := strings.ToLower(strings.TrimSpace(subdomain))
	if sub == "" {
		return nil
	}

	credsMap, err := e.repo.GetSubdomainCredentialsMap()
	if err == nil {
		if creds, ok := credsMap[sub]; ok {
			if m, ok := creds["mikrotik"].(map[string]interface{}); ok {
				host, _ := m["host"].(string)
				if host == "" {
					host, _ = m["address"].(string)
				}
				user, _ := m["username"].(string)
				pass, _ := m["password"].(string)
				if user != "" || pass != "" {
					return map[string]string{
						"host": host,
						"user": user,
						"pass": pass,
					}
				}
			}
		}
	}

	// Fallback to active agent session sync data
	if agent := e.tunnelSvc.GetAgentBySubdomain(subdomain); agent != nil {
		if agent.SyncData != nil {
			if m, ok := agent.SyncData["credentials"].(map[string]interface{}); ok {
				if mt, ok := m["mikrotik"].(map[string]interface{}); ok {
					host, _ := mt["host"].(string)
					user, _ := mt["username"].(string)
					pass, _ := mt["password"].(string)
					if user != "" || pass != "" {
						return map[string]string{
							"host": host,
							"user": user,
							"pass": pass,
						}
					}
				}
			}
		}
	}
	return nil
}

// ExecuteRouterCommand sends a RouterOS CLI command to the specified agent router
func (e *Engine) ExecuteRouterCommand(subdomain string, command string) (map[string]interface{}, error) {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return nil, fmt.Errorf("يجب تحديد اسم نطاق الوكيل")
	}

	cmdParts := strings.Fields(strings.TrimSpace(command))
	if len(cmdParts) == 0 {
		return nil, fmt.Errorf("أمر المايكروتك فارغ")
	}

	payload := map[string]interface{}{
		"command": cmdParts,
	}
	if auth := e.getAgentRouterAuth(subdomain); auth != nil {
		payload["router_auth"] = auth
	}

	reqBody, _ := json.Marshal(payload)

	_, respBytes, err := e.tunnelSvc.SendAgentHTTPRequest(subdomain, "POST", "/radius/api/internal/routeros/exec", reqBody, nil)
	if err != nil {
		return nil, fmt.Errorf("فشل إرسال الأمر للراوتر: %w", err)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return map[string]interface{}{"raw": string(respBytes)}, nil
	}
	return res, nil
}

// ExecuteSystemAudit retrieves full snapshot of system resources, firewall, interfaces, and logs
func (e *Engine) ExecuteSystemAudit(subdomain string) (map[string]interface{}, error) {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return nil, fmt.Errorf("يجب تحديد اسم نطاق الوكيل")
	}

	payload := map[string]interface{}{
		"audit": true,
	}
	if auth := e.getAgentRouterAuth(subdomain); auth != nil {
		payload["router_auth"] = auth
	}

	reqBody, _ := json.Marshal(payload)

	_, respBytes, err := e.tunnelSvc.SendAgentHTTPRequest(subdomain, "POST", "/radius/api/internal/routeros/exec", reqBody, nil)
	if err != nil {
		return nil, fmt.Errorf("فشل فحص الراوتر: %w", err)
	}

	var res struct {
		Success bool                   `json:"success"`
		Audit   map[string]interface{} `json:"audit"`
		Error   string                 `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return nil, fmt.Errorf("تنسيق رد الفحص غير صحيح: %w", err)
	}
	if !res.Success && res.Error != "" {
		return nil, fmt.Errorf("خطأ فحص الراوتر: %s", res.Error)
	}
	return res.Audit, nil
}

// ExecuteBatchCommands runs multiple commands in sequence on the agent router
func (e *Engine) ExecuteBatchCommands(subdomain string, commands []string) (map[string]interface{}, error) {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return nil, fmt.Errorf("يجب تحديد اسم نطاق الوكيل")
	}

	var cmdList [][]string
	for _, c := range commands {
		parts := strings.Fields(strings.TrimSpace(c))
		if len(parts) > 0 {
			cmdList = append(cmdList, parts)
		}
	}

	if len(cmdList) == 0 {
		return nil, fmt.Errorf("لا توجد أوامر صالحة للتنفيذ")
	}

	payload := map[string]interface{}{
		"commands": cmdList,
	}
	if auth := e.getAgentRouterAuth(subdomain); auth != nil {
		payload["router_auth"] = auth
	}

	reqBody, _ := json.Marshal(payload)

	_, respBytes, err := e.tunnelSvc.SendAgentHTTPRequest(subdomain, "POST", "/radius/api/internal/routeros/exec", reqBody, nil)
	if err != nil {
		return nil, fmt.Errorf("فشل إرسال حزمة الأوامر للراوتر: %w", err)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return map[string]interface{}{"raw": string(respBytes)}, nil
	}
	return res, nil
}

// Chat handles conversation with the AI copilot including RouterOS MCP tool-calling loops
func (e *Engine) Chat(ctx context.Context, messages []ChatMessage, targetSubdomain string) (*ChatMessage, *ChangePlan, error) {
	return e.ChatStream(ctx, messages, targetSubdomain, nil)
}

// ChatStream handles conversation with the AI copilot and streams every agentic step and tool execution
func (e *Engine) ChatStream(ctx context.Context, messages []ChatMessage, targetSubdomain string, onStep func(StreamEvent)) (*ChatMessage, *ChangePlan, error) {
	emit := func(ev StreamEvent) {
		if ev.Timestamp == "" {
			ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
		}
		if onStep != nil {
			onStep(ev)
		}
	}

	settings, err := e.repo.GetAISettings()
	if err != nil {
		emit(StreamEvent{Type: "error", Text: err.Error()})
		return nil, nil, err
	}
	if !settings.Enabled {
		err := fmt.Errorf("خدمة المساعد الذكي معطلة حالياً من الإعدادات")
		emit(StreamEvent{Type: "error", Text: err.Error()})
		return nil, nil, err
	}

	systemPrompt := SystemPromptTemplate
	if settings.SystemPrompt != "" {
		systemPrompt = settings.SystemPrompt
	}
	if targetSubdomain != "" {
		systemPrompt += fmt.Sprintf("\nالوكيل والراوتر المستهدف حالياً: %s", targetSubdomain)

		// Inject per-agent persistent memory context
		if memBlock := e.buildAgentMemoryPromptBlock(targetSubdomain); memBlock != "" {
			systemPrompt += memBlock
			emit(StreamEvent{
				Type:  "thought",
				Title: "استرجاع ذاكرة الوكيل",
				Text:  fmt.Sprintf("تم استرجاع ذاكرة وسجل الوكيل (%s) المحفوظة لتجنب استهلاك التوكنات وتكرار الأسئلة.", targetSubdomain),
			})
		}
	}

	emit(StreamEvent{
		Type:  "thought",
		Title: "بدء المعالجة والتفكير",
		Text:  fmt.Sprintf("جاري تحليل الطلب والتحقق من الراوتر المستهدف (%s)...", targetSubdomain),
	})

	if targetSubdomain != "" {
		if auth := e.getAgentRouterAuth(targetSubdomain); auth != nil {
			emit(StreamEvent{
				Type:  "thought",
				Title: "تجهيز بيانات الدخول",
				Text:  fmt.Sprintf("تم تجهيز بيانات دخول الراوتر (%s) بنجاح.", auth["user"]),
			})
		}
	}

	conversation := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}
	conversation = append(conversation, messages...)

	tools := GetRouterOSToolDefinitions()
	var finalPlan *ChangePlan

	// Execute Tool Calling loop (max 5 iterations)
	for iter := 0; iter < 5; iter++ {
		emit(StreamEvent{
			Type:  "thought",
			Title: "استدعاء نموذج الذكاء الاصطناعي",
			Text:  fmt.Sprintf("جاري التخطيط للخطوات بواسطة %s (%s)...", settings.Provider, settings.Model),
		})

		req := ChatCompletionRequest{
			Model:       settings.Model,
			Messages:    conversation,
			Tools:       tools,
			Temperature: settings.Temperature,
		}

		resp, err := e.llm.Complete(ctx, settings, req)
		if err != nil {
			emit(StreamEvent{Type: "error", Text: err.Error()})
			return nil, nil, err
		}

		if len(resp.Choices) == 0 {
			err := fmt.Errorf("لم يتم استلام رد من نموذج الذكاء الاصطناعي")
			emit(StreamEvent{Type: "error", Text: err.Error()})
			return nil, nil, err
		}

		choice := resp.Choices[0]
		replyMsg := choice.Message

		// If no tool calls, this is the final message
		if len(replyMsg.ToolCalls) == 0 {
			emit(StreamEvent{
				Type:    "done",
				Title:   "اكتمل الرد بنجاح",
				Text:    replyMsg.Content,
				Message: &replyMsg,
				Plan:    finalPlan,
			})

			// Auto-record compact session summary into agent memory
			if targetSubdomain != "" {
				e.saveChatSessionMemory(targetSubdomain, messages, replyMsg.Content, finalPlan)
			}

			return &replyMsg, finalPlan, nil
		}

		conversation = append(conversation, replyMsg)

		// Execute tool calls requested by the model
		for _, tc := range replyMsg.ToolCalls {
			var fnName string
			var argsStr string
			var tcID string

			if id, ok := tc["id"].(string); ok {
				tcID = id
			}
			if fn, ok := tc["function"].(map[string]interface{}); ok {
				if n, ok := fn["name"].(string); ok {
					fnName = n
				}
				if a, ok := fn["arguments"].(string); ok {
					argsStr = a
				}
			}

			var args map[string]interface{}
			_ = json.Unmarshal([]byte(argsStr), &args)

			sub := targetSubdomain
			if s, ok := args["subdomain"].(string); ok && s != "" {
				sub = s
			}
			if s, ok := args["target_router"].(string); ok && s != "" {
				sub = s
			}

			toolTitle := fnName
			switch fnName {
			case "mikrotik_audit_router":
				toolTitle = fmt.Sprintf("🛡️ فحص شامل لراوتر (%s)", sub)
			case "mikrotik_run_command":
				cmdStr, _ := args["command"].(string)
				toolTitle = fmt.Sprintf("⚡ تنفيذ أمر RouterOS: `%s`", cmdStr)
			case "mikrotik_attack_detection":
				toolTitle = fmt.Sprintf("🔐 كشف سجلات الهجمات لراوتر (%s)", sub)
			case "mikrotik_generate_plan":
				titleStr, _ := args["title"].(string)
				toolTitle = fmt.Sprintf("📋 توليد خطة تعديل: %s", titleStr)
			}

			emit(StreamEvent{
				Type:   "tool_start",
				Tool:   fnName,
				Title:  toolTitle,
				Args:   args,
				Status: "running",
				Text:   fmt.Sprintf("جاري استدعاء الأداة `%s` عبر نفق الوكيل المشفر...", fnName),
			})

			startTime := time.Now()
			var toolResult interface{}

			switch fnName {
			case "mikrotik_audit_router":
				emit(StreamEvent{
					Type:  "tunnel_exec",
					Tool:  fnName,
					Title: "نفق WebSocket",
					Text:  fmt.Sprintf("إرسال طلب فحص الـ 14 جدول للراوتر `%s` عبر النفق...", sub),
				})
				audit, err := e.ExecuteSystemAudit(sub)
				if err != nil {
					toolResult = map[string]string{"error": err.Error()}
					emit(StreamEvent{
						Type:     "tool_result",
						Tool:     fnName,
						Status:   "error",
						Summary:  "تعذر الاتصال بالراوتر: " + err.Error(),
						Duration: fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
					})
				} else {
					toolResult = audit
					emit(StreamEvent{
						Type:     "tool_result",
						Tool:     fnName,
						Status:   "success",
						Summary:  fmt.Sprintf("تم استلام بيانات الفحص بنجاح (%d جدول، الموارد، الفايروول، السجلات)", len(audit)),
						Duration: fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
					})
				}

			case "mikrotik_run_command":
				cmd, _ := args["command"].(string)
				emit(StreamEvent{
					Type:  "tunnel_exec",
					Tool:  fnName,
					Title: "تنفيذ أمر RouterOS",
					Text:  fmt.Sprintf("تشغيل `%s` على راوتر `%s`...", cmd, sub),
				})
				res, err := e.ExecuteRouterCommand(sub, cmd)
				if err != nil {
					toolResult = map[string]string{"error": err.Error()}
					emit(StreamEvent{
						Type:     "tool_result",
						Tool:     fnName,
						Status:   "error",
						Summary:  "خطأ في التنفيذ: " + err.Error(),
						Duration: fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
					})
				} else {
					toolResult = res
					emit(StreamEvent{
						Type:     "tool_result",
						Tool:     fnName,
						Status:   "success",
						Summary:  "تم تنفيذ الأمر بنجاح واستلام النتيجة الحية من الراوتر",
						Duration: fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
					})
				}

			case "mikrotik_attack_detection":
				emit(StreamEvent{
					Type:  "tunnel_exec",
					Tool:  fnName,
					Title: "جلب السجلات",
					Text:  fmt.Sprintf("جلب سجلات الـ Log من راوتر `%s` وتحليل محاولات الدخول...", sub),
				})
				audit, err := e.ExecuteSystemAudit(sub)
				if err != nil {
					toolResult = map[string]string{"error": err.Error()}
					emit(StreamEvent{
						Type:     "tool_result",
						Tool:     fnName,
						Status:   "error",
						Summary:  "تعذر جلب السجلات: " + err.Error(),
						Duration: fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
					})
				} else {
					var logs []interface{}
					if l, ok := audit["logs"].([]interface{}); ok {
						logs = l
					}
					toolResult = map[string]interface{}{
						"logs": logs,
					}
					emit(StreamEvent{
						Type:     "tool_result",
						Tool:     fnName,
						Status:   "success",
						Summary:  fmt.Sprintf("تم تحليل %d سجل من سجلات الراوتر", len(logs)),
						Duration: fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
					})
				}

			case "mikrotik_generate_plan":
				title, _ := args["title"].(string)
				desc, _ := args["description"].(string)
				risk, _ := args["risk_level"].(string)
				var cmds, rollbacks []string
				if cList, ok := args["commands"].([]interface{}); ok {
					for _, c := range cList {
						if str, ok := c.(string); ok {
							cmds = append(cmds, str)
						}
					}
				}
				if rList, ok := args["rollback"].([]interface{}); ok {
					for _, r := range rList {
						if str, ok := r.(string); ok {
							rollbacks = append(rollbacks, str)
						}
					}
				}

				diffBuilder := strings.Builder{}
				diffBuilder.WriteString("```diff\n")
				for _, c := range cmds {
					diffBuilder.WriteString("+ " + c + "\n")
				}
				for _, r := range rollbacks {
					diffBuilder.WriteString("- (Rollback) " + r + "\n")
				}
				diffBuilder.WriteString("```")

				plan := &ChangePlan{
					Title:        title,
					Description:  desc,
					TargetRouter: sub,
					Commands:     cmds,
					Rollback:     rollbacks,
					DiffPreview:  diffBuilder.String(),
					RiskLevel:    risk,
				}
				finalPlan = plan
				toolResult = map[string]interface{}{
					"status":  "plan_generated",
					"message": "تم توليد خطة التعديل بنجاح وبانتظار موافقة المستخدم لتطبيقها.",
					"plan":    plan,
				}
				emit(StreamEvent{
					Type:     "plan",
					Plan:     plan,
					Status:   "success",
					Summary:  fmt.Sprintf("تم تجهيز خطة التعديل (%d أوامر تنفيذ + %d أوامر تراجع)", len(cmds), len(rollbacks)),
					Duration: fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
				})

			default:
				toolResult = map[string]string{"error": "أداة غير معروفة"}
			}

			resultBytes, _ := json.Marshal(toolResult)
			conversation = append(conversation, ChatMessage{
				Role:       "tool",
				Name:       fnName,
				ToolCallID: tcID,
				Content:    string(resultBytes),
			})
		}
	}

	return nil, finalPlan, fmt.Errorf("تم تجاوز الحد الأقصى لدورات استدعاء الأدوات")
}

// RunSecurityAudit performs a full heuristic + LLM-backed security and health assessment of an agent router
func (e *Engine) RunSecurityAudit(ctx context.Context, subdomain string) (*DiagnosticReport, error) {
	auditData, err := e.ExecuteSystemAudit(subdomain)
	if err != nil {
		return nil, err
	}

	report := &DiagnosticReport{
		Subdomain:       subdomain,
		Score:           100,
		OverallStatus:   "excellent",
		ResourceSummary: make(map[string]string),
		Findings:        []SecurityFinding{},
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	// 1. Extract Resources
	if resList, ok := auditData["resource"].([]interface{}); ok && len(resList) > 0 {
		if res, ok := resList[0].(map[string]interface{}); ok {
			report.ResourceSummary["cpu_load"] = fmt.Sprintf("%v%%", res["cpu-load"])
			report.ResourceSummary["uptime"] = fmt.Sprintf("%v", res["uptime"])
			report.ResourceSummary["version"] = fmt.Sprintf("%v", res["version"])
			report.ResourceSummary["free_memory"] = fmt.Sprintf("%v", res["free-memory"])
			report.ResourceSummary["board_name"] = fmt.Sprintf("%v", res["board-name"])

			if cpuStr, ok := res["cpu-load"].(string); ok {
				if cpuVal, err := strconv.Atoi(cpuStr); err == nil && cpuVal > 80 {
					report.Score -= 20
					report.Findings = append(report.Findings, SecurityFinding{
						Category:    "system",
						Severity:    "critical",
						Title:       "استهلاك المعالج مرتفع جداً (High CPU Load)",
						Description: fmt.Sprintf("نسبة استهلاك المعالج بلغت %d%%، مما قد يسبب بطء تصفح للمشتركين وتأخير في الاستجابة.", cpuVal),
						FixPlan:     "فحص قائمة الـ Profile / Torch ومعرفة القواعد أو السكربتات المسببة للضغط.",
					})
				}
			}
		}
	}

	// 2. Check DNS Open Resolver
	if dnsList, ok := auditData["dns"].([]interface{}); ok && len(dnsList) > 0 {
		if dns, ok := dnsList[0].(map[string]interface{}); ok {
			if allowRemote, ok := dns["allow-remote-requests"].(string); ok && allowRemote == "yes" || allowRemote == "true" {
				report.Score -= 25
				report.Findings = append(report.Findings, SecurityFinding{
					Category:    "firewall",
					Severity:    "critical",
					Title:       "خطر DNS Open Resolver (منفذ 53 مفتوح)",
					Description: "خاصية allow-remote-requests مفعلة بالراوتر بدون تقييد، مما يجعله عرضة لاستخدامه في هجمات الـ DNS Amplification DDoS وحجب الخدمة.",
					FixPlan:     "إغلاق طلبات الـ DNS من جهة الـ WAN عبر إضافة رول Drop في الفايروول.",
					FixCommand:  "/ip firewall filter add chain=input action=drop protocol=udp dst-port=53 in-interface-list=WAN comment=\"Drop WAN DNS\"",
				})
			}
		}
	}

	// 3. Check FastTrack Status
	hasFastTrack := false
	if filterList, ok := auditData["firewall_filter"].([]interface{}); ok {
		for _, f := range filterList {
			if rule, ok := f.(map[string]interface{}); ok {
				if action, ok := rule["action"].(string); ok && action == "fasttrack-connection" {
					if disabled, ok := rule["disabled"].(string); !ok || disabled == "false" || disabled == "no" {
						hasFastTrack = true
					}
				}
			}
		}
	}
	if !hasFastTrack {
		report.Score -= 15
		report.Findings = append(report.Findings, SecurityFinding{
			Category:    "system",
			Severity:    "warning",
			Title:       "ميزة FastTrack معطلة أو غير موجودة",
			Description: "تفعيل FastTrack يقلل استهلاك المعالج بنسبة تصل إلى 70% عند ترافيك عالي لمعالجة الحزم المعتمدة مسبقاً.",
			FixPlan:     "إضافة رول FastTrack في قائمة Forward الفايروول.",
			FixCommand:  "/ip firewall filter add chain=forward action=fasttrack-connection connection-state=established,related comment=\"FastTrack Traffic\"",
		})
	}

	// 4. Check Open Sensitive Services (Telnet, API, Web, Winbox)
	if svcList, ok := auditData["ip_services"].([]interface{}); ok {
		for _, s := range svcList {
			if svc, ok := s.(map[string]interface{}); ok {
				name, _ := svc["name"].(string)
				disabled, _ := svc["disabled"].(string)
				address, _ := svc["address"].(string)

				if (name == "telnet" || name == "ftp") && (disabled == "no" || disabled == "false") {
					report.Score -= 10
					report.Findings = append(report.Findings, SecurityFinding{
						Category:    "service",
						Severity:    "warning",
						Title:       fmt.Sprintf("خدمة %s غير المشفرة مفعلة", strings.ToUpper(name)),
						Description: "خدمات Telnet و FTP ترسل كلمات المرور بنص واضح غير مشفر عبر الشبكة.",
						FixPlan:     fmt.Sprintf("تعطيل خدمة %s والاعتماد على SSH و Winbox الآمنين.", name),
						FixCommand:  fmt.Sprintf("/ip service disable %s", name),
					})
				}

				if (name == "api" || name == "winbox") && (disabled == "no" || disabled == "false") && address == "" {
					report.Findings = append(report.Findings, SecurityFinding{
						Category:    "service",
						Severity:    "info",
						Title:       fmt.Sprintf("منفذ %s متاح لجميع الآيبيهات بدون تقييد", strings.ToUpper(name)),
						Description: "يُفضل تقييد عناوين الدخول المسموح بها في قائمة Address بالخدمة لزيادة الأمان.",
					})
				}
			}
		}
	}

	// 5. Generate Mermaid Network Topology
	mermaidBuilder := strings.Builder{}
	mermaidBuilder.WriteString("graph TD\n")
	mermaidBuilder.WriteString("    WAN[\"🌐 شبكة الإنترنت (WAN / ISP)\"]\n")
	mermaidBuilder.WriteString(fmt.Sprintf("    Router[\"🖥️ راوتر المايكروتك (%s)\"]\n", subdomain))
	mermaidBuilder.WriteString("    WAN --> Router\n")

	if ifList, ok := auditData["interfaces"].([]interface{}); ok {
		for idx, iface := range ifList {
			if m, ok := iface.(map[string]interface{}); ok {
				name, _ := m["name"].(string)
				ifType, _ := m["type"].(string)
				running, _ := m["running"].(string)
				if running == "true" || running == "yes" {
					nodeID := fmt.Sprintf("IF_%d", idx)
					mermaidBuilder.WriteString(fmt.Sprintf("    Router --> %s[\"🔌 %s (%s)\"]\n", nodeID, name, ifType))
				}
			}
		}
	}
	report.MermaidTopology = mermaidBuilder.String()

	// Adjust status
	if report.Score < 60 {
		report.OverallStatus = "critical"
	} else if report.Score < 85 {
		report.OverallStatus = "warning"
	} else if report.Score < 95 {
		report.OverallStatus = "good"
	} else {
		report.OverallStatus = "excellent"
	}

	// 6. Generate Narrative with LLM
	auditJSON, _ := json.MarshalIndent(auditData, "", "  ")
	promptMsg := fmt.Sprintf(`حلل تقرير فحص راوتر المايكروتك التالي للوكيل "%s":
- درجة التقييم الحالية: %d/100
- المشاكل المكتشفة أولياً: %d
بيانات الفحص الخام:
%s

اكتب تحليلاً مختصراً وواضحاً باللغة العربية (ملخص الحالة، أهم 3 نصائح لتحسين الشبكة والأمان، وتأثير المشاكل الحالية على المشتركين).`, subdomain, report.Score, len(report.Findings), string(auditJSON))

	settings, _ := e.repo.GetAISettings()
	if settings != nil && settings.Enabled && settings.APIKey != "" {
		llmResp, err := e.llm.Complete(ctx, settings, ChatCompletionRequest{
			Model: settings.Model,
			Messages: []ChatMessage{
				{Role: "system", Content: SystemPromptTemplate},
				{Role: "user", Content: promptMsg},
			},
			Temperature: 0.3,
		})
		if err == nil && len(llmResp.Choices) > 0 {
			report.Narrative = llmResp.Choices[0].Message.Content
		}
	}

	if report.Narrative == "" {
		report.Narrative = fmt.Sprintf("تم إكمال الفحص الشامل لراوتر %s بنجاح. النتيجة العامة: %d/100 (%s). تم اكتشاف %d عناصر فنية وأمنية تحتاج مراجعة.",
			subdomain, report.Score, report.OverallStatus, len(report.Findings))
	}

	// 7. Save Audit Log in SQLite
	findingsBytes, _ := json.Marshal(report.Findings)
	_ = e.repo.SaveAIAuditLog(storage.AIAuditLog{
		Subdomain:           subdomain,
		AuditType:           "security_and_health",
		Score:               report.Score,
		FindingsJSON:        string(findingsBytes),
		RecommendationsJSON: "[]",
		RawSummary:          report.Narrative,
	})

	// 8. Update Agent Persistent Memory
	lastAuditData := map[string]interface{}{
		"score":    report.Score,
		"status":   report.OverallStatus,
		"findings": report.Findings,
		"date":     report.CreatedAt,
	}
	_ = e.repo.UpdateAgentLastAudit(subdomain, lastAuditData)

	routerInfoMap := map[string]interface{}{}
	for k, v := range report.ResourceSummary {
		routerInfoMap[k] = v
	}
	if len(routerInfoMap) > 0 {
		_ = e.repo.UpdateAgentRouterInfo(subdomain, routerInfoMap)
	}

	log.Printf("[AI Copilot] Completed audit for %s: score=%d, findings=%d", subdomain, report.Score, len(report.Findings))
	return report, nil
}

// buildAgentMemoryPromptBlock constructs a concise memory context to prevent token waste
func (e *Engine) buildAgentMemoryPromptBlock(subdomain string) string {
	if subdomain == "" {
		return ""
	}
	mem, err := e.repo.GetAgentMemory(subdomain)
	if err != nil || mem == nil {
		return ""
	}

	var sb strings.Builder
	hasContent := false

	sb.WriteString("\n\n### 🧠 ذاكرة وسجل الوكيل والراوتر المحفوظة (" + subdomain + "):\n")
	sb.WriteString("(استند إلى هذه المعلومات السابقة ولا تطلب من المستخدم إعادة إدخالها):\n")

	if len(mem.RouterInfo) > 0 {
		hasContent = true
		sb.WriteString("- **معلومات الراوتر المكتشفة سابقاً**: ")
		for k, v := range mem.RouterInfo {
			sb.WriteString(fmt.Sprintf("%s=%v, ", k, v))
		}
		sb.WriteString("\n")
	}

	if len(mem.LastAudit) > 0 {
		hasContent = true
		score := mem.LastAudit["score"]
		status := mem.LastAudit["status"]
		date := mem.LastAudit["date"]
		sb.WriteString(fmt.Sprintf("- **آخر فحص أمني سابق**: النتيجة %v/100 (%v) بتاريخ %v\n", score, status, date))
		if findings, ok := mem.LastAudit["findings"].([]interface{}); ok && len(findings) > 0 {
			sb.WriteString("  * أهم الملاحظات السابقة: ")
			for i, f := range findings {
				if i >= 3 {
					break
				}
				if fm, ok := f.(map[string]interface{}); ok {
					sb.WriteString(fmt.Sprintf("[%v: %v] ", fm["title"], fm["severity"]))
				}
			}
			sb.WriteString("\n")
		}
	}

	if len(mem.AppliedCommands) > 0 {
		hasContent = true
		sb.WriteString("- **سجل آخر التعديلات والأوامر المنفذة على الراوتر**:\n")
		startIdx := 0
		if len(mem.AppliedCommands) > 5 {
			startIdx = len(mem.AppliedCommands) - 5
		}
		for _, ac := range mem.AppliedCommands[startIdx:] {
			sb.WriteString(fmt.Sprintf("  * `%s` (%s)\n", ac.Command, ac.Context))
		}
	}

	if len(mem.ConversationSummaries) > 0 {
		hasContent = true
		sb.WriteString("- **ملخصات الجلسات السابقة مع هذا الوكيل**:\n")
		startIdx := 0
		if len(mem.ConversationSummaries) > 4 {
			startIdx = len(mem.ConversationSummaries) - 4
		}
		for _, cs := range mem.ConversationSummaries[startIdx:] {
			sb.WriteString(fmt.Sprintf("  * [%s]: %s\n", cs.Topic, cs.Summary))
		}
	}

	if strings.TrimSpace(mem.Notes) != "" {
		hasContent = true
		sb.WriteString("- **ملاحظات فنية مسجلة**: " + mem.Notes + "\n")
	}

	if !hasContent {
		return ""
	}
	return sb.String()
}

// saveChatSessionMemory asynchronously records a compact session summary into agent memory
func (e *Engine) saveChatSessionMemory(subdomain string, messages []ChatMessage, aiReply string, plan *ChangePlan) {
	if subdomain == "" || strings.TrimSpace(aiReply) == "" {
		return
	}
	go func(sub string, userMsg string, reply string, p *ChangePlan) {
		topic := "استفسار ومساعدة عامة"
		if p != nil {
			topic = fmt.Sprintf("خطة تعديل: %s", p.Title)
		} else if strings.Contains(userMsg, "فحص") || strings.Contains(userMsg, "بطء") || strings.Contains(userMsg, "أمان") {
			topic = "تشخيص وصيانة الراوتر"
		} else if strings.Contains(userMsg, "أمر") || strings.Contains(userMsg, "تعديل") || strings.Contains(userMsg, "firewall") {
			topic = "إدارة إعدادات الراوتر"
		}

		summary := strings.TrimSpace(reply)
		// Strip markdown code blocks for a clean short summary
		if idx := strings.Index(summary, "```"); idx != -1 && idx > 50 {
			summary = summary[:idx]
		}
		summary = strings.ReplaceAll(summary, "\n", " ")
		if len(summary) > 200 {
			summary = summary[:197] + "..."
		}

		_ = e.repo.AppendConversationSummary(sub, summary, topic, 10)
	}(subdomain, extractLastUserMessage(messages), aiReply, plan)
}

func extractLastUserMessage(messages []ChatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}
