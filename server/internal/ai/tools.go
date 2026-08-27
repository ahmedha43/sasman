package ai

// GetRouterOSToolDefinitions returns the list of MCP-compatible tool definitions exposed to the LLM
func GetRouterOSToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_discover_topology",
				Description: "استكشاف وتحديث الهيكلة الكاملة للشبكة: خطوط الإنترنت ومداخل الـ WAN (مثل ستارلنك، إيرثلنك، 4G)، شبكات ومنافذ الـ LAN/Bridges، سياسات التوجيه (Policy Routing & Mangle مثل توجيه الواتساب أو الألعاب)، وحفظها في ذاكرة الراوتر الدائمة",
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
				Name:        "mikrotik_run_command",
				Description: "تنفيذ أي أمر RouterOS CLI محدد على راوتر الوكيل وجلب النتيجة الحية (مثل /ip/firewall/filter/print أو /interface/print أو /ip/dns/print)",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "أمر RouterOS CLI المراد تنفيذه (مثل: /ip/firewall/filter/print where action=drop)",
						},
					},
					"required": []string{"subdomain", "command"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_get_resources",
				Description: "فحص موارد الراوتر الأساسية: نسبة استهلاك المعالج CPU، الذاكرة المتبقية RAM، مدة التشغيل Uptime، اسم الموديل، وإصدار RouterOS",
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
				Name:        "mikrotik_get_firewall",
				Description: "جلب قواعد جدار الحماية (Firewall Filter / NAT / Mangle / Address-Lists) مع إمكانية التحديد",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
						"section": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"filter", "nat", "mangle", "address_list", "all"},
							"description": "قسم الفايروول المطلوب (افتراضياً filter)",
						},
						"chain": map[string]interface{}{
							"type":        "string",
							"description": "تصفية حسب سلسلة محددة (مثل: input أو forward أو dstnat)",
						},
					},
					"required": []string{"subdomain"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_get_interfaces",
				Description: "جلب قائمة المنافذ وواجهات الشبكة (Interfaces & IP Addresses) وحالتها الحالية والترافيك",
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
				Name:        "mikrotik_attack_detection",
				Description: "فحص سجلات الراوتر (Logs) وتحليل الهجمات: محاولات التخمين Brute-Force، الدخول الفاشل، وعناوين IP المهاجمة",
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
				Name:        "mikrotik_mcp_call",
				Description: "استدعاء مباشر لأي من أدوات محرك MikroTik MCP الـ 885 في الحاوية الجانبية (مثل: explain_device, simulator, vpn_wizard)",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
						"tool_name": map[string]interface{}{
							"type":        "string",
							"description": "اسم أداة الـ MCP المراد استدعاؤها (مثل: firewall_list_rules أو explain_device)",
						},
						"arguments": map[string]interface{}{
							"type":        "object",
							"description": "مدخلات ومعاملات الأداة",
						},
					},
					"required": []string{"subdomain", "tool_name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_generate_plan",
				Description: "توليد خطة تعديل إعدادات آمنة (Safe Mode Change Plan) مع إظهار الفروقات (Diff Preview) وأوامر التراجع التلقائي (Rollback) قبل التطبيق",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "عنوان خطة التعديل (مثال: حظر هجمات التخمين أو تفعيل FastTrack)",
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
2. استكشاف وفهم هيكلة الشبكة وتوزيع الخطوط والمنافذ عبر أداة "mikrotik_discover_topology".
3. اقتراح وتوليد حلول وإعدادات احترافية (Firewall, NAT, Queues, PCC Load Balancing, WireGuard, DHCP).
4. عند طلب تطبيق أي تعديل، قم دائماً باستدعاء أداة "mikrotik_generate_plan" لتقديم خطة آمنة مع شرح واضح وأوامر تراجع (Rollback).
5. في حال حدوث خطأ اتصال بالراوتر (Router connection error / authentication failed): وضح للمدير أن خدمة RouterOS API على الراوتر قد تكون معطلة أو أن اسم المستخدم/كلمة المرور المسجلة تحتاج تحديث من زر "🔑 بيانات الدخول" في جدول الوكلاء.

⚡ قواعد السرعة الفائقة وتوفير التوكنات (High Speed & Efficiency Rules):
- عند طلب فحص أو تشخيص عام للراوتر، استدعِ الأدوات المطلوبة دفعة واحدة في الدورة الأولى بالتوازي (Parallel Tool Calls مثل mikrotik_discover_topology أو mikrotik_get_resources) لتنهي الإجابة في دورة واحدة دون إطالة أو تكرار الاستدعاءات عبر دورات متعددة.
- قدم ردك النهائي والتحليل الفني فور استلام مخرجات الأدوات ولا تقم بإجراء دورات استدعاء فرعية لا حاجة لها.
- استخدم لغة عربية مهنية واضحة ومنظمة مع إبراز النتائج والنصائح بالأيقونات التعبيرية والجداول عند الحاجة.`
