package wan

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"mikrotik-manager/pkg/core"

	"github.com/go-routeros/routeros/v3"
	"github.com/gofiber/fiber/v2"
)

type WanDiagnosis struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	AutoFix  bool   `json:"auto_fix"`
	Fixed    bool   `json:"fixed"`
	Detail   string `json:"detail,omitempty"`
}

type RouterSnapshot struct {
	Identity      []map[string]string `json:"identity,omitempty"`
	Resource      []map[string]string `json:"resource,omitempty"`
	Interfaces    []map[string]string `json:"interfaces,omitempty"`
	Addresses     []map[string]string `json:"addresses,omitempty"`
	IPSettings    []map[string]string `json:"ip_settings,omitempty"`
	Routes        []map[string]string `json:"routes,omitempty"`
	NatRules      []map[string]string `json:"nat_rules,omitempty"`
	MangleRules   []map[string]string `json:"mangle_rules,omitempty"`
	FilterRules   []map[string]string `json:"filter_rules,omitempty"`
	IPv6Filters   []map[string]string `json:"ipv6_filter_rules,omitempty"`
	DhcpClients   []map[string]string `json:"dhcp_clients,omitempty"`
	PppoeClients  []map[string]string `json:"pppoe_clients,omitempty"`
	InterfaceList []map[string]string `json:"interface_list,omitempty"`
	ListMembers   []map[string]string `json:"list_members,omitempty"`
	RoutingTables []map[string]string `json:"routing_tables,omitempty"`
	RoutingRules  []map[string]string `json:"routing_rules,omitempty"`
}

type WanPreflightReport struct {
	Status        string         `json:"status"`
	Mode          string         `json:"mode"`
	LanInterface  string         `json:"lan_interface"`
	Lines         []WanLine      `json:"lines"`
	LocalSubnets  []string       `json:"local_subnets"`
	Issues        []WanDiagnosis `json:"issues"`
	Snapshot      RouterSnapshot `json:"snapshot,omitempty"`
	FixedCount    int            `json:"fixed_count"`
	WarningCount  int            `json:"warning_count"`
	CriticalCount int            `json:"critical_count"`
}

type wanPreflightOptions struct {
	Mode            string
	LanInterface    string
	Lines           []WanLine
	AutoFix         bool
	IncludeSnapshot bool
}

func GetWanPreflight(c *fiber.Ctx) error {
	type Request struct {
		Mode            string    `json:"mode"`
		LanInterface    string    `json:"lan"`
		Lines           []WanLine `json:"lines"`
		AutoFix         bool      `json:"auto_fix"`
		IncludeSnapshot bool      `json:"include_snapshot"`
	}
	var req Request
	_ = c.BodyParser(&req)

	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	report, err := RunWanPreflight(client, wanPreflightOptions{
		Mode:            req.Mode,
		LanInterface:    req.LanInterface,
		Lines:           req.Lines,
		AutoFix:         req.AutoFix,
		IncludeSnapshot: req.IncludeSnapshot,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "report": report})
	}
	return c.JSON(report)
}

func RunWanPreflight(client *routeros.Client, opt wanPreflightOptions) (WanPreflightReport, error) {
	report := WanPreflightReport{
		Mode:         strings.TrimSpace(opt.Mode),
		LanInterface: strings.TrimSpace(opt.LanInterface),
		Lines:        normalizeWanLines(opt.Lines),
	}

	snapshot := collectRouterSnapshot(client)
	report.Snapshot = snapshot
	report.LanInterface = detectLanInterface(snapshot, report.LanInterface)
	report.LocalSubnets = detectLocalSubnets(snapshot)
	if len(report.LocalSubnets) == 0 {
		report.LocalSubnets = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	}
	if len(report.Lines) == 0 {
		report.Lines = discoverWanLines(snapshot)
	}

	report.Issues = analyzeWanReadiness(client, snapshot, report.Mode, report.Lines, report.LanInterface, report.LocalSubnets)
	if opt.AutoFix {
		fixedIssues := autoHealWanReadiness(client, report.Issues, report.Mode, report.Lines, report.LanInterface, report.LocalSubnets)
		snapshot = collectRouterSnapshot(client)
		report.Snapshot = snapshot
		finalIssues := analyzeWanReadiness(client, snapshot, report.Mode, report.Lines, report.LanInterface, report.LocalSubnets)
		report.Issues = append(fixedOnlyDiagnostics(fixedIssues), finalIssues...)
		report.Issues = dedupeDiagnostics(report.Issues)
	}

	if !opt.IncludeSnapshot {
		report.Snapshot = RouterSnapshot{}
	}
	report.Status = "ready"
	for _, issue := range report.Issues {
		if issue.Fixed {
			report.FixedCount++
		}
		switch issue.Severity {
		case "critical":
			report.CriticalCount++
			report.Status = "blocked"
		case "warning":
			report.WarningCount++
			if report.Status == "ready" {
				report.Status = "warning"
			}
		}
	}
	if report.CriticalCount > 0 {
		return report, fmt.Errorf("فحص WAN وجد مشاكل حرجة قبل الدمج")
	}
	return report, nil
}

func collectRouterSnapshot(client *routeros.Client) RouterSnapshot {
	return RouterSnapshot{
		Identity:      rosRows(client, "/system/identity/print"),
		Resource:      rosRows(client, "/system/resource/print"),
		Interfaces:    rosRows(client, "/interface/print"),
		Addresses:     rosRows(client, "/ip/address/print"),
		IPSettings:    rosRows(client, "/ip/settings/print"),
		Routes:        rosRows(client, "/ip/route/print"),
		NatRules:      rosRows(client, "/ip/firewall/nat/print"),
		MangleRules:   rosRows(client, "/ip/firewall/mangle/print"),
		FilterRules:   rosRows(client, "/ip/firewall/filter/print"),
		IPv6Filters:   rosRows(client, "/ipv6/firewall/filter/print"),
		DhcpClients:   rosRows(client, "/ip/dhcp-client/print"),
		PppoeClients:  rosRows(client, "/interface/pppoe-client/print"),
		InterfaceList: rosRows(client, "/interface/list/print"),
		ListMembers:   rosRows(client, "/interface/list/member/print"),
		RoutingTables: rosRows(client, "/routing/table/print"),
		RoutingRules:  rosRows(client, "/routing/rule/print"),
	}
}

func rosRows(client *routeros.Client, command string) []map[string]string {
	reply, err := core.SafeRun(client, command)
	if err != nil || reply == nil {
		return nil
	}
	rows := make([]map[string]string, 0, len(reply.Re))
	for _, re := range reply.Re {
		row := make(map[string]string, len(re.Map))
		for k, v := range re.Map {
			row[k] = v
		}
		rows = append(rows, row)
	}
	return rows
}

func normalizeWanLines(lines []WanLine) []WanLine {
	normalized := make([]WanLine, 0, len(lines))
	for i, line := range lines {
		line.Interface = strings.TrimSpace(line.Interface)
		line.Name = strings.TrimSpace(line.Name)
		line.Gateway = strings.TrimSpace(line.Gateway)
		line.Weight = normalizeWeight(line.Weight)
		if line.Interface == "" {
			continue
		}
		if line.Name == "" {
			line.Name = fmt.Sprintf("WAN%d", i+1)
		}
		normalized = append(normalized, line)
	}
	return normalized
}

func discoverWanLines(snapshot RouterSnapshot) []WanLine {
	found := map[string]WanLine{}
	for _, row := range snapshot.NatRules {
		comment := row["comment"]
		if strings.HasPrefix(comment, "TM_Masq_WAN") && row["out-interface"] != "" {
			name := strings.TrimPrefix(comment, "TM_Masq_")
			found[row["out-interface"]] = WanLine{Interface: row["out-interface"], Name: name, Weight: 1}
		}
	}
	for _, row := range snapshot.DhcpClients {
		if row["comment"] == "TM_WAN" && row["interface"] != "" {
			found[row["interface"]] = WanLine{Interface: row["interface"], Name: row["interface"], Gateway: row["gateway"], Weight: 1}
		}
	}
	for _, row := range snapshot.PppoeClients {
		if row["comment"] == "TM_WAN" && row["name"] != "" {
			found[row["name"]] = WanLine{Interface: row["name"], Name: row["name"], Weight: 1}
		}
	}
	lines := make([]WanLine, 0, len(found))
	for _, line := range found {
		lines = append(lines, line)
	}
	sort.Slice(lines, func(i, j int) bool {
		return extractNumber(lines[i].Name) < extractNumber(lines[j].Name)
	})
	return lines
}

func detectLanInterface(snapshot RouterSnapshot, requested string) string {
	if requested != "" {
		return requested
	}
	for _, member := range snapshot.ListMembers {
		if (member["list"] == "TM_LAN" || strings.EqualFold(member["list"], "LAN")) && member["interface"] != "" {
			return member["interface"]
		}
	}
	for _, row := range snapshot.Interfaces {
		if row["type"] == "bridge" && row["running"] == "true" && row["name"] != "" {
			return row["name"]
		}
	}
	for _, row := range snapshot.Interfaces {
		if row["type"] == "bridge" && row["name"] != "" {
			return row["name"]
		}
	}
	return "bridge"
}

func detectLocalSubnets(snapshot RouterSnapshot) []string {
	seen := map[string]bool{}
	for _, row := range snapshot.Addresses {
		addr := row["address"]
		if addr == "" || strings.Contains(strings.ToLower(row["interface"]), "wan") {
			continue
		}
		ip, network, err := net.ParseCIDR(addr)
		if err != nil || ip == nil || network == nil || !ip.IsPrivate() {
			continue
		}
		network.IP = ip.Mask(network.Mask)
		seen[network.String()] = true
	}
	subnets := make([]string, 0, len(seen))
	for subnet := range seen {
		subnets = append(subnets, subnet)
	}
	sort.Strings(subnets)
	return subnets
}

func analyzeWanReadiness(client *routeros.Client, snapshot RouterSnapshot, mode string, lines []WanLine, lan string, localSubnets []string) []WanDiagnosis {
	var issues []WanDiagnosis
	if lan == "" || !snapshotHasInterface(snapshot, lan) {
		issues = append(issues, WanDiagnosis{"warning", "LAN_UNKNOWN", "لم يتم العثور على واجهة LAN مؤكدة، سيتم استخدام bridge كخيار افتراضي", false, false, lan})
	}
	if hasEnabledFastTrack(snapshot) {
		issues = append(issues, WanDiagnosis{"warning", "FASTTRACK_ENABLED", "FastTrack مفعل وقد يمنع mangle/PCC من العمل بشكل صحيح", true, false, ""})
	}
	if ipSettingsNeedRpFilterFix(snapshot) {
		issues = append(issues, WanDiagnosis{"warning", "RP_FILTER_ENABLED", "rp-filter ليس no وقد يسبب إسقاط حزم في سيناريوهات multi-WAN", true, false, ""})
	}
	if isECMPPreflightMode(mode) && ipSettingsNeedEcmpL4HashFix(snapshot) {
		issues = append(issues, WanDiagnosis{"warning", "ECMP_HASH_POLICY_NOT_L4", "ECMP hash policy ليس L4، وهذا قد يقلل توزيع الاتصالات عندما تتشابه عناوين المصدر والوجهة", true, false, ""})
	}
	if hasEnabledIPv6FastTrack(snapshot) {
		issues = append(issues, WanDiagnosis{"warning", "IPV6_FASTTRACK_ENABLED", "IPv6 FastTrack مفعل، إذا كان العملاء يستخدمون IPv6 فقد يتجاوز سياسات الدمج الخاصة بـ IPv4", false, false, ""})
	}
	if len(localSubnets) == 0 {
		issues = append(issues, WanDiagnosis{"warning", "LOCAL_SUBNETS_EMPTY", "لم يتم اكتشاف شبكات LAN محلية لإضافتها إلى TM_Local_Subnets", true, false, ""})
	}
	if !listHasMember(snapshot, "TM_LAN", lan) {
		issues = append(issues, WanDiagnosis{"warning", "LAN_LIST_MISSING", "واجهة LAN غير مثبتة داخل TM_LAN", true, false, lan})
	}
	if isPCCPreflightMode(mode) {
		issues = append(issues, analyzeRoutingReadiness(snapshot, lines)...)
		issues = append(issues, analyzeMangleOrderReadiness(snapshot, lines)...)
	}
	for _, line := range lines {
		if !snapshotHasInterface(snapshot, line.Interface) {
			issues = append(issues, WanDiagnosis{"critical", "WAN_INTERFACE_MISSING", "واجهة WAN غير موجودة على الراوتر", false, false, line.Interface})
			continue
		}
		if !interfaceRunning(snapshot, line.Interface) {
			issues = append(issues, WanDiagnosis{"warning", "WAN_INTERFACE_DOWN", "واجهة WAN غير running حالياً وقد يتم استبعادها عملياً من الدمج", false, false, line.Interface})
		}
		if !listHasMember(snapshot, "WAN", line.Interface) {
			issues = append(issues, WanDiagnosis{"warning", "WAN_LIST_MISSING", "واجهة WAN غير مضافة إلى قائمة WAN", true, false, line.Interface})
		}
		if dhcp := findDhcpClient(snapshot, line.Interface); dhcp != nil && !dhcpClientIsRunning(dhcp, interfaceRunningMap(snapshot)) {
			issues = append(issues, WanDiagnosis{"warning", "DHCP_NOT_BOUND", "DHCP Client غير bound بعد، سيتم الانتظار قليلاً قبل الدمج", true, false, line.Interface})
		}
		if pppoe := findPPPoEClient(snapshot, line.Interface); pppoe != nil && !pppoeClientRunning(pppoe) {
			issues = append(issues, WanDiagnosis{"warning", "PPPOE_NOT_CONNECTED", "PPPoE غير متصل بعد، سيتم الانتظار قليلاً قبل الدمج", true, false, line.Interface})
		}
		if _, err := resolveWanGateway(client, line); err != nil {
			issues = append(issues, WanDiagnosis{"critical", "GATEWAY_UNRESOLVED", "لم يتمكن المحرك من تحديد gateway صالح لهذا الخط", false, false, line.Interface})
		}
	}
	return issues
}

func autoHealWanReadiness(client *routeros.Client, issues []WanDiagnosis, mode string, lines []WanLine, lan string, localSubnets []string) []WanDiagnosis {
	for idx := range issues {
		if !issues[idx].AutoFix {
			continue
		}
		switch issues[idx].Code {
		case "FASTTRACK_ENABLED":
			disableFastTrackRules(client)
			issues[idx].Fixed = true
		case "RP_FILTER_ENABLED":
			core.SafeRun(client, "/ip/settings/set", "=rp-filter=no")
			issues[idx].Fixed = true
		case "ECMP_HASH_POLICY_NOT_L4":
			core.SafeRun(client, "/ip/settings/set", "=ipv4-multipath-hash-policy=l4")
			issues[idx].Fixed = true
		case "LAN_LIST_MISSING":
			core.SafeRun(client, "/interface/list/add", "=name=TM_LAN", "=comment=TM_LAN")
			if lan != "" {
				core.SafeRun(client, "/interface/list/member/add", "=list=TM_LAN", "=interface="+lan, "=comment=TM_LAN")
			}
			issues[idx].Fixed = true
		case "WAN_LIST_MISSING":
			core.SafeRun(client, "/interface/list/add", "=name=WAN")
			if issues[idx].Detail != "" {
				core.SafeRun(client, "/interface/list/member/add", "=interface="+issues[idx].Detail, "=list=WAN", "=comment=TM_WAN")
			}
			issues[idx].Fixed = true
		case "DHCP_NOT_BOUND", "PPPOE_NOT_CONNECTED":
			waitForWanLineReady(client, issues[idx].Detail, 6*time.Second)
			issues[idx].Fixed = true
		case "LOCAL_SUBNETS_EMPTY":
			issues[idx].Fixed = true
		case "ROUTING_TABLE_MISSING":
			if issues[idx].Detail != "" {
				core.SafeRun(client, "/routing/table/add", "=name="+issues[idx].Detail, "=fib", "=comment=TM_Preflight")
				issues[idx].Fixed = true
			}
		}
	}
	core.SafeRun(client, "/ip/settings/set", "=rp-filter=no")
	if isECMPPreflightMode(mode) {
		core.SafeRun(client, "/ip/settings/set", "=ipv4-multipath-hash-policy=l4")
	}
	core.SafeRun(client, "/ip/dns/set", "=allow-remote-requests=yes")
	if lan != "" {
		core.SafeRun(client, "/interface/list/add", "=name=TM_LAN", "=comment=TM_LAN")
		core.SafeRun(client, "/interface/list/member/add", "=list=TM_LAN", "=interface="+lan, "=comment=TM_LAN")
	}
	for _, line := range lines {
		core.SafeRun(client, "/interface/list/add", "=name=WAN")
		core.SafeRun(client, "/interface/list/member/add", "=interface="+line.Interface, "=list=WAN", "=comment=TM_WAN")
	}
	for _, subnet := range localSubnets {
		core.SafeRun(client, "/ip/firewall/address-list/add", "=list=TM_Local_Subnets", "=address="+subnet, "=comment=TM_Bypass")
	}
	return issues
}

func dedupeDiagnostics(items []WanDiagnosis) []WanDiagnosis {
	seen := map[string]bool{}
	out := make([]WanDiagnosis, 0, len(items))
	for _, item := range items {
		key := item.Code + "|" + item.Detail + "|" + item.Severity
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func fixedOnlyDiagnostics(items []WanDiagnosis) []WanDiagnosis {
	out := make([]WanDiagnosis, 0, len(items))
	for _, item := range items {
		if item.Fixed {
			out = append(out, item)
		}
	}
	return out
}

func isPCCPreflightMode(mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	return mode == "" || mode == "pcc" || mode == "advanced" || mode == "pcc-optimize" || mode == "load-balance"
}

func isECMPPreflightMode(mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	return mode == "default-route" || mode == "normal" || mode == "ecmp" || mode == "default-route-ecmp"
}

func analyzeRoutingReadiness(snapshot RouterSnapshot, lines []WanLine) []WanDiagnosis {
	var issues []WanDiagnosis
	if len(lines) <= 1 {
		return issues
	}

	expectedTables := map[string]bool{}
	for i := range lines {
		expectedTables[fmt.Sprintf("to_WAN%d", i+1)] = true
	}

	for tableName := range expectedTables {
		if !routingTableExists(snapshot, tableName) {
			issues = append(issues, WanDiagnosis{
				Severity: "warning",
				Code:     "ROUTING_TABLE_MISSING",
				Message:  "جدول routing الخاص بأحد خطوط PCC غير موجود قبل التطبيق",
				AutoFix:  true,
				Detail:   tableName,
			})
			continue
		}
		if !routingTableFibEnabled(snapshot, tableName) {
			issues = append(issues, WanDiagnosis{
				Severity: "warning",
				Code:     "ROUTING_TABLE_NO_FIB",
				Message:  "جدول routing موجود لكن fib غير مفعل، وهذا قد يمنع توجيه PCC على RouterOS v7",
				AutoFix:  false,
				Detail:   tableName,
			})
		}
		if !routingTableHasDefaultRoute(snapshot, tableName) {
			issues = append(issues, WanDiagnosis{
				Severity: "warning",
				Code:     "ROUTING_TABLE_NO_DEFAULT",
				Message:  "جدول routing لا يحتوي default route حالياً، سيتم إنشاؤه أثناء تطبيق الدمج",
				AutoFix:  false,
				Detail:   tableName,
			})
		}
	}

	for _, table := range snapshot.RoutingTables {
		name := table["name"]
		if strings.HasPrefix(name, "to_WAN") && !expectedTables[name] && table["disabled"] != "true" {
			issues = append(issues, WanDiagnosis{
				Severity: "warning",
				Code:     "STALE_ROUTING_TABLE",
				Message:  "يوجد جدول routing قديم من SASMAN لا يقابله خط WAN حالي",
				AutoFix:  false,
				Detail:   name,
			})
		}
	}

	for _, route := range snapshot.Routes {
		table := routeRoutingTable(route)
		if strings.HasPrefix(table, "to_WAN") && route["dst-address"] == "0.0.0.0/0" && gatewayNeedsExplicitLookup(route["gateway"]) {
			issues = append(issues, WanDiagnosis{
				Severity: "warning",
				Code:     "ROUTING_GATEWAY_LOOKUP_UNSCOPED",
				Message:  "default route داخل جدول PCC يستخدم gateway بدون @main أو %interface، وهذا قد يسبب lookup غامض في RouterOS v7",
				AutoFix:  false,
				Detail:   table + " gateway=" + route["gateway"],
			})
		}
		if strings.HasPrefix(table, "to_WAN") && !expectedTables[table] && route["disabled"] != "true" {
			issues = append(issues, WanDiagnosis{
				Severity: "warning",
				Code:     "STALE_ROUTING_ROUTE",
				Message:  "يوجد route قديم داخل جدول SASMAN لا يقابله خط WAN حالي",
				AutoFix:  false,
				Detail:   table + " " + route["dst-address"],
			})
		}
	}

	for _, rule := range snapshot.RoutingRules {
		if rule["disabled"] == "true" {
			continue
		}
		table := firstNonEmpty(rule["table"], rule["routing-table"])
		if strings.HasPrefix(table, "to_WAN") && !expectedTables[table] {
			issues = append(issues, WanDiagnosis{
				Severity: "warning",
				Code:     "STALE_ROUTING_RULE",
				Message:  "توجد routing rule قديمة تشير إلى جدول SASMAN غير مستخدم حالياً",
				AutoFix:  false,
				Detail:   table,
			})
			continue
		}
		if isBroadRoutingRule(rule) && !isManagedRoutingRule(rule) {
			issues = append(issues, WanDiagnosis{
				Severity: "warning",
				Code:     "BROAD_USER_ROUTING_RULE",
				Message:  "توجد routing rule عامة من المستخدم قد تتجاوز PCC أو تغير مسار بعض العملاء",
				AutoFix:  false,
				Detail:   describeRoutingRule(rule),
			})
		}
	}

	return issues
}

func analyzeMangleOrderReadiness(snapshot RouterSnapshot, lines []WanLine) []WanDiagnosis {
	var issues []WanDiagnosis
	if len(lines) <= 1 {
		return issues
	}
	bypassLocal := mangleRuleIndex(snapshot, func(rule map[string]string) bool {
		return rule["comment"] == "TM_Bypass_Local"
	})
	localToLocal := mangleRuleIndex(snapshot, func(rule map[string]string) bool {
		return rule["comment"] == "TM_Local_to_Local_Bypass"
	})
	firstPCC := mangleRuleIndex(snapshot, func(rule map[string]string) bool {
		return strings.HasPrefix(rule["comment"], "TM_PCC_PRE_") || strings.HasPrefix(rule["comment"], "TM_PCC_WAN")
	})
	firstRouteMark := mangleRuleIndex(snapshot, func(rule map[string]string) bool {
		return strings.HasPrefix(rule["comment"], "TM_Route_LAN_") || (rule["chain"] == "prerouting" && rule["action"] == "mark-routing")
	})

	if firstPCC >= 0 && bypassLocal < 0 {
		issues = append(issues, WanDiagnosis{"warning", "MANGLE_BYPASS_LOCAL_MISSING", "قاعدة bypass للوجهات المحلية غير موجودة قبل PCC", false, false, "TM_Bypass_Local"})
	}
	if firstPCC >= 0 && localToLocal < 0 {
		issues = append(issues, WanDiagnosis{"warning", "MANGLE_LOCAL_TO_LOCAL_MISSING", "قاعدة LAN-to-LAN bypass غير موجودة قبل PCC", false, false, "TM_Local_to_Local_Bypass"})
	}
	if firstPCC >= 0 && bypassLocal > firstPCC {
		issues = append(issues, WanDiagnosis{"warning", "MANGLE_BYPASS_AFTER_PCC", "قاعدة bypass المحلية تأتي بعد PCC، وهذا قد يسبب تعليم traffic محلي بالغلط", false, false, "TM_Bypass_Local"})
	}
	if firstPCC >= 0 && localToLocal > firstPCC {
		issues = append(issues, WanDiagnosis{"warning", "MANGLE_LOCAL_BYPASS_AFTER_PCC", "قاعدة LAN-to-LAN bypass تأتي بعد PCC", false, false, "TM_Local_to_Local_Bypass"})
	}
	if firstPCC >= 0 && firstRouteMark >= 0 && firstRouteMark < firstPCC {
		issues = append(issues, WanDiagnosis{"warning", "MANGLE_ROUTE_MARK_BEFORE_PCC", "قواعد mark-routing تظهر قبل PCC، وهذا قد يمنع توزيع الاتصالات بشكل صحيح", false, false, ""})
	}
	return issues
}

func routingTableExists(snapshot RouterSnapshot, name string) bool {
	for _, table := range snapshot.RoutingTables {
		if table["name"] == name && table["disabled"] != "true" {
			return true
		}
	}
	return false
}

func routingTableFibEnabled(snapshot RouterSnapshot, name string) bool {
	for _, table := range snapshot.RoutingTables {
		if table["name"] == name {
			return table["fib"] == "true" || table["fib"] == "yes"
		}
	}
	return false
}

func routingTableHasDefaultRoute(snapshot RouterSnapshot, table string) bool {
	for _, route := range snapshot.Routes {
		if route["disabled"] == "true" {
			continue
		}
		if route["dst-address"] == "0.0.0.0/0" && routeRoutingTable(route) == table {
			return true
		}
	}
	return false
}

func gatewayNeedsExplicitLookup(gateway string) bool {
	gateway = strings.TrimSpace(gateway)
	if gateway == "" || strings.Contains(gateway, "@") || strings.Contains(gateway, "%") || isInterfaceGateway(gateway) {
		return false
	}
	return net.ParseIP(gateway) != nil
}

func routeRoutingTable(route map[string]string) string {
	table := firstNonEmpty(route["routing-table"], route["vrf-interface"])
	if table == "" {
		return "main"
	}
	return table
}

func isBroadRoutingRule(rule map[string]string) bool {
	action := strings.ToLower(firstNonEmpty(rule["action"], "lookup"))
	if action != "lookup" && action != "lookup-only-in-table" {
		return false
	}
	if rule["src-address"] != "" || rule["dst-address"] != "" || rule["interface"] != "" || rule["in-interface"] != "" {
		return true
	}
	table := firstNonEmpty(rule["table"], rule["routing-table"])
	return table != "" && table != "main"
}

func isManagedRoutingRule(rule map[string]string) bool {
	comment := rule["comment"]
	return strings.Contains(comment, "TM_") || strings.Contains(comment, "SASMAN") || strings.HasPrefix(comment, "Route-")
}

func describeRoutingRule(rule map[string]string) string {
	parts := []string{}
	for _, key := range []string{"src-address", "dst-address", "interface", "in-interface", "action", "table", "routing-table", "comment"} {
		if rule[key] != "" {
			parts = append(parts, key+"="+rule[key])
		}
	}
	return strings.Join(parts, " ")
}

func mangleRuleIndex(snapshot RouterSnapshot, match func(map[string]string) bool) int {
	for idx, rule := range snapshot.MangleRules {
		if rule["disabled"] == "true" {
			continue
		}
		if match(rule) {
			return idx
		}
	}
	return -1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func snapshotHasInterface(snapshot RouterSnapshot, name string) bool {
	for _, row := range snapshot.Interfaces {
		if row["name"] == name {
			return true
		}
	}
	return false
}

func interfaceRunning(snapshot RouterSnapshot, name string) bool {
	for _, row := range snapshot.Interfaces {
		if row["name"] == name {
			return row["running"] == "true" && row["disabled"] != "true"
		}
	}
	return false
}

func interfaceRunningMap(snapshot RouterSnapshot) map[string]bool {
	m := map[string]bool{}
	for _, row := range snapshot.Interfaces {
		m[row["name"]] = row["running"] == "true" && row["disabled"] != "true"
	}
	return m
}

func hasEnabledFastTrack(snapshot RouterSnapshot) bool {
	for _, row := range snapshot.FilterRules {
		if row["action"] == "fasttrack-connection" && row["disabled"] != "true" {
			return true
		}
	}
	return false
}

func hasEnabledIPv6FastTrack(snapshot RouterSnapshot) bool {
	for _, row := range snapshot.IPv6Filters {
		if row["action"] == "fasttrack-connection" && row["disabled"] != "true" {
			return true
		}
	}
	return false
}

func ipSettingsNeedRpFilterFix(snapshot RouterSnapshot) bool {
	for _, row := range snapshot.IPSettings {
		rpFilter := strings.ToLower(strings.TrimSpace(row["rp-filter"]))
		return rpFilter != "" && rpFilter != "no"
	}
	return false
}

func ipSettingsNeedEcmpL4HashFix(snapshot RouterSnapshot) bool {
	for _, row := range snapshot.IPSettings {
		policy := strings.ToLower(strings.TrimSpace(row["ipv4-multipath-hash-policy"]))
		return policy != "" && policy != "l4"
	}
	return false
}

func listHasMember(snapshot RouterSnapshot, list string, iface string) bool {
	for _, row := range snapshot.ListMembers {
		if row["list"] == list && row["interface"] == iface && row["disabled"] != "true" {
			return true
		}
	}
	return false
}

func findDhcpClient(snapshot RouterSnapshot, iface string) map[string]string {
	for _, row := range snapshot.DhcpClients {
		if row["interface"] == iface {
			return row
		}
	}
	return nil
}

func findPPPoEClient(snapshot RouterSnapshot, name string) map[string]string {
	for _, row := range snapshot.PppoeClients {
		if row["name"] == name {
			return row
		}
	}
	return nil
}

func pppoeClientRunning(row map[string]string) bool {
	return row["disabled"] != "true" && (row["running"] == "true" || row["connected"] == "true" || strings.EqualFold(row["status"], "connected"))
}

func waitForWanLineReady(client *routeros.Client, iface string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ifaceReply, _ := core.SafeRun(client, "/interface/print", "?name="+iface)
		if ifaceReply != nil && len(ifaceReply.Re) > 0 && ifaceReply.Re[0].Map["running"] == "true" {
			return true
		}
		if dhcpReply, _ := core.SafeRun(client, "/ip/dhcp-client/print", "?interface="+iface); dhcpReply != nil && len(dhcpReply.Re) > 0 {
			if dhcpClientIsRunning(dhcpReply.Re[0].Map, map[string]bool{iface: true}) {
				return true
			}
		}
		if pppReply, _ := core.SafeRun(client, "/interface/pppoe-client/print", "?name="+iface); pppReply != nil && len(pppReply.Re) > 0 {
			if pppoeClientRunning(pppReply.Re[0].Map) {
				return true
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
