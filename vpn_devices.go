package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type vpnDeviceStatus struct {
	SourceIP            string    `json:"source_ip"`
	DisplayName         string    `json:"display_name"`
	DeviceType          string    `json:"device_type"`
	DeviceTypeLabel     string    `json:"device_type_label"`
	DisplayLabel        string    `json:"display_label"`
	ActiveConnections   int       `json:"active_connections"`
	Online              bool      `json:"online"`
	FirstSeenAt         time.Time `json:"first_seen_at"`
	LastSeenAt          time.Time `json:"last_seen_at"`
}

type vpnUserWithDevices struct {
	vpnUser
	OnlineDeviceCount int               `json:"online_device_count"`
	OnlineDevices     []vpnDeviceStatus `json:"online_devices"`
	KnownDevices      []vpnDeviceStatus `json:"known_devices"`
}

type clashConnectionsResponse struct {
	Connections []clashConnection `json:"connections"`
}

type clashConnection struct {
	Metadata clashConnectionMetadata `json:"metadata"`
}

type clashConnectionMetadata struct {
	SourceIP    string `json:"sourceIP"`
	InboundUser string `json:"inboundUser"`
	InboundName string `json:"inboundName"`
	User        string `json:"user"`
	Type        string `json:"type"`
}

var deviceTypeLabels = map[string]string{
	"unknown": "未知设备",
	"phone":   "手机",
	"mac":     "Mac",
	"pc":      "台式机/Windows",
	"tablet":  "平板",
	"tv":      "电视/盒子",
	"other":   "其他",
}

func migrateVPNDevices(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS vpn_device_registry (
	user_id INTEGER NOT NULL,
	source_ip TEXT NOT NULL,
	display_name TEXT NOT NULL DEFAULT '',
	device_type TEXT NOT NULL DEFAULT 'unknown',
	first_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (user_id, source_ip),
	FOREIGN KEY(user_id) REFERENCES vpn_users(id) ON DELETE CASCADE
);
`)
	return err
}

func normalizeDeviceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "phone", "mac", "pc", "tablet", "tv", "other":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func deviceTypeLabel(deviceType string) string {
	if label, ok := deviceTypeLabels[normalizeDeviceType(deviceType)]; ok {
		return label
	}
	return deviceTypeLabels["unknown"]
}

func deviceDisplayLabel(displayName, deviceType, sourceIP string) string {
	name := strings.TrimSpace(displayName)
	if name != "" {
		return name
	}
	return deviceTypeLabel(deviceType) + " (" + sourceIP + ")"
}

func clashAPISettings() (listen, secret string, enabled bool) {
	listen = strings.TrimSpace(env("VPN_CLASH_API_LISTEN", "127.0.0.1:9090"))
	secret = strings.TrimSpace(env("VPN_CLASH_API_SECRET", ""))
	return listen, secret, secret != ""
}

func fetchClashConnections(ctx context.Context) ([]clashConnection, error) {
	listen, secret, enabled := clashAPISettings()
	if !enabled {
		return nil, errors.New("VPN_CLASH_API_SECRET is not configured")
	}
	url := "http://" + listen + "/connections"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	client := &http.Client{Timeout: 4 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clash api /connections returned %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	var payload clashConnectionsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload.Connections, nil
}

func (m vpnManager) syncOnlineDevices(ctx context.Context, users []vpnUser) (map[int64][]vpnDeviceStatus, string, error) {
	refreshUserIPMappings(ctx)
	connections, err := fetchClashConnections(ctx)
	if err != nil {
		return nil, "", err
	}
	userByName := make(map[string]vpnUser, len(users))
	for _, user := range users {
		userByName[user.Name] = user
	}
	type liveDevice struct {
		connections int
	}
	live := make(map[int64]map[string]*liveDevice)
	for _, conn := range connections {
		meta := conn.Metadata
		if !isVPNInboundConnection(meta) {
			continue
		}
		sourceIP := strings.TrimSpace(meta.SourceIP)
		if sourceIP == "" {
			continue
		}
		userName := clashConnectionUser(meta, sourceIP)
		if userName == "" {
			continue
		}
		user, ok := userByName[userName]
		if !ok || !user.Enabled {
			continue
		}
		if live[user.ID] == nil {
			live[user.ID] = make(map[string]*liveDevice)
		}
		entry := live[user.ID][sourceIP]
		if entry == nil {
			entry = &liveDevice{}
			live[user.ID][sourceIP] = entry
		}
		entry.connections++
	}
	now := time.Now().UTC()
	for userID, devices := range live {
		for sourceIP, entry := range devices {
			if err := m.touchDeviceRegistry(ctx, userID, sourceIP, now); err != nil {
				return nil, "", err
			}
			_ = entry
		}
	}
	result := make(map[int64][]vpnDeviceStatus, len(users))
	note := "在线设备按客户端公网 IP 统计；同一 WiFi 下多台设备可能显示为 1 台，切换 4G/WiFi 可能被算作 2 台。设备名称和类型可在「管理设备」中手动标注。"
	for _, user := range users {
		online := make([]vpnDeviceStatus, 0)
		if devices, ok := live[user.ID]; ok {
			for sourceIP, entry := range devices {
				status, err := m.deviceStatus(ctx, user.ID, sourceIP, true, entry.connections)
				if err != nil {
					return nil, "", err
				}
				online = append(online, status)
			}
		}
		result[user.ID] = online
	}
	return result, note, nil
}

func (m vpnManager) touchDeviceRegistry(ctx context.Context, userID int64, sourceIP string, now time.Time) error {
	nowText := now.Format(time.RFC3339)
	_, err := m.db.ExecContext(ctx, `
INSERT INTO vpn_device_registry (user_id, source_ip, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(user_id, source_ip) DO UPDATE SET last_seen_at = excluded.last_seen_at;
`, userID, sourceIP, nowText, nowText)
	return err
}

func (m vpnManager) deviceStatus(ctx context.Context, userID int64, sourceIP string, online bool, activeConnections int) (vpnDeviceStatus, error) {
	var displayName, deviceType, firstSeenAt, lastSeenAt string
	err := m.db.QueryRowContext(ctx, `
SELECT display_name, device_type, first_seen_at, last_seen_at
FROM vpn_device_registry
WHERE user_id = ? AND source_ip = ?;
`, userID, sourceIP).Scan(&displayName, &deviceType, &firstSeenAt, &lastSeenAt)
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().UTC()
		if err := m.touchDeviceRegistry(ctx, userID, sourceIP, now); err != nil {
			return vpnDeviceStatus{}, err
		}
		firstSeenAt = now.Format(time.RFC3339)
		lastSeenAt = firstSeenAt
		deviceType = "unknown"
	} else if err != nil {
		return vpnDeviceStatus{}, err
	}
	deviceType = normalizeDeviceType(deviceType)
	return vpnDeviceStatus{
		SourceIP:          sourceIP,
		DisplayName:       displayName,
		DeviceType:        deviceType,
		DeviceTypeLabel:   deviceTypeLabel(deviceType),
		DisplayLabel:      deviceDisplayLabel(displayName, deviceType, sourceIP),
		ActiveConnections: activeConnections,
		Online:            online,
		FirstSeenAt:         parseSQLiteTime(firstSeenAt),
		LastSeenAt:          parseSQLiteTime(lastSeenAt),
	}, nil
}

func (m vpnManager) listKnownDevices(ctx context.Context, userID int64, online map[string]int) ([]vpnDeviceStatus, error) {
	rows, err := m.db.QueryContext(ctx, `
SELECT source_ip, display_name, device_type, first_seen_at, last_seen_at
FROM vpn_device_registry
WHERE user_id = ?
ORDER BY last_seen_at DESC
LIMIT 20;
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices := make([]vpnDeviceStatus, 0)
	for rows.Next() {
		var sourceIP, displayName, deviceType, firstSeenAt, lastSeenAt string
		if err := rows.Scan(&sourceIP, &displayName, &deviceType, &firstSeenAt, &lastSeenAt); err != nil {
			return nil, err
		}
		deviceType = normalizeDeviceType(deviceType)
		activeConnections := online[sourceIP]
		devices = append(devices, vpnDeviceStatus{
			SourceIP:          sourceIP,
			DisplayName:       displayName,
			DeviceType:        deviceType,
			DeviceTypeLabel:   deviceTypeLabel(deviceType),
			DisplayLabel:      deviceDisplayLabel(displayName, deviceType, sourceIP),
			ActiveConnections: activeConnections,
			Online:            activeConnections > 0,
			FirstSeenAt:         parseSQLiteTime(firstSeenAt),
			LastSeenAt:          parseSQLiteTime(lastSeenAt),
		})
	}
	return devices, rows.Err()
}

func (m vpnManager) updateDeviceRegistry(ctx context.Context, userID int64, sourceIP, displayName, deviceType string) (vpnDeviceStatus, error) {
	deviceType = normalizeDeviceType(deviceType)
	displayName = strings.TrimSpace(displayName)
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := m.db.ExecContext(ctx, `
UPDATE vpn_device_registry
SET display_name = ?, device_type = ?, last_seen_at = ?
WHERE user_id = ? AND source_ip = ?;
`, displayName, deviceType, now, userID, sourceIP)
	if err != nil {
		return vpnDeviceStatus{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		if err := m.touchDeviceRegistry(ctx, userID, sourceIP, time.Now().UTC()); err != nil {
			return vpnDeviceStatus{}, err
		}
		_, err = m.db.ExecContext(ctx, `
UPDATE vpn_device_registry
SET display_name = ?, device_type = ?
WHERE user_id = ? AND source_ip = ?;
`, displayName, deviceType, userID, sourceIP)
		if err != nil {
			return vpnDeviceStatus{}, err
		}
	}
	return m.deviceStatus(ctx, userID, sourceIP, false, 0)
}

func (m vpnManager) usersWithDevices(ctx context.Context, users []vpnUser) ([]vpnUserWithDevices, string, error) {
	_, secret, enabled := clashAPISettings()
	if !enabled {
		out := make([]vpnUserWithDevices, 0, len(users))
		for _, user := range users {
			out = append(out, vpnUserWithDevices{vpnUser: user})
		}
		return out, "未配置 VPN_CLASH_API_SECRET，无法统计在线设备；请在 vpn.env 中配置后点击「重新应用配置」。", nil
	}
	onlineByUser, note, err := m.syncOnlineDevices(ctx, users)
	if err != nil {
		log.Printf("sync online devices: %v", err)
		out := make([]vpnUserWithDevices, 0, len(users))
		for _, user := range users {
			out = append(out, vpnUserWithDevices{vpnUser: user})
		}
		return out, "暂时无法读取 sing-box 在线连接，请确认已重新应用配置且 sing-box 正在运行。", nil
	}
	out := make([]vpnUserWithDevices, 0, len(users))
	for _, user := range users {
		online := onlineByUser[user.ID]
		if online == nil {
			online = []vpnDeviceStatus{}
		}
		onlineMap := make(map[string]int, len(online))
		for _, device := range online {
			onlineMap[device.SourceIP] = device.ActiveConnections
		}
		known, err := m.listKnownDevices(ctx, user.ID, onlineMap)
		if err != nil {
			return nil, "", err
		}
		out = append(out, vpnUserWithDevices{
			vpnUser:           user,
			OnlineDeviceCount: len(online),
			OnlineDevices:     online,
			KnownDevices:      known,
		})
	}
	_ = secret
	return out, note, nil
}

func (a *app) vpnUpdateDevice(w http.ResponseWriter, r *http.Request, current sessionUser) {
	userID, sourceIP, err := vpnDevicePath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if current.Role != "admin" && current.ID != userID {
		writeError(w, http.StatusForbidden, errors.New("cannot edit another user's device"))
		return
	}
	var req struct {
		DisplayName *string `json:"display_name"`
		DeviceType  *string `json:"device_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.DisplayName == nil && req.DeviceType == nil {
		writeError(w, http.StatusBadRequest, errors.New("display_name or device_type is required"))
		return
	}
	currentDevice, err := a.vpn.deviceStatus(r.Context(), userID, sourceIP, false, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	displayName := currentDevice.DisplayName
	deviceType := currentDevice.DeviceType
	if req.DisplayName != nil {
		displayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.DeviceType != nil {
		deviceType = normalizeDeviceType(*req.DeviceType)
	}
	device, err := a.vpn.updateDeviceRegistry(r.Context(), userID, sourceIP, displayName, deviceType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func vpnDevicePath(path string) (int64, string, error) {
	part := strings.TrimPrefix(path, "/api/vpn/devices/")
	part = strings.Trim(part, "/")
	pieces := strings.Split(part, "/")
	if len(pieces) != 2 {
		return 0, "", errors.New("expected /api/vpn/devices/{id}/{source_ip}")
	}
	userID, err := strconv.ParseInt(pieces[0], 10, 64)
	if err != nil {
		return 0, "", err
	}
	sourceIP := strings.TrimSpace(pieces[1])
	if sourceIP == "" {
		return 0, "", errors.New("source_ip is required")
	}
	return userID, sourceIP, nil
}
