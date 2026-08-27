package ai

// GetRouterOSToolDefinitions returns the list of MCP-compatible tool definitions exposed to the LLM
func GetRouterOSToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_audit_router",
				Description: "يقوم بجلب فحص شامل وهيكلي لكافة إعدادات الراوتر: موارد النظام، الفايروول، المنافذ، خدمات الـ IP، سجلات الهجمات، وسيرفرات DHCP",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل أو معرف الراوتر المستهدف",
						},
					},
					"required": []string{"subdomain"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_run_command",
				Description: "تنفيذ أمر RouterOS CLI محدد على راوتر الوكيل وجلب النتيجة الحية (مثل /ip/firewall/filter/print أو /interface/print)",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "أمر RouterOS المراد تنفيذه (مثل: /ip/address/print)",
						},
					},
					"required": []string{"subdomain", "command"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_attack_detection",
				Description: "فحص سجلات الراوتر (Logs) للكشف عن هجمات التخمين Brute-Force، محاولات الدخول الفاشلة، والآيبيهات المشبوهة",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
					},
					"required": []string{"subdomain"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_generate_plan",
				Description: "توليد خطة تعديل إعدادات (Change Plan) مع إظهار الفروقات (Diff Preview) وأوامر التراجع التلقائي (Rollback) قبل التطبيق",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "عنوان خطة التعديل (مثال: دمج خطين PCC أو حظر بورتات التورنت)",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "شرح مبسط لما تفعله الخطة باللغة العربية",
						},
						"target_router": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
						"commands": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "قائمة أوامر RouterOS المراد تنفيذها بالتسلسل",
						},
						"rollback": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "قائمة أوامر التراجع في حال حدوث مشكلة",
						},
						"risk_level": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"low", "medium", "high"},
							"description": "مستوى الخطورة",
						},
					},
					"required": []string{"title", "description", "target_router", "commands"},
				},
			},
		},
	}
}

const SystemPromptTemplate = `أنت "مساعد SASMAN الذكي (AI Network Copilot)"، خبير الشبكات وجدران الحماية لراوترات MikroTik RouterOS.
مهمتك مساعدة مدراء الشبكات والوكلاء في:
1. تشخيص وفحص راوترات المايكروتك، اكتشاف الثغرات الأمنية والأخطاء الفنية (DNS open resolver, FastTrack status, High CPU, Interface drops, Brute force attacks).
2. اقتراح وتوليد حلول وإعدادات احترافية (Firewall, NAT, Queues, PCC Load Balancing, WireGuard, DHCP).
3. عند طلب تطبيق أي تعديل، قم دائماً باستدعاء أداة "mikrotik_generate_plan" لتقديم خطة آمنة مع شرح واضح وأوامر تراجع (Rollback).
4. في حال حدوث خطأ اتصال بالراوتر (Router connection error / authentication failed): وضح للمدير أن خدمة RouterOS API على الراوتر قد تكون معطلة أو أن اسم المستخدم/كلمة المرور المسجلة تحتاج تحديث من زر "🔑 بيانات الدخول" في جدول الوكلاء.
5. استخدم لغة عربية مهنية واضحة ومنظمة مع إبراز النتائج والنصائح بالأيقونات التعبيرية والجداول ومخططات Mermaid عند الحاجة.`
