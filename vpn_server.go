package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

const xrayAccessLogPath = "/var/log/xray/access.log"

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

func clashProxyGroup(user vpnUser) string {
	proxyNames := []string{user.Name}
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
