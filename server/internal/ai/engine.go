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
}

func NewEngine(repo *storage.SQLiteRepository, tunnelSvc *tunnel.Service) *Engine {
	return &Engine{
		repo:      repo,
		tunnelSvc: tunnelSvc,
		llm:       NewLLMClient(),
	}
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

	reqBody, _ := json.Marshal(map[string]interface{}{
		"command": cmdParts,
	})

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

	reqBody, _ := json.Marshal(map[string]interface{}{
		"audit": true,
	})

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

	reqBody, _ := json.Marshal(map[string]interface{}{
		"commands": cmdList,
	})

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
	settings, err := e.repo.GetAISettings()
	if err != nil {
		return nil, nil, err
	}
	if !settings.Enabled {
		return nil, nil, fmt.Errorf("خدمة المساعد الذكي معطلة حالياً من الإعدادات")
	}

	systemPrompt := SystemPromptTemplate
	if settings.SystemPrompt != "" {
		systemPrompt = settings.SystemPrompt
	}
	if targetSubdomain != "" {
		systemPrompt += fmt.Sprintf("\nالوكيل والراوتر المستهدف حالياً: %s", targetSubdomain)
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

	// Execute Tool Calling loop (max 4 iterations)
	for iter := 0; iter < 4; iter++ {
		req := ChatCompletionRequest{
			Model:       settings.Model,
			Messages:    conversation,
			Tools:       tools,
			Temperature: settings.Temperature,
		}

		resp, err := e.llm.Complete(ctx, settings, req)
		if err != nil {
			return nil, nil, err
		}

		if len(resp.Choices) == 0 {
			return nil, nil, fmt.Errorf("لم يتم استلام رد من نموذج الذكاء الاصطناعي")
		}

		choice := resp.Choices[0]
		replyMsg := choice.Message

		// If no tool calls, this is the final message
		if len(replyMsg.ToolCalls) == 0 {
			return &replyMsg, finalPlan, nil
		}

		conversation = append(conversation, replyMsg)

		// Execute tool calls requested by the model
		for _, tc := range replyMsg.ToolCalls {
			fnName := tc.Function.Name
			var args map[string]interface{}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

			sub := targetSubdomain
			if s, ok := args["subdomain"].(string); ok && s != "" {
				sub = s
			}
			if s, ok := args["target_router"].(string); ok && s != "" {
				sub = s
			}

			var toolResult interface{}

			switch fnName {
			case "mikrotik_audit_router":
				audit, err := e.ExecuteSystemAudit(sub)
				if err != nil {
					toolResult = map[string]string{"error": err.Error()}
				} else {
					toolResult = audit
				}

			case "mikrotik_run_command":
				cmd, _ := args["command"].(string)
				res, err := e.ExecuteRouterCommand(sub, cmd)
				if err != nil {
					toolResult = map[string]string{"error": err.Error()}
				} else {
					toolResult = res
				}

			case "mikrotik_attack_detection":
				audit, err := e.ExecuteSystemAudit(sub)
				if err != nil {
					toolResult = map[string]string{"error": err.Error()}
				} else {
					var logs []interface{}
					if l, ok := audit["logs"].([]interface{}); ok {
						logs = l
					}
					toolResult = map[string]interface{}{
						"logs": logs,
					}
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

			default:
				toolResult = map[string]string{"error": "أداة غير معروفة"}
			}

			resultBytes, _ := json.Marshal(toolResult)
			conversation = append(conversation, ChatMessage{
				Role:       "tool",
				Name:       fnName,
				ToolCallID: tc.ID,
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

	log.Printf("[AI Copilot] Completed audit for %s: score=%d, findings=%d", subdomain, report.Score, len(report.Findings))
	return report, nil
}
