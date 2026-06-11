package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type vpnConfigValidationCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type vpnConfigValidationItem struct {
	Kind   string                     `json:"kind"`
	Label  string                     `json:"label"`
	OK     bool                       `json:"ok"`
	Status string                     `json:"status"`
	Checks []vpnConfigValidationCheck `json:"checks"`
}

type vpnConfigValidationResponse struct {
	OK    bool                      `json:"ok"`
	Items []vpnConfigValidationItem `json:"items"`
}

func validationKindsForQuery(kind string) []string {
	switch strings.TrimSpace(kind) {
	case "clash":
		return []string{"clash-android", "clash"}
	case "clash-android", "clash-desktop", "vless", "sing-box", "rocket":
		if kind == "clash-desktop" {
			return []string{"clash"}
		}
		return []string{strings.TrimSpace(kind)}
	default:
		return []string{"clash-android", "clash"}
	}
}

func buildConfigValidation(user vpnUser, rt vpnRuntime, kinds []string) vpnConfigValidationResponse {
	items := make([]vpnConfigValidationItem, 0, len(kinds))
	allOK := true
	for _, kind := range kinds {
		item := validateGeneratedConfig(user, rt, kind)
		items = append(items, item)
		if !item.OK {
			allOK = false
		}
	}
	return vpnConfigValidationResponse{OK: allOK, Items: items}
}

func validateGeneratedConfig(user vpnUser, rt vpnRuntime, kind string) vpnConfigValidationItem {
	body := configBodyForKind(user, rt, kind)
	return validateConfigContent(user, rt, kind, body, "")
}

func configBodyForKind(user vpnUser, rt vpnRuntime, kind string) string {
	switch kind {
	case "clash":
		return clashConfig(user, rt)
	case "clash-android":
		return clashAndroidConfig(user, rt)
	case "vless":
		return vlessURL(user, rt)
	case "sing-box":
		return singBoxClientConfig(user, rt)
	default:
		return ""
	}
}

func validateConfigContent(user vpnUser, rt vpnRuntime, kind, body, source string) vpnConfigValidationItem {
	label := vpnConfigKindLabels[kind]
	if label == "" {
		label = kind
	}
	checks := make([]vpnConfigValidationCheck, 0, 12)
	checks = append(checks, validateRuntime(rt)...)
	checks = append(checks, validateUserState(user)...)
	if strings.TrimSpace(body) == "" {
		checks = append(checks, failCheck("内容", "配置内容为空"))
		return finalizeValidation(kind, label, checks)
	}
	switch kind {
	case "clash", "clash-android":
		checks = append(checks, validateClashYAML(body, user, rt, kind)...)
	case "vless":
		checks = append(checks, validateVLESSLink(body, user, rt)...)
	case "sing-box":
		checks = append(checks, validateSingBoxJSON(body, user, rt)...)
	default:
		checks = append(checks, failCheck("类型", "暂不支持该配置类型的检测"))
	}
	if source != "" {
		checks = append(checks, validateEmbeddedMeta(body, user, rt, kind)...)
	}
	return finalizeValidation(kind, label, checks)
}

func finalizeValidation(kind, label string, checks []vpnConfigValidationCheck) vpnConfigValidationItem {
	ok := true
	for _, check := range checks {
		if !check.OK {
			ok = false
			break
		}
	}
	status := "可用"
	if !ok {
		status = "不可用"
	}
	return vpnConfigValidationItem{
		Kind:   kind,
		Label:  label,
		OK:     ok,
		Status: status,
		Checks: checks,
	}
}

func validateRuntime(rt vpnRuntime) []vpnConfigValidationCheck {
	if err := validateVPNRuntime(rt); err != nil {
		return []vpnConfigValidationCheck{failCheck("服务端环境", err.Error())}
	}
	return []vpnConfigValidationCheck{passCheck("服务端环境", "VPN 运行参数已配置")}
}

func validateUserState(user vpnUser) []vpnConfigValidationCheck {
	if !user.Enabled {
		return []vpnConfigValidationCheck{failCheck("账号状态", "用户已停用，导入后也无法连接")}
	}
	return []vpnConfigValidationCheck{passCheck("账号状态", "用户已启用")}
}

func validateEmbeddedMeta(body string, user vpnUser, rt vpnRuntime, kind string) []vpnConfigValidationCheck {
	version, checksum := parseConfigMeta(body)
	if version == "" && checksum == "" {
		return []vpnConfigValidationCheck{warnCheck("版本标记", "未找到版本/校验和注释，无法确认是否为最新配置")}
	}
	raw := stripConfigMetaLines(body)
	expectedChecksum := vpnConfigChecksum(raw)
	checks := make([]vpnConfigValidationCheck, 0, 2)
	if checksum != "" {
		if strings.EqualFold(checksum, expectedChecksum) {
			checks = append(checks, passCheck("校验和", "与当前服务端生成的配置一致"))
		} else {
			checks = append(checks, failCheck("校验和", fmt.Sprintf("不一致（文件 %s，当前应为 %s）", checksum, expectedChecksum)))
		}
	}
	if version != "" {
		expectedVersion := vpnConfigVersion(user, rt, kind)
		if version == expectedVersion {
			checks = append(checks, passCheck("版本号", "与当前服务端一致"))
		} else {
			checks = append(checks, failCheck("版本号", fmt.Sprintf("已过期（文件 %s，当前应为 %s）", version, expectedVersion)))
		}
	}
	return checks
}

func validateClashYAML(body string, user vpnUser, rt vpnRuntime, kind string) []vpnConfigValidationCheck {
	raw := stripConfigMetaLines(body)
	checks := make([]vpnConfigValidationCheck, 0, 16)

	mode := yamlTopLevelValue(raw, "mode")
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "rule":
		checks = append(checks, passCheck("路由模式", "规则模式（国内直连，国外走代理）"))
	case "global":
		checks = append(checks, failCheck("路由模式", "当前为全局模式，国内网站/App 可能无法访问"))
	case "":
		checks = append(checks, failCheck("路由模式", "缺少 mode 字段"))
	default:
		checks = append(checks, warnCheck("路由模式", "当前为 "+mode+"，建议使用 rule"))
	}

	if strings.Contains(raw, "GEOIP,CN,DIRECT") {
		checks = append(checks, passCheck("分流规则", "包含 GEOIP,CN,DIRECT"))
	} else {
		checks = append(checks, failCheck("分流规则", "缺少 GEOIP,CN,DIRECT，国内流量可能误走代理"))
	}
	if strings.Contains(raw, "GEOSITE,cn,DIRECT") {
		checks = append(checks, passCheck("国内域名分流", "包含 GEOSITE,cn,DIRECT"))
	} else if kind == "clash-android" {
		checks = append(checks, warnCheck("国内域名分流", "建议包含 GEOSITE,cn,DIRECT（Android 需 geox 数据库）"))
	}
	if strings.Contains(raw, "GEOSITE,gfw,PROXY") {
		checks = append(checks, passCheck("国外域名分流", "包含 GEOSITE,gfw,PROXY"))
	} else if kind == "clash-android" {
		checks = append(checks, warnCheck("国外域名分流", "建议包含 GEOSITE,gfw,PROXY"))
	}
	if strings.Contains(raw, "geox-url:") {
		checks = append(checks, passCheck("Geo 数据库", "已配置 geox-url，CMFA 可下载 GEOIP/GEOSITE"))
	} else if kind == "clash-android" {
		checks = append(checks, warnCheck("Geo 数据库", "未配置 geox-url，GEOIP/GEOSITE 规则可能不生效"))
	}
	if strings.Contains(raw, "enhanced-mode: redir-host") {
		checks = append(checks, passCheck("DNS 模式", "redir-host（Android 兼容性更好）"))
	} else if strings.Contains(raw, "enhanced-mode: fake-ip") {
		checks = append(checks, warnCheck("DNS 模式", "fake-ip 在部分 Android App 上可能异常"))
	}

	proxyBlock := extractClashProxyBlock(raw, user.Name)
	if proxyBlock == "" {
		checks = append(checks, failCheck("节点", fmt.Sprintf("未找到名为 %q 的代理节点", user.Name)))
		return checks
	}
	checks = append(checks, passCheck("节点", fmt.Sprintf("找到节点 %q", user.Name)))

	if typ := yamlScalarInBlock(proxyBlock, "type"); typ != "vless" {
		checks = append(checks, failCheck("协议", fmt.Sprintf("节点类型应为 vless，当前为 %q", typ)))
	} else {
		checks = append(checks, passCheck("协议", "VLESS"))
	}

	uuid := strings.ToLower(yamlScalarInBlock(proxyBlock, "uuid"))
	if uuid != strings.ToLower(user.UUID) {
		checks = append(checks, failCheck("UUID", fmt.Sprintf("应为 %s，当前为 %s", user.UUID, uuid)))
	} else {
		checks = append(checks, passCheck("UUID", "与当前账号一致"))
	}

	server := yamlScalarInBlock(proxyBlock, "server")
	switch {
	case server == "":
		checks = append(checks, failCheck("服务器", "缺少 server 字段"))
	case server == "127.0.0.1" || server == "localhost":
		checks = append(checks, failCheck("服务器", "server 为本地地址，配置无效"))
	case server != rt.ServerHost:
		checks = append(checks, failCheck("服务器", fmt.Sprintf("应为 %s，当前为 %s", rt.ServerHost, server)))
	default:
		checks = append(checks, passCheck("服务器", server))
	}

	portText := yamlScalarInBlock(proxyBlock, "port")
	port, _ := strconv.Atoi(portText)
	if port != rt.Port {
		checks = append(checks, failCheck("端口", fmt.Sprintf("应为 %d，当前为 %s", rt.Port, portText)))
	} else {
		checks = append(checks, passCheck("端口", portText))
	}

	publicKey := yamlNestedScalar(proxyBlock, "reality-opts", "public-key")
	switch {
	case publicKey == "":
		checks = append(checks, failCheck("Reality 公钥", "public-key 为空，连接会失败"))
	case publicKey != rt.PublicKey:
		checks = append(checks, failCheck("Reality 公钥", "与服务端当前公钥不一致"))
	default:
		checks = append(checks, passCheck("Reality 公钥", "已配置"))
	}

	shortID := yamlNestedScalar(proxyBlock, "reality-opts", "short-id")
	if shortID == "" {
		checks = append(checks, failCheck("Reality ShortID", "short-id 为空"))
	} else if shortID != rt.ShortID {
		checks = append(checks, failCheck("Reality ShortID", "与服务端当前 short-id 不一致"))
	} else {
		checks = append(checks, passCheck("Reality ShortID", shortID))
	}

	serverName := yamlScalarInBlock(proxyBlock, "servername")
	if serverName == "" {
		checks = append(checks, failCheck("SNI", "缺少 servername"))
	} else if serverName != rt.SNI {
		checks = append(checks, failCheck("SNI", fmt.Sprintf("应为 %s，当前为 %s", rt.SNI, serverName)))
	} else {
		checks = append(checks, passCheck("SNI", serverName))
	}

	expectedFlow := vpnUserFlow(user)
	flow := yamlScalarInBlock(proxyBlock, "flow")
	if expectedFlow == "" && flow != "" {
		checks = append(checks, failCheck("Flow", fmt.Sprintf("当前账号不应启用 flow，但配置含 %q", flow)))
	} else if expectedFlow != "" && flow != expectedFlow {
		checks = append(checks, failCheck("Flow", fmt.Sprintf("应为 %s，当前为 %s", expectedFlow, flow)))
	} else if expectedFlow != "" {
		checks = append(checks, passCheck("Flow", expectedFlow))
	} else {
		checks = append(checks, passCheck("Flow", "未启用（兼容模式）"))
	}

	if strings.Contains(raw, "name: PROXY") && strings.Contains(raw, "- "+user.Name) {
		checks = append(checks, passCheck("代理组", "PROXY 组包含本节点"))
	} else {
		checks = append(checks, warnCheck("代理组", "未确认 PROXY 组是否包含本节点"))
	}
	if proxyGroupListsDirect(raw) {
		checks = append(checks, failCheck("代理组选项", "PROXY 组含 DIRECT，若误选会导致外网无法访问；请重新下载最新配置"))
	} else {
		checks = append(checks, passCheck("代理组选项", "PROXY 组未包含 DIRECT，外网会走代理节点"))
	}

	return checks
}

func proxyGroupListsDirect(body string) bool {
	lines := strings.Split(body, "\n")
	inProxyGroup := false
	inProxiesList := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "- name: PROXY" || strings.HasPrefix(trimmed, "- name: PROXY ") {
			inProxyGroup = true
			inProxiesList = false
			continue
		}
		if inProxyGroup {
			if strings.HasPrefix(trimmed, "- name:") && !strings.Contains(trimmed, "PROXY") {
				break
			}
			if trimmed == "proxies:" {
				inProxiesList = true
				continue
			}
			if inProxiesList {
				if strings.HasPrefix(trimmed, "- ") {
					entry := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
					if strings.EqualFold(entry, "DIRECT") {
						return true
					}
					continue
				}
				if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
					break
				}
			}
		}
	}
	return false
}

func validateVLESSLink(body string, user vpnUser, rt vpnRuntime) []vpnConfigValidationCheck {
	raw := stripConfigMetaLines(body)
	checks := make([]vpnConfigValidationCheck, 0, 6)
	if !strings.HasPrefix(raw, "vless://") {
		checks = append(checks, failCheck("链接格式", "不是有效的 VLESS 链接"))
		return checks
	}
	checks = append(checks, passCheck("链接格式", "VLESS 分享链接"))
	if strings.Contains(strings.ToLower(raw), strings.ToLower(user.UUID)) {
		checks = append(checks, passCheck("UUID", "与当前账号一致"))
	} else {
		checks = append(checks, failCheck("UUID", "与当前账号不一致"))
	}
	if strings.Contains(raw, rt.ServerHost) {
		checks = append(checks, passCheck("服务器", rt.ServerHost))
	} else if strings.Contains(raw, "127.0.0.1") {
		checks = append(checks, failCheck("服务器", "链接指向本地地址"))
	} else {
		checks = append(checks, failCheck("服务器", "未包含当前服务端地址"))
	}
	if strings.Contains(raw, "pbk=") && !strings.Contains(raw, "pbk=&") {
		checks = append(checks, passCheck("Reality 公钥", "已包含 pbk"))
	} else {
		checks = append(checks, failCheck("Reality 公钥", "公钥缺失或为空"))
	}
	return checks
}

func validateSingBoxJSON(body string, user vpnUser, rt vpnRuntime) []vpnConfigValidationCheck {
	raw := stripConfigMetaLines(body)
	checks := make([]vpnConfigValidationCheck, 0, 6)
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return []vpnConfigValidationCheck{failCheck("JSON 格式", "无法解析 sing-box 配置")}
	}
	checks = append(checks, passCheck("JSON 格式", "可解析"))
	encoded, _ := json.Marshal(doc)
	text := string(encoded)
	if strings.Contains(strings.ToLower(text), strings.ToLower(user.UUID)) {
		checks = append(checks, passCheck("UUID", "与当前账号一致"))
	} else {
		checks = append(checks, failCheck("UUID", "与当前账号不一致"))
	}
	if strings.Contains(text, rt.ServerHost) {
		checks = append(checks, passCheck("服务器", rt.ServerHost))
	} else {
		checks = append(checks, failCheck("服务器", "未包含当前服务端地址"))
	}
	if strings.Contains(text, rt.PublicKey) {
		checks = append(checks, passCheck("Reality 公钥", "已配置"))
	} else {
		checks = append(checks, failCheck("Reality 公钥", "缺失或不一致"))
	}
	return checks
}

func stripConfigMetaLines(body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# vpn-config-") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func parseConfigMeta(body string) (version, checksum string) {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# vpn-config-version:") {
			version = strings.TrimSpace(strings.TrimPrefix(trimmed, "# vpn-config-version:"))
		}
		if strings.HasPrefix(trimmed, "# vpn-config-checksum:") {
			checksum = strings.TrimSpace(strings.TrimPrefix(trimmed, "# vpn-config-checksum:"))
		}
	}
	return version, checksum
}

func yamlTopLevelValue(body, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return ""
}

func yamlScalarInBlock(block, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			return strings.Trim(value, `"'`)
		}
	}
	return ""
}

func yamlNestedScalar(block, section, key string) string {
	lines := strings.Split(block, "\n")
	inSection := false
	sectionPrefix := section + ":"
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == sectionPrefix || strings.HasPrefix(trimmed, sectionPrefix+" ") {
			inSection = true
			continue
		}
		if inSection {
			if strings.HasPrefix(trimmed, "- ") || (strings.Contains(trimmed, ":") && !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "\t\t")) {
				if !strings.HasPrefix(trimmed, key+":") {
					break
				}
			}
			if strings.HasPrefix(trimmed, key+":") {
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, key+":"))
				return strings.Trim(value, `"'`)
			}
		}
	}
	return ""
}

var clashProxyNamePattern = regexp.MustCompile(`(?m)^\s*-\s*name:\s*"?([^"\n]+)"?\s*$`)

func extractClashProxyBlock(body, name string) string {
	lines := strings.Split(body, "\n")
	inProxies := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "proxies:" {
			inProxies = true
			continue
		}
		if !inProxies {
			continue
		}
		if trimmed == "proxy-groups:" || trimmed == "rules:" {
			break
		}
		match := clashProxyNamePattern.FindStringSubmatch(line)
		if len(match) == 2 && strings.TrimSpace(match[1]) == name {
			block := []string{line}
			for j := i + 1; j < len(lines); j++ {
				next := lines[j]
				nextTrim := strings.TrimSpace(next)
				if nextTrim == "" {
					block = append(block, next)
					continue
				}
				if strings.HasPrefix(nextTrim, "- name:") {
					break
				}
				if !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
					break
				}
				block = append(block, next)
			}
			return strings.Join(block, "\n")
		}
	}
	return ""
}

func passCheck(name, message string) vpnConfigValidationCheck {
	return vpnConfigValidationCheck{Name: name, OK: true, Message: message}
}

func failCheck(name, message string) vpnConfigValidationCheck {
	return vpnConfigValidationCheck{Name: name, OK: false, Message: message}
}

func warnCheck(name, message string) vpnConfigValidationCheck {
	return vpnConfigValidationCheck{Name: name, OK: true, Message: message}
}

func (a *app) vpnConfigPost(w http.ResponseWriter, r *http.Request, current sessionUser) {
	id, kind, err := vpnConfigPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if kind != "validate-yaml" {
		writeError(w, http.StatusNotFound, errors.New("unknown config action"))
		return
	}
	user, err := a.vpn.getUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if current.Role != "admin" && current.ID != user.ID {
		writeError(w, http.StatusForbidden, errors.New("cannot access another user's config"))
		return
	}
	rt := currentVPNRuntime()
	var req struct {
		Kind    string `json:"kind"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Kind = strings.TrimSpace(req.Kind)
	if req.Kind == "" {
		req.Kind = "clash-android"
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, errors.New("content is required"))
		return
	}
	item := validateConfigContent(user, rt, req.Kind, req.Content, "upload")
	writeJSON(w, http.StatusOK, vpnConfigValidationResponse{OK: item.OK, Items: []vpnConfigValidationItem{item}})
}
