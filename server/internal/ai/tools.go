package ai

import "strings"

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
				Name:        "mikrotik_packet_simulator",
				Description: "محاكي مسار الباكت الافتراضي (Offline Packet Simulator): فحص واختبار عبور باكت افتراضي عبر جداول الـ NAT، والتوجيه، والفايروول دون لمس الراوتر لمعرفة هل سيمر أم يسقط (PASS or DROP) ورقم القاعدة المسببة",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
						"src_ip": map[string]interface{}{
							"type":        "string",
							"description": "عنوان IP المصدر للباكت (مثال: 192.168.1.100 أو 10.20.0.5)",
						},
						"dst_ip": map[string]interface{}{
							"type":        "string",
							"description": "عنوان IP الوجهة (مثال: 8.8.8.8 أو 1.1.1.1)",
						},
						"protocol": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"tcp", "udp", "icmp"},
							"description": "بروتوكول الباكت (افتراضياً tcp)",
						},
						"dst_port": map[string]interface{}{
							"type":        "string",
							"description": "منفذ الوجهة (مثال: 53 أو 80 أو 443)",
						},
						"in_interface": map[string]interface{}{
							"type":        "string",
							"description": "المنفذ الداخل (مثال: ether1 أو bridge-lan)",
						},
					},
					"required": []string{"subdomain", "src_ip", "dst_ip"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_explain_device",
				Description: "توليد وثيقة هندسية معمارية شاملة للراوتر (Explain Device) باللغة العربية تشرح هيكلة المنافذ، خطوط الـ WAN، وسلاسل الفايروول، والخدمات المكشوفة مع مخطط Mermaid",
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
				Name:        "mikrotik_active_defense",
				Description: "الرصد السيبراني النشط (Active Cyber Defense): تحليل محاولات التخمين وهجمات Brute-Force وتوليد خطة حظر آمنة ومؤقتة للمهاجمين في الفايروول",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
						"block_duration_minutes": map[string]interface{}{
							"type":        "integer",
							"description": "مدة الحظر المؤقت بالدقائق (افتراضياً 60 دقيقة)",
						},
					},
					"required": []string{"subdomain"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_setup_vpn",
				Description: "أتمتة سويت الـ VPN: توليد خطة إعداد سيرفر أو عميل VPN (WireGuard, IPsec, SSTP, L2TP, OpenVPN, EoIP) وتوليد ملف إعداد العميل للموبايل/الكمبيوتر",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subdomain": map[string]interface{}{
							"type":        "string",
							"description": "اسم نطاق الوكيل المستهدف",
						},
						"vpn_type": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"wireguard", "ipsec", "sstp", "l2tp", "openvpn", "eoip"},
							"description": "نوع بروتوكول الـ VPN المطلوب إعداده",
						},
						"client_name": map[string]interface{}{
							"type":        "string",
							"description": "اسم العميل أو الموقع المراد ربطه (مثال: phone_client أو site_b)",
						},
						"subnet": map[string]interface{}{
							"type":        "string",
							"description": "نطاق شبكة الـ VPN (مثال: 10.50.0.0/24)",
						},
					},
					"required": []string{"subdomain", "vpn_type"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "mikrotik_drift_guard",
				Description: "كاشف انحراف الإعدادات (Drift Guard): مقارنة إعدادات الراوتر الحالية مع الإعداد الأساسي المعتمد في الذاكرة وكشف أي تعديل يدوي أو غير مصرح به",
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
				Name:        "mikrotik_l2_rescue",
				Description: "مساعد الإنقاذ عبر الطبقة الثانية (L2 Rescue): فحص واكتشاف الأجهزة المجاورة (MNDP / CDP / LLDP) وتقديم إرشادات وأوامر الإنقاذ عبر MAC-Telnet/Winbox MAC عند فقدان الـ IP",
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
3. محاكاة مسار البيانات قبل التطبيق عبر أداة "mikrotik_packet_simulator".
4. توثيق وشرح معمارية الراوتر بلغة عربية احترافية عبر أداة "mikrotik_explain_device".
5. كشف الهجمات والحظر التلقائي المؤقت عبر أداة "mikrotik_active_defense".
6. كشف التغييرات والانحراف عن الإعدادات المعتمدة عبر أداة "mikrotik_drift_guard".
7. إنقاذ الراوترات المعطلة وفاقدة الـ IP عبر أداة "mikrotik_l2_rescue".

🛡️ قاعدة خط الحماية الإلزامي الصارم (Zero Direct Write Guardrail):
- يمنع منعاً باتاً تنفيذ أي أوامر تعديل أو كتابة على الراوتر (مثل: /add, /set, /remove, /enable, /disable) مباشرة عبر mikrotik_run_command!
- أي طلب تعديل أو إصلاح أو حظر هجمات أو إعداد VPN أو جدار حماية يجب أن يتم حصراً وبشكل إلزامي عن طريق استدعاء أداة "mikrotik_generate_plan" لتقديم خطة آمنة (Safe Mode Plan) توضح الأهداف ومستوى الخطورة وأوامر التراجع (Rollback)، ولا يتم التنفيذ الفعلي على الراوتر إلا بعد مراجعة المدير وموافقته بالضغط على زر التطبيق.

⚡ استدعاء الأدوات المباشر (Direct Tool Invocation):
- إذا أرسل المستخدم أو ذكر اسم أي أداة مباشرة في رسالته (مثل: mikrotik_discover_topology, mikrotik_packet_simulator, mikrotik_explain_device, mikrotik_active_defense, mikrotik_setup_vpn, mikrotik_drift_guard, mikrotik_l2_rescue, mikrotik_get_resources, mikrotik_get_firewall, mikrotik_attack_detection, mikrotik_run_command, mikrotik_generate_plan): قم باستدعاء هذه الأداة فوراً للراوتر المستهدف واجلب تفاصيلها الحية كاملة واعرضها بشكل منظم ومفصل.

⚡ قواعد السرعة الفائقة وتوفير التوكنات (High Speed & Efficiency Rules):
- عند طلب فحص أو تشخيص عام للراوتر، استدعِ الأدوات المطلوبة دفعة واحدة في الدورة الأولى بالتوازي (Parallel Tool Calls) لتنهي الإجابة في دورة واحدة دون إطالة أو تكرار الاستدعاءات عبر دورات متعددة.
- بمجرد استدعاء الأداة (خاصة الأدوات الشاملة مثل mikrotik_explain_device أو mikrotik_discover_topology أو mikrotik_active_defense): يمنع منعاً باتاً استدعاء أي أدوات فرعية بعدها، لأن هذه الأدوات تجلب تلقائياً كافة بيانات المنافذ، التوجيه، الموارد، ومخطط Mermaid. يجب الانتقال فوراً في الدورة التالية لكتابة التوثيق الهندسي الشامل والتحليل الفني باللغة العربية.

👥 قاعدة حاسمة لاستعلام المشتركين والمتصلين (PPPoE / Active Users):
- عند طلب معرفة عدد المشتركين أو المتصلين حالياً بالراوتر أو PPPoE / Broadband، استخدم حصراً الأمر "/ppp/active/print" ولا تستخدم إطلاقاً "/interface/pppoe-client/print" (لأن pppoe-client مخصص لخطوط استلام الإنترنت الخارجية وليس لمشتركي الراوتر).

- استخدم لغة عربية مهنية واضحة ومنظمة مع إبراز النتائج والنصائح بالأيقونات التعبيرية والجداول ومخططات Mermaid عند الحاجة.`

// CompactSystemPromptTemplate is an ultra-concise prompt used for standard operational queries to save prompt tokens
const CompactSystemPromptTemplate = `أنت المساعد الذكي المدمج لمنظومة SASMAN لإدارة راوترات MikroTik RouterOS.
دورك: تحليل الحالة، استدعاء الأدوات المتاحة بدقة، وتقديم إجابات باللغة العربية واضحة ومنظمة مع توليد خطط التعديل الآمنة (mikrotik_generate_plan) عند طلب إجراء أي تغيير أو إصلاح.
القواعد:
- استدعِ فقط الأدوات المحددة المطلوبة للإجابة على السؤال الحالي دفعة واحدة بالتوازي.
- يمنع منعاً باتاً تنفيذ أوامر التعديل (/add, /set, /remove) مباشرة؛ استخدم دائماً mikrotik_generate_plan.
- اعتمد على صيغ أوامر RouterOS الدقيقة.`

// SelectRelevantTools dynamically filters and ranks the full toolset down to at most 3 tools based on deterministic priority scoring
func SelectRelevantTools(userQuery string, allTools []ToolDefinition) []ToolDefinition {
	q := strings.ToLower(strings.TrimSpace(userQuery))
	if q == "" {
		if len(allTools) > 3 {
			return allTools[:3]
		}
		return allTools
	}

	toolMap := make(map[string]ToolDefinition)
	for _, t := range allTools {
		toolMap[t.Function.Name] = t
	}

	scores := make(map[string]int)

	// 1. Explicit tool mention (+1000)
	for _, t := range allTools {
		if strings.Contains(q, strings.ToLower(t.Function.Name)) {
			scores[t.Function.Name] += 1000
		}
	}

	// 2. Intent-based scoring
	// Architecture / Topology / Explain
	if strings.Contains(q, "explain") || strings.Contains(q, "معمار") || strings.Contains(q, "هيكل") || strings.Contains(q, "مخطط") || strings.Contains(q, "توثيق") || strings.Contains(q, "رسم") || strings.Contains(q, "mermaid") || strings.Contains(q, "topology") || strings.Contains(q, "توزيع") {
		scores["mikrotik_explain_device"] += 100
		scores["mikrotik_discover_topology"] += 80
	}

	// Resources / CPU / Memory / Uptime
	if strings.Contains(q, "cpu") || strings.Contains(q, "معالج") || strings.Contains(q, "رام") || strings.Contains(q, "ذاكرة") || strings.Contains(q, "حرارة") || strings.Contains(q, "uptime") || strings.Contains(q, "موارد") || strings.Contains(q, "ضغط") {
		scores["mikrotik_get_resources"] += 90
		scores["mikrotik_run_command"] += 30
	}

	// Firewall / Security / Attacks / Filter / NAT
	if strings.Contains(q, "firewall") || strings.Contains(q, "فايروول") || strings.Contains(q, "جدار") || strings.Contains(q, "حظر") || strings.Contains(q, "block") || strings.Contains(q, "attack") || strings.Contains(q, "هجوم") || strings.Contains(q, "تخمين") || strings.Contains(q, "brute") || strings.Contains(q, "ثغرات") || strings.Contains(q, "أمان") || strings.Contains(q, "امن") {
		scores["mikrotik_get_firewall"] += 90
		scores["mikrotik_active_defense"] += 85
		scores["mikrotik_attack_detection"] += 70
	}

	// Packet Simulator
	if strings.Contains(q, "باكت") || strings.Contains(q, "packet") || strings.Contains(q, "مسار") || strings.Contains(q, "يمر") || strings.Contains(q, "يسقط") || strings.Contains(q, "drop") {
		scores["mikrotik_packet_simulator"] += 100
	}

	// Interfaces / Ports / Traffic / PPPoE / WAN / LAN
	if strings.Contains(q, "interface") || strings.Contains(q, "منفذ") || strings.Contains(q, "منافذ") || strings.Contains(q, "واجهة") || strings.Contains(q, "واجهات") || strings.Contains(q, "بورت") || strings.Contains(q, "بورتات") || strings.Contains(q, "wan") || strings.Contains(q, "lan") || strings.Contains(q, "pppoe") || strings.Contains(q, "مشترك") || strings.Contains(q, "متصل") {
		scores["mikrotik_get_interfaces"] += 90
		scores["mikrotik_run_command"] += 30
	}

	// VPN
	if strings.Contains(q, "vpn") || strings.Contains(q, "wireguard") || strings.Contains(q, "sstp") || strings.Contains(q, "l2tp") || strings.Contains(q, "ipsec") || strings.Contains(q, "نفق") {
		scores["mikrotik_setup_vpn"] += 100
	}

	// Drift / Baseline
	if strings.Contains(q, "انحراف") || strings.Contains(q, "تغيير") || strings.Contains(q, "مقارنة") || strings.Contains(q, "drift") || strings.Contains(q, "baseline") {
		scores["mikrotik_drift_guard"] += 100
	}

	// L2 Rescue
	if strings.Contains(q, "rescue") || strings.Contains(q, "انقاذ") || strings.Contains(q, "إنقاذ") || strings.Contains(q, "mac-telnet") || strings.Contains(q, "فصل") || strings.Contains(q, "معطل") {
		scores["mikrotik_l2_rescue"] += 100
	}

	// Safe Plan generation
	if strings.Contains(q, "خطة") || strings.Contains(q, "صلح") || strings.Contains(q, "عدل") || strings.Contains(q, "غير") || strings.Contains(q, "احذف") || strings.Contains(q, "اضف") || strings.Contains(q, "طبق") || strings.Contains(q, "fix") || strings.Contains(q, "plan") {
		scores["mikrotik_generate_plan"] += 95
		scores["mikrotik_run_command"] += 40
	}

	// If no specific scores matched, fall back to core diagnostic tools
	if len(scores) == 0 {
		scores["mikrotik_get_resources"] = 50
		scores["mikrotik_get_interfaces"] = 40
		scores["mikrotik_run_command"] = 30
	}

	// Deterministic sorting by score descending, then by name
	type toolCandidate struct {
		name  string
		score int
	}
	var candidates []toolCandidate
	for name, score := range scores {
		if _, exists := toolMap[name]; exists && score > 0 {
			candidates = append(candidates, toolCandidate{name: name, score: score})
		}
	}

	// Sort deterministically (selection sort / simple bubble sort)
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score || (candidates[j].score == candidates[i].score && candidates[j].name < candidates[i].name) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Pick top 3 max
	maxTools := 3
	if len(candidates) < maxTools {
		maxTools = len(candidates)
	}

	var result []ToolDefinition
	for i := 0; i < maxTools; i++ {
		result = append(result, toolMap[candidates[i].name])
	}

	return result
}
