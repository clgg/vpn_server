package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	vpnHy2InboundTag = "hysteria2-in"
	xrayAccessLogPath = "/var/log/xray/access.log"
)

func hy2AuthSecret() string {
	secret := strings.TrimSpace(env("VPN_HY2_AUTH_SECRET", ""))
	if secret != "" {
		return secret
	}
	return env("VPN_ADMIN_PASSWORD", "change-me-before-use")
}

func hy2UserPassword(user vpnUser) string {
	mac := hmac.New(sha256.New, []byte(hy2AuthSecret()))
	mac.Write([]byte("hy2:" + user.UUID))
	return hex.EncodeToString(mac.Sum(nil))[:24]
}

func hy2UserAuth(user vpnUser) string {
	return user.Name + ":" + hy2UserPassword(user)
}

func hy2Port() int {
	return envInt("VPN_HY2_PORT", envInt("VPN_SERVER_PORT", 443))
}

func hy2ExtraPorts() []int {
	raw := strings.TrimSpace(env("VPN_HY2_EXTRA_PORTS", "51820"))
	if raw == "" || strings.EqualFold(raw, "none") {
		return nil
	}
	seen := make(map[int]struct{})
	ports := make([]int, 0, 4)
	mainPort := hy2Port()
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		port, err := strconv.Atoi(part)
		if err != nil || port < 1 || port > 65535 {
			continue
		}
		if port == mainPort {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}

func hy2PortUsesObfs(port int) bool {
	key := fmt.Sprintf("VPN_HY2_%d_OBFS", port)
	if value, ok := os.LookupEnv(key); ok {
		v := strings.ToLower(strings.TrimSpace(value))
		return v != "" && v != "none" && v != "off" && v != "false"
	}
	// UDP 51820 is often used to bypass QUIC filtering; plain hy2 is more compatible.
	if port == 51820 {
		return false
	}
	return hy2ObfsType() != "" && hy2ObfsType() != "none"
}

func hy2SNI() string {
	return env("VPN_HY2_SNI", "www.bing.com")
}

func hy2TLSPaths() (cert, key string) {
	cert = env("VPN_HY2_TLS_CERT", "/etc/hysteria/server.crt")
	key = env("VPN_HY2_TLS_KEY", "/etc/hysteria/server.key")
	return cert, key
}

func hy2ObfsPassword() string {
	mac := hmac.New(sha256.New, []byte(hy2AuthSecret()))
	mac.Write([]byte("hy2-obfs"))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func hy2ObfsType() string {
	return strings.ToLower(strings.TrimSpace(env("VPN_HY2_OBFS", "salamander")))
}

func hy2BandwidthMbps() int {
	if v := envInt("VPN_HY2_BANDWIDTH_MBPS", 0); v > 0 {
		return v
	}
	return 100
}

func xrayServerConfig(users []vpnUser) (string, error) {
	privateKey := strings.TrimSpace(env("VPN_REALITY_PRIVATE_KEY", ""))
	if privateKey == "" {
		return "", fmt.Errorf("VPN_REALITY_PRIVATE_KEY is required")
	}
	rt := currentVPNRuntime()
	clients := make([]map[string]any, 0)
	for _, user := range users {
		if !user.Enabled {
			continue
		}
		client := map[string]any{
			"id":    user.UUID,
			"email": user.Name,
		}
		if flow := vpnUserFlow(user); flow != "" {
			client["flow"] = flow
		}
		clients = append(clients, client)
	}
	if len(clients) == 0 {
		return "", fmt.Errorf("at least one enabled user is required")
	}
	config := map[string]any{
		"log": map[string]any{
			"access":   xrayAccessLogPath,
			"error":    "/var/log/xray/error.log",
			"loglevel": "info",
		},
		"inbounds": []any{
			map[string]any{
				"listen":   "0.0.0.0",
				"port":     rt.Port,
				"protocol": "vless",
				"tag":      vpnInboundTag,
				"settings": map[string]any{
					"clients":    clients,
					"decryption": "none",
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "reality",
					"realitySettings": map[string]any{
						"show":        false,
						"dest":        rt.SNI + ":443",
						"xver":        0,
						"serverNames": []string{rt.SNI},
						"privateKey":  privateKey,
						"shortIds":    []string{rt.ShortID},
					},
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls", "quic"},
				},
			},
		},
		"outbounds": []any{
			map[string]any{"protocol": "freedom", "tag": "direct"},
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func hysteria2ServerConfig(users []vpnUser) (string, error) {
	return hysteria2ServerConfigForPort(hy2Port(), hy2PortUsesObfs(hy2Port()), users)
}

func hysteria2ServerConfigForPort(port int, useObfs bool, users []vpnUser) (string, error) {
	userpass := make(map[string]string)
	for _, user := range users {
		if user.Enabled {
			userpass[user.Name] = hy2UserPassword(user)
		}
	}
	if len(userpass) == 0 {
		return "", fmt.Errorf("at least one enabled user is required")
	}
	cert, key := hy2TLSPaths()
	sni := hy2SNI()
	bw := hy2BandwidthMbps()
	lines := []string{
		fmt.Sprintf("listen: 0.0.0.0:%d", port),
		"",
		"tls:",
		fmt.Sprintf("  cert: %s", cert),
		fmt.Sprintf("  key: %s", key),
		"",
		"auth:",
		"  type: userpass",
		"  userpass:",
	}
	names := make([]string, 0, len(userpass))
	for name := range userpass {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("    %s: %s", name, userpass[name]))
	}
	if useObfs {
		obfsType := hy2ObfsType()
		if obfsType != "" && obfsType != "none" {
			lines = append(lines,
				"",
				"obfs:",
				fmt.Sprintf("  type: %s", obfsType),
				fmt.Sprintf("  %s:", obfsType),
				fmt.Sprintf("    password: %s", hy2ObfsPassword()),
			)
		}
	}
	lines = append(lines,
		"",
		"masquerade:",
		"  type: proxy",
		"  proxy:",
		fmt.Sprintf("    url: https://%s", sni),
		"    rewriteHost: true",
		"",
		"bandwidth:",
		fmt.Sprintf("  up: %d mbps", bw),
		fmt.Sprintf("  down: %d mbps", bw),
		"",
	)
	return strings.Join(lines, "\n"), nil
}

func hy2ClashProxyName(user vpnUser, port int) string {
	if port == hy2Port() {
		return user.Name + "-hy2"
	}
	return fmt.Sprintf("%s-hy2-%d", user.Name, port)
}

func clashHy2Proxy(user vpnUser, rt vpnRuntime) string {
	return clashHy2ProxyForPort(user, rt, hy2Port(), hy2PortUsesObfs(hy2Port()))
}

func clashHy2ProxyForPort(user vpnUser, rt vpnRuntime, port int, useObfs bool) string {
	bw := hy2BandwidthMbps()
	obfsBlock := ""
	if useObfs {
		if obfsType := hy2ObfsType(); obfsType != "" && obfsType != "none" {
			obfsBlock = fmt.Sprintf(`    obfs: %s
    obfs-password: %s
`, obfsType, hy2ObfsPassword())
		}
	}
	return fmt.Sprintf(`  - name: %s
    type: hysteria2
    server: %s
    port: %d
    password: "%s"
    up: %d
    down: %d
    sni: %s
    skip-cert-verify: true
%s`, hy2ClashProxyName(user, port), rt.ServerHost, port, hy2UserAuth(user), bw, bw, hy2SNI(), obfsBlock)
}

func clashAllHy2Proxies(user vpnUser, rt vpnRuntime) string {
	parts := []string{clashHy2Proxy(user, rt)}
	for _, port := range hy2ExtraPorts() {
		parts = append(parts, clashHy2ProxyForPort(user, rt, port, hy2PortUsesObfs(port)))
	}
	return strings.Join(parts, "")
}

func clashProxyNames(user vpnUser) (hy2Name, vlessName string) {
	return user.Name + "-hy2", user.Name
}

func clashProxyGroup(user vpnUser) string {
	proxyNames := []string{user.Name}
	for _, port := range hy2ExtraPorts() {
		proxyNames = append(proxyNames, hy2ClashProxyName(user, port))
	}
	proxyNames = append(proxyNames, hy2ClashProxyName(user, hy2Port()))
	return fmt.Sprintf(`  - name: PROXY
    type: select
    proxies:
%s`, clashProxyListLines(proxyNames))
}

func clashProxyListLines(names []string) string {
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("      - %s", name))
	}
	return strings.Join(lines, "\n") + "\n"
}
