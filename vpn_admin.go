package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	vpnInboundTag = "vless-reality-in"
	vpnFlow       = "xtls-rprx-vision"
)

type vpnManager struct {
	db *sql.DB
}

type vpnUser struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	UUID       string    `json:"uuid"`
	Enabled    bool      `json:"enabled"`
	Note       string    `json:"note"`
	LoginEmail string    `json:"login_email"`
	Role       string    `json:"role"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type sessionUser struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	LoginEmail string `json:"login_email"`
	Role       string `json:"role"`
}

type vpnRuntime struct {
	ServerHost string `json:"server_host"`
	Port       int    `json:"port"`
	SNI        string `json:"sni"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
}

func newVPNManager(db *sql.DB) vpnManager {
	return vpnManager{db: db}
}

func migrateVPN(db *sql.DB) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS vpn_users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	uuid TEXT NOT NULL UNIQUE,
	enabled INTEGER NOT NULL DEFAULT 1,
	note TEXT NOT NULL DEFAULT '',
	login_email TEXT NOT NULL DEFAULT '',
	password_hash TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL DEFAULT 'user',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS vpn_sessions (
	token TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(user_id) REFERENCES vpn_users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS vpn_traffic_samples (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	interface TEXT NOT NULL,
	rx_bytes INTEGER NOT NULL,
	tx_bytes INTEGER NOT NULL,
	sampled_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`); err != nil {
		return err
	}
	for _, col := range []struct {
		name string
		sql  string
	}{
		{"login_email", "ALTER TABLE vpn_users ADD COLUMN login_email TEXT NOT NULL DEFAULT ''"},
		{"password_hash", "ALTER TABLE vpn_users ADD COLUMN password_hash TEXT NOT NULL DEFAULT ''"},
		{"role", "ALTER TABLE vpn_users ADD COLUMN role TEXT NOT NULL DEFAULT 'user'"},
	} {
		if err := ensureVPNColumn(db, col.name, col.sql); err != nil {
			return err
		}
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM vpn_users`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := db.Exec(`
INSERT INTO vpn_users (name, uuid, enabled, note)
VALUES (?, ?, 1, ?);
`, env("VPN_DEFAULT_USER_NAME", "owner"), env("VPN_DEFAULT_USER_UUID", "da289730-2524-44f4-8d76-d6a7af321084"), "seeded from existing sing-box user")
		if err != nil {
			return err
		}
	}
	if err := migrateVPNDevices(db); err != nil {
		return err
	}
	return seedAdminAccount(db)
}

func ensureVPNColumn(db *sql.DB, name, sqlText string) error {
	rows, err := db.Query(`PRAGMA table_info(vpn_users)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if colName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(sqlText)
	return err
}

func seedAdminAccount(db *sql.DB) error {
	email := strings.TrimSpace(env("VPN_ADMIN_EMAIL", "admin@example.com"))
	password := env("VPN_ADMIN_PASSWORD", "change-me-before-use")
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	var id int64
	err = db.QueryRow(`SELECT id FROM vpn_users WHERE login_email = ?`, email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = db.QueryRow(`SELECT id FROM vpn_users ORDER BY id LIMIT 1`).Scan(&id)
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(`
UPDATE vpn_users
SET login_email = ?, password_hash = ?, role = 'admin', updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
`, email, hash, id)
	return err
}

func (a *app) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<meta http-equiv="refresh" content="0; url=/vpn-admin">`)
}

func (a *app) authMe(w http.ResponseWriter, r *http.Request) {
	user, ok := a.sessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("not logged in"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *app) authLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, passwordHash, err := a.vpn.getUserByEmail(r.Context(), strings.TrimSpace(req.Email))
	if err != nil || !verifyPassword(strings.TrimSpace(req.Password), passwordHash) {
		writeError(w, http.StatusUnauthorized, errors.New("email or password is incorrect"))
		return
	}
	if !user.Enabled {
		writeError(w, http.StatusForbidden, errors.New("user is disabled"))
		return
	}
	token, expires, err := a.vpn.createSession(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "vpn_session",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"user": user.sessionUser()})
}

func (a *app) authLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("vpn_session"); err == nil {
		_ = a.vpn.deleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "vpn_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (a *app) requireLogin(next func(http.ResponseWriter, *http.Request, sessionUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.sessionUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, errors.New("not logged in"))
			return
		}
		next(w, r, user)
	}
}

func (a *app) requireAdmin(next func(http.ResponseWriter, *http.Request, sessionUser)) http.HandlerFunc {
	return a.requireLogin(func(w http.ResponseWriter, r *http.Request, user sessionUser) {
		if user.Role != "admin" {
			writeError(w, http.StatusForbidden, errors.New("admin role is required"))
			return
		}
		next(w, r, user)
	})
}

func (a *app) sessionUser(r *http.Request) (sessionUser, bool) {
	cookie, err := r.Cookie("vpn_session")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return sessionUser{}, false
	}
	user, err := a.vpn.getSessionUser(r.Context(), cookie.Value)
	if err != nil {
		return sessionUser{}, false
	}
	return user, true
}

func (a *app) vpnStatus(w http.ResponseWriter, r *http.Request, current sessionUser) {
	rt := currentVPNRuntime()
	if current.Role != "admin" {
		writeJSON(w, http.StatusOK, map[string]any{
			"runtime":       rt,
			"traffic_total": interfaceTraffic{Interface: env("VPN_TRAFFIC_INTERFACE", "ens5")},
			"stats_note":    "当前服务端尚未启用用户级精确统计；普通用户不展示服务器总流量，避免误认为是个人用量。",
		})
		return
	}
	total, err := readInterfaceTraffic(env("VPN_TRAFFIC_INTERFACE", "ens5"))
	if err != nil {
		log.Printf("read interface traffic: %v", err)
	} else if err := a.vpn.recordTrafficSample(r.Context(), total); err != nil {
		log.Printf("record interface traffic: %v", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"runtime":       rt,
		"traffic_total": total,
		"stats_note":    "管理员看到的是服务器网卡总流量；用户级精确流量需要启用带 V2Ray API 的 sing-box 或接入 Xray/面板。",
	})
}

func (a *app) vpnTrafficHistory(w http.ResponseWriter, r *http.Request, current sessionUser) {
	name := env("VPN_TRAFFIC_INTERFACE", "ens5")
	bucket := strings.TrimSpace(r.URL.Query().Get("bucket"))
	if current.Role != "admin" {
		writeJSON(w, http.StatusOK, emptyTrafficHistory(bucket, name, "当前服务端尚未启用用户级精确统计；这里不展示服务器总流量。"))
		return
	}
	total, err := readInterfaceTraffic(name)
	if err == nil {
		if err := a.vpn.recordTrafficSample(r.Context(), total); err != nil {
			log.Printf("record interface traffic: %v", err)
		}
	}
	history, err := a.vpn.trafficHistory(r.Context(), name, bucket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (a *app) vpnUsers(w http.ResponseWriter, r *http.Request, current sessionUser) {
	var users []vpnUser
	var err error
	if current.Role == "admin" {
		users, err = a.vpn.listUsers(r.Context())
	} else {
		var user vpnUser
		user, err = a.vpn.getUser(r.Context(), current.ID)
		if err == nil {
			users = []vpnUser{user}
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	enriched, devicesNote, err := a.vpn.usersWithDevices(r.Context(), users)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":         enriched,
		"current_user":  current,
		"devices_note":  devicesNote,
	})
}

func (a *app) vpnCreateUser(w http.ResponseWriter, r *http.Request, current sessionUser) {
	var req struct {
		Name       string `json:"name"`
		Note       string `json:"note"`
		LoginEmail string `json:"login_email"`
		Password   string `json:"password"`
		Role       string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Name = sanitizeUserName(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	uuid, err := generateUUID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	user, err := a.vpn.createUser(r.Context(), req.Name, uuid, strings.TrimSpace(req.Note), req.LoginEmail, req.Password, req.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := a.vpn.apply(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *app) vpnUpdateUser(w http.ResponseWriter, r *http.Request, current sessionUser) {
	id, err := vpnUserIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req struct {
		Name       *string `json:"name"`
		Note       *string `json:"note"`
		Enabled    *bool   `json:"enabled"`
		LoginEmail *string `json:"login_email"`
		Password   *string `json:"password"`
		Role       *string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if id == current.ID {
		if req.Enabled != nil && !*req.Enabled {
			writeError(w, http.StatusBadRequest, errors.New("cannot disable current admin"))
			return
		}
		if req.Role != nil && normalizeRole(*req.Role) != "admin" {
			writeError(w, http.StatusBadRequest, errors.New("cannot demote current admin"))
			return
		}
	}
	user, err := a.vpn.updateUser(r.Context(), id, req.Name, req.Note, req.Enabled, req.LoginEmail, req.Password, req.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := a.vpn.apply(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *app) vpnDeleteUser(w http.ResponseWriter, r *http.Request, current sessionUser) {
	id, err := vpnUserIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if id == current.ID {
		writeError(w, http.StatusBadRequest, errors.New("cannot delete current admin"))
		return
	}
	if err := a.vpn.deleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := a.vpn.apply(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) vpnApply(w http.ResponseWriter, r *http.Request, current sessionUser) {
	if err := a.vpn.apply(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

func (a *app) vpnConfig(w http.ResponseWriter, r *http.Request, current sessionUser) {
	id, kind, err := vpnConfigPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
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
	if err := validateVPNRuntime(rt); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	manifest := buildVPNConfigManifest(user, rt)
	switch kind {
	case "manifest":
		writeJSON(w, http.StatusOK, manifest)
	case "validate":
		kindParam := strings.TrimSpace(r.URL.Query().Get("kind"))
		if kindParam == "" {
			kindParam = "clash"
		}
		writeJSON(w, http.StatusOK, buildConfigValidation(user, rt, validationKindsForQuery(kindParam)))
	case "vless":
		serveConfigArtifact(w, user, rt, "vless", fmt.Sprintf(`"%s-vless.txt"`, user.Name), "text/plain; charset=utf-8")
	case "clash":
		serveConfigArtifact(w, user, rt, "clash", fmt.Sprintf(`"%s-clash.yaml"`, user.Name), "text/yaml; charset=utf-8")
	case "clash-android":
		serveConfigArtifact(w, user, rt, "clash-android", fmt.Sprintf(`"%s-clash-android.yaml"`, user.Name), "text/yaml; charset=utf-8")
	case "clash-import":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, clashImportURL(r, user))
	case "clash-android-import":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, clashAndroidImportURL(r, user))
	case "clash-import-qr":
		png, err := qrcode.Encode(clashImportURL(r, user), qrcode.Medium, 256)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	case "clash-android-import-qr":
		png, err := qrcode.Encode(clashAndroidImportURL(r, user), qrcode.Medium, 256)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	case "sing-box":
		serveConfigArtifact(w, user, rt, "sing-box", fmt.Sprintf(`"%s-sing-box.json"`, user.Name), "application/json; charset=utf-8")
	case "qr":
		png, err := qrcode.Encode(vlessURL(user, rt), qrcode.Medium, 256)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	case "sing-box-import":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, singBoxImportURL(r, user))
	case "sing-box-import-qr":
		png, err := qrcode.Encode(singBoxImportURL(r, user), qrcode.Medium, 256)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	case "rocket":
		serveConfigArtifact(w, user, rt, "rocket", fmt.Sprintf(`"%s-clg-rocket.json"`, user.Name), "application/json; charset=utf-8")
	case "rocket-import":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, rocketImportURL(r, user))
	case "rocket-import-qr":
		png, err := qrcode.Encode(rocketImportURL(r, user), qrcode.Medium, 256)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	default:
		writeError(w, http.StatusBadRequest, errors.New("unknown config kind"))
	}
}

func (a *app) rocketProfile(w http.ResponseWriter, r *http.Request) {
	uuid, err := rocketProfileUUIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if !verifyRocketProfileToken(uuid, token) {
		writeError(w, http.StatusForbidden, errors.New("invalid profile token"))
		return
	}
	user, err := a.vpn.getUserByUUID(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if !user.Enabled {
		writeError(w, http.StatusForbidden, errors.New("user is disabled"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := writeRocketProfile(w, user, currentVPNRuntime()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
}

func (a *app) vpnPublicConfig(w http.ResponseWriter, r *http.Request) {
	uuid, kind, err := vpnPublicConfigPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := a.vpn.getUserByUUID(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if !user.Enabled {
		writeError(w, http.StatusForbidden, errors.New("user is disabled"))
		return
	}
	rt := currentVPNRuntime()
	if err := validateVPNRuntime(rt); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch kind {
	case "sing-box.json":
		serveConfigArtifact(w, user, rt, "sing-box", fmt.Sprintf(`"%s-sing-box.json"`, user.Name), "application/json; charset=utf-8")
	case "clash.yaml":
		serveConfigArtifact(w, user, rt, "clash", fmt.Sprintf(`"%s-clash.yaml"`, user.Name), "text/yaml; charset=utf-8")
	case "clash-android.yaml":
		serveConfigArtifact(w, user, rt, "clash-android", fmt.Sprintf(`"%s-clash-android.yaml"`, user.Name), "text/yaml; charset=utf-8")
	case "vless.txt":
		serveConfigArtifact(w, user, rt, "vless", fmt.Sprintf(`"%s-vless.txt"`, user.Name), "text/plain; charset=utf-8")
	default:
		writeError(w, http.StatusBadRequest, errors.New("unknown public config kind"))
	}
}

func (m vpnManager) listUsers(ctx context.Context) ([]vpnUser, error) {
	rows, err := m.db.QueryContext(ctx, `
SELECT id, name, uuid, enabled, note, login_email, role, created_at, updated_at
FROM vpn_users
ORDER BY id ASC;
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]vpnUser, 0)
	for rows.Next() {
		user, err := scanVPNUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (m vpnManager) getUser(ctx context.Context, id int64) (vpnUser, error) {
	row := m.db.QueryRowContext(ctx, `
SELECT id, name, uuid, enabled, note, login_email, role, created_at, updated_at
FROM vpn_users
WHERE id = ?;
`, id)
	return scanVPNUser(row)
}

func (m vpnManager) getUserByUUID(ctx context.Context, uuid string) (vpnUser, error) {
	row := m.db.QueryRowContext(ctx, `
SELECT id, name, uuid, enabled, note, login_email, role, created_at, updated_at
FROM vpn_users
WHERE uuid = ?;
`, uuid)
	return scanVPNUser(row)
}

func (m vpnManager) createUser(ctx context.Context, name, uuid, note, loginEmail, password, role string) (vpnUser, error) {
	loginEmail = strings.TrimSpace(loginEmail)
	password = strings.TrimSpace(password)
	role = normalizeRole(role)
	passwordHash := ""
	if password != "" {
		var err error
		passwordHash, err = hashPassword(password)
		if err != nil {
			return vpnUser{}, err
		}
	}
	res, err := m.db.ExecContext(ctx, `
INSERT INTO vpn_users (name, uuid, note, login_email, password_hash, role)
VALUES (?, ?, ?, ?, ?, ?);
`, name, uuid, note, loginEmail, passwordHash, role)
	if err != nil {
		return vpnUser{}, err
	}
	id, _ := res.LastInsertId()
	return m.getUser(ctx, id)
}

func (m vpnManager) updateUser(ctx context.Context, id int64, name, note *string, enabled *bool, loginEmail, password, role *string) (vpnUser, error) {
	current, err := m.getUser(ctx, id)
	if err != nil {
		return vpnUser{}, err
	}
	nextName := current.Name
	if name != nil {
		nextName = sanitizeUserName(*name)
		if nextName == "" {
			return vpnUser{}, errors.New("name cannot be empty")
		}
	}
	nextNote := current.Note
	if note != nil {
		nextNote = strings.TrimSpace(*note)
	}
	nextEnabled := current.Enabled
	if enabled != nil {
		nextEnabled = *enabled
	}
	nextEmail := current.LoginEmail
	if loginEmail != nil {
		nextEmail = strings.TrimSpace(*loginEmail)
	}
	nextRole := current.Role
	if role != nil {
		nextRole = normalizeRole(*role)
	}
	enabledValue := 0
	if nextEnabled {
		enabledValue = 1
	}
	if password != nil {
		trimmedPassword := strings.TrimSpace(*password)
		if trimmedPassword == "" {
			password = nil
		} else {
			*password = trimmedPassword
		}
	}
	if password != nil {
		hash, err := hashPassword(*password)
		if err != nil {
			return vpnUser{}, err
		}
		if _, err := m.db.ExecContext(ctx, `
UPDATE vpn_users
SET name = ?, note = ?, enabled = ?, login_email = ?, password_hash = ?, role = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
`, nextName, nextNote, enabledValue, nextEmail, hash, nextRole, id); err != nil {
			return vpnUser{}, err
		}
		return m.getUser(ctx, id)
	}
	if _, err := m.db.ExecContext(ctx, `
UPDATE vpn_users
SET name = ?, note = ?, enabled = ?, login_email = ?, role = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
`, nextName, nextNote, enabledValue, nextEmail, nextRole, id); err != nil {
		return vpnUser{}, err
	}
	return m.getUser(ctx, id)
}

func (m vpnManager) deleteUser(ctx context.Context, id int64) error {
	res, err := m.db.ExecContext(ctx, `DELETE FROM vpn_users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("user %d not found", id)
	}
	return nil
}

func (m vpnManager) apply(ctx context.Context) error {
	users, err := m.listUsers(ctx)
	if err != nil {
		return err
	}
	xrayConfig, err := xrayServerConfig(users)
	if err != nil {
		return err
	}
	xrayTarget := env("VPN_XRAY_CANDIDATE", "/var/lib/go-sqlite-api/xray-config.json")
	for _, item := range []struct {
		path    string
		content string
	}{
		{xrayTarget, xrayConfig},
	} {
		if err := os.MkdirAll(filepath.Dir(item.path), 0750); err != nil {
			return err
		}
		if err := os.WriteFile(item.path, []byte(item.content), 0600); err != nil {
			return err
		}
	}
	// Keep sing-box JSON around for legacy tooling; server now runs xray.
	if legacy, err := singBoxServerConfig(users); err == nil {
		legacyTarget := env("VPN_SING_BOX_CANDIDATE", "/var/lib/go-sqlite-api/sing-box-config.json")
		_ = os.WriteFile(legacyTarget, []byte(legacy), 0600)
	}
	cmd := exec.CommandContext(ctx, "sudo", "/usr/local/sbin/vpn-admin-apply")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apply vpn server config: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type vpnUserScanner interface {
	Scan(dest ...any) error
}

func scanVPNUser(scanner vpnUserScanner) (vpnUser, error) {
	var user vpnUser
	var enabled int
	var createdAt, updatedAt string
	if err := scanner.Scan(&user.ID, &user.Name, &user.UUID, &enabled, &user.Note, &user.LoginEmail, &user.Role, &createdAt, &updatedAt); err != nil {
		return vpnUser{}, err
	}
	user.Enabled = enabled == 1
	user.CreatedAt = parseSQLiteTime(createdAt)
	user.UpdatedAt = parseSQLiteTime(updatedAt)
	return user, nil
}

func (u vpnUser) sessionUser() sessionUser {
	return sessionUser{ID: u.ID, Name: u.Name, LoginEmail: u.LoginEmail, Role: u.Role}
}

func normalizeRole(role string) string {
	if strings.EqualFold(strings.TrimSpace(role), "admin") {
		return "admin"
	}
	return "user"
}

func (m vpnManager) getUserByEmail(ctx context.Context, email string) (vpnUser, string, error) {
	var user vpnUser
	var enabled int
	var createdAt, updatedAt, passwordHash string
	err := m.db.QueryRowContext(ctx, `
SELECT id, name, uuid, enabled, note, login_email, role, password_hash, created_at, updated_at
FROM vpn_users
WHERE login_email = ? AND login_email != '';
`, email).Scan(&user.ID, &user.Name, &user.UUID, &enabled, &user.Note, &user.LoginEmail, &user.Role, &passwordHash, &createdAt, &updatedAt)
	if err != nil {
		return vpnUser{}, "", err
	}
	user.Enabled = enabled == 1
	user.CreatedAt = parseSQLiteTime(createdAt)
	user.UpdatedAt = parseSQLiteTime(updatedAt)
	return user, passwordHash, nil
}

func (m vpnManager) createSession(ctx context.Context, userID int64) (string, time.Time, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	_, err = m.db.ExecContext(ctx, `
INSERT INTO vpn_sessions (token, user_id, expires_at)
VALUES (?, ?, ?);
`, token, userID, expires.Format(time.RFC3339))
	return token, expires, err
}

func (m vpnManager) deleteSession(ctx context.Context, token string) error {
	_, err := m.db.ExecContext(ctx, `DELETE FROM vpn_sessions WHERE token = ?`, token)
	return err
}

func (m vpnManager) getSessionUser(ctx context.Context, token string) (sessionUser, error) {
	var user sessionUser
	var expiresAt string
	err := m.db.QueryRowContext(ctx, `
SELECT u.id, u.name, u.login_email, u.role, s.expires_at
FROM vpn_sessions s
JOIN vpn_users u ON u.id = s.user_id
WHERE s.token = ? AND u.enabled = 1;
`, token).Scan(&user.ID, &user.Name, &user.LoginEmail, &user.Role, &expiresAt)
	if err != nil {
		return sessionUser{}, err
	}
	expires := parseSQLiteTime(expiresAt)
	if !expires.IsZero() && time.Now().UTC().After(expires) {
		_ = m.deleteSession(ctx, token)
		return sessionUser{}, sql.ErrNoRows
	}
	return user, nil
}

func vpnUserIDFromPath(path string) (int64, error) {
	part := strings.TrimPrefix(path, "/api/vpn/users/")
	part = strings.Trim(part, "/")
	if part == "" {
		return 0, errors.New("id is required")
	}
	return strconv.ParseInt(part, 10, 64)
}

func vpnConfigPath(path string) (int64, string, error) {
	part := strings.TrimPrefix(path, "/api/vpn/configs/")
	pieces := strings.Split(strings.Trim(part, "/"), "/")
	if len(pieces) != 2 {
		return 0, "", errors.New("expected /api/vpn/configs/{id}/{kind}")
	}
	id, err := strconv.ParseInt(pieces[0], 10, 64)
	if err != nil {
		return 0, "", err
	}
	return id, pieces[1], nil
}

func vpnPublicConfigPath(path string) (string, string, error) {
	part := strings.TrimPrefix(path, "/api/vpn/public/")
	pieces := strings.Split(strings.Trim(part, "/"), "/")
	if len(pieces) != 2 {
		return "", "", errors.New("expected /api/vpn/public/{uuid}/{kind}")
	}
	if strings.TrimSpace(pieces[0]) == "" || strings.TrimSpace(pieces[1]) == "" {
		return "", "", errors.New("uuid and kind are required")
	}
	return pieces[0], pieces[1], nil
}

func rocketProfileUUIDFromPath(path string) (string, error) {
	part := strings.TrimPrefix(path, "/api/rocket/profiles/")
	part = strings.Trim(part, "/")
	part = strings.TrimSuffix(part, ".json")
	if strings.TrimSpace(part) == "" {
		return "", errors.New("profile id is required")
	}
	return part, nil
}

func singBoxImportURL(r *http.Request, user vpnUser) string {
	profileURL := externalBaseURL(r) + "/api/vpn/public/" + user.UUID + "/sing-box.json"
	return "sing-box://import-remote-profile?url=" + url.QueryEscape(profileURL) + "#" + url.QueryEscape(user.Name)
}

func rocketImportURL(r *http.Request, user vpnUser) string {
	profileURL := externalBaseURL(r) + "/api/rocket/profiles/" + user.UUID + ".json"
	values := url.Values{}
	values.Set("url", profileURL)
	values.Set("token", rocketProfileToken(user.UUID))
	return "clgrocket://profile?" + values.Encode()
}

func clashImportURL(r *http.Request, user vpnUser) string {
	profileURL := externalBaseURL(r) + "/api/vpn/public/" + user.UUID + "/clash-android.yaml"
	return "clashmeta://install-config?url=" + url.QueryEscape(profileURL)
}

func clashAndroidImportURL(r *http.Request, user vpnUser) string {
	return clashImportURL(r, user)
}

func externalBaseURL(r *http.Request) string {
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = "http"
		if r.TLS != nil {
			proto = "https"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return proto + "://" + host
}

func currentVPNRuntime() vpnRuntime {
	return vpnRuntime{
		ServerHost: env("VPN_SERVER_HOST", "127.0.0.1"),
		Port:       envInt("VPN_SERVER_PORT", 443),
		SNI:        env("VPN_REALITY_SNI", "www.cloudflare.com"),
		PublicKey:  env("VPN_REALITY_PUBLIC_KEY", ""),
		ShortID:    env("VPN_REALITY_SHORT_ID", ""),
	}
}

func validateVPNRuntime(rt vpnRuntime) error {
	host := strings.TrimSpace(rt.ServerHost)
	if host == "" || host == "127.0.0.1" || host == "localhost" {
		return errors.New("VPN_SERVER_HOST is not configured; check /etc/go-sqlite-api/vpn.env")
	}
	if strings.TrimSpace(rt.PublicKey) == "" {
		return errors.New("VPN_REALITY_PUBLIC_KEY is not configured; check /etc/go-sqlite-api/vpn.env")
	}
	if strings.TrimSpace(rt.ShortID) == "" {
		return errors.New("VPN_REALITY_SHORT_ID is not configured; check /etc/go-sqlite-api/vpn.env")
	}
	return nil
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func sanitizeUserName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func generateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexed[0:8], hexed[8:12], hexed[12:16], hexed[16:20], hexed[20:32]), nil
}

func randomHex(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func hashPassword(password string) (string, error) {
	salt, err := randomHex(16)
	if err != nil {
		return "", err
	}
	sum := passwordDigest(password, salt)
	return "sha256$" + salt + "$" + base64.StdEncoding.EncodeToString(sum), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := passwordDigest(password, parts[1])
	return hmac.Equal(got, want)
}

func passwordDigest(password, salt string) []byte {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(password))
	sum := mac.Sum(nil)
	for i := 0; i < 10000; i++ {
		mac = hmac.New(sha256.New, []byte(password))
		mac.Write(sum)
		sum = mac.Sum(nil)
	}
	return sum
}

func singBoxServerConfig(users []vpnUser) (string, error) {
	privateKey := strings.TrimSpace(env("VPN_REALITY_PRIVATE_KEY", ""))
	if privateKey == "" {
		return "", errors.New("VPN_REALITY_PRIVATE_KEY is required")
	}
	rt := currentVPNRuntime()
	type serverUser struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
		Flow string `json:"flow,omitempty"`
	}
	enabled := make([]serverUser, 0)
	for _, user := range users {
		if user.Enabled {
			enabled = append(enabled, serverUser{Name: user.Name, UUID: user.UUID, Flow: vpnUserFlow(user)})
		}
	}
	if len(enabled) == 0 {
		return "", errors.New("at least one enabled user is required")
	}
	config := map[string]any{
		"log": map[string]any{
			"level":     "info",
			"timestamp": true,
			"output":    "/var/lib/sing-box/access.log",
		},
		"inbounds": []any{
			map[string]any{
				"type":        "vless",
				"tag":         vpnInboundTag,
				"listen":      "0.0.0.0",
				"listen_port": rt.Port,
				"users":       enabled,
				"tls": map[string]any{
					"enabled":     true,
					"server_name": rt.SNI,
					"reality": map[string]any{
						"enabled": true,
						"handshake": map[string]any{
							"server":      rt.SNI,
							"server_port": 443,
						},
						"private_key": privateKey,
						"short_id":    []string{rt.ShortID},
					},
				},
			},
		},
		"outbounds": []any{
			map[string]any{"type": "direct", "tag": "direct"},
		},
	}
	if listen, secret, ok := clashAPISettings(); ok {
		config["experimental"] = map[string]any{
			"clash_api": map[string]any{
				"external_controller": listen,
				"secret":              secret,
			},
		}
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func vlessURL(user vpnUser, rt vpnRuntime) string {
	values := []string{
		"encryption=none",
		"security=reality",
		"sni=" + rt.SNI,
		"fp=chrome",
		"pbk=" + rt.PublicKey,
		"sid=" + rt.ShortID,
		"type=tcp",
	}
	if flow := vpnUserFlow(user); flow != "" {
		values = append(values[:1], append([]string{"flow=" + flow}, values[1:]...)...)
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, rt.ServerHost, rt.Port, strings.Join(values, "&"), user.Name)
}

func clashBypassRules(serverHost string) string {
	host := strings.TrimSpace(serverHost)
	if host == "" {
		return ""
	}
	return fmt.Sprintf(`  - IP-CIDR,%s/32,DIRECT,no-resolve
  - IP-CIDR,127.0.0.0/8,DIRECT,no-resolve
  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
  - IP-CIDR,172.16.0.0/12,DIRECT,no-resolve
  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve
  - IP-CIDR,169.254.0.0/16,DIRECT,no-resolve
`, host)
}

func clashForeignDomainRules() string {
	return `  - DOMAIN-SUFFIX,google.com,PROXY
  - DOMAIN-SUFFIX,googleapis.com,PROXY
  - DOMAIN-SUFFIX,gstatic.com,PROXY
  - DOMAIN-SUFFIX,ggpht.com,PROXY
  - DOMAIN-SUFFIX,googlevideo.com,PROXY
  - DOMAIN-SUFFIX,youtube.com,PROXY
  - DOMAIN-SUFFIX,ytimg.com,PROXY
  - DOMAIN-SUFFIX,facebook.com,PROXY
  - DOMAIN-SUFFIX,twitter.com,PROXY
  - DOMAIN-SUFFIX,x.com,PROXY
  - DOMAIN-SUFFIX,instagram.com,PROXY
  - DOMAIN-SUFFIX,github.com,PROXY
  - DOMAIN-SUFFIX,openai.com,PROXY
  - DOMAIN-SUFFIX,chatgpt.com,PROXY
`
}

func clashVlessProxy(user vpnUser, rt vpnRuntime, fingerprint string, includePacketEncoding bool) string {
	flowLine := ""
	packetEncodingLine := ""
	if flow := vpnUserFlow(user); flow != "" {
		flowLine = fmt.Sprintf("    flow: %s\n", flow)
		if includePacketEncoding {
			packetEncodingLine = "    packet-encoding: xudp\n"
		}
	}
	return fmt.Sprintf(`  - name: %s
    type: vless
    server: %s
    port: %d
    uuid: %s
    udp: true
    encryption: none
%s%s    tls: true
    servername: %s
    client-fingerprint: %s
    skip-cert-verify: true
    reality-opts:
      public-key: %s
      short-id: %s
    network: tcp
`, user.Name, rt.ServerHost, rt.Port, user.UUID, flowLine, packetEncodingLine, rt.SNI, fingerprint, rt.PublicKey, rt.ShortID)
}

func clashConfig(user vpnUser, rt vpnRuntime) string {
	bypassRules := clashBypassRules(rt.ServerHost)
	dnsPolicyLine := ""
	fakeIPFilterLine := ""
	if host := strings.TrimSpace(rt.ServerHost); host != "" {
		dnsPolicyLine = fmt.Sprintf("    '%s': system\n", host)
		fakeIPFilterLine = fmt.Sprintf("    - '%s'\n", host)
	}
	return fmt.Sprintf(`mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
tcp-concurrent: false
connect-timeout: 15000
keep-alive-interval: 15
unified-delay: false

profile:
  store-selected: true

dns:
  enable: true
  enhanced-mode: fake-ip
  default-nameserver:
    - system
    - 223.5.5.5
    - 119.29.29.29
  fake-ip-filter:
    - '+.lan'
%s  nameserver:
    - 223.5.5.5
    - 119.29.29.29
  nameserver-policy:
%s
proxies:
%s
proxy-groups:
%s

rules:
%s  - GEOIP,CN,DIRECT
  - MATCH,PROXY
`, fakeIPFilterLine, dnsPolicyLine, clashVlessProxy(user, rt, "chrome", false), clashProxyGroup(user), bypassRules)
}

func clashAndroidConfig(user vpnUser, rt vpnRuntime) string {
	bypassRules := clashBypassRules(rt.ServerHost)
	foreignRules := clashForeignDomainRules()
	serverPolicyLine := ""
	if host := strings.TrimSpace(rt.ServerHost); host != "" {
		serverPolicyLine = fmt.Sprintf("    '%s': system\n", host)
	}
	return fmt.Sprintf(`mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
tcp-concurrent: false
connect-timeout: 10000
keep-alive-interval: 15
unified-delay: false
geodata-mode: true
geo-auto-update: true
geox-url:
  geoip: "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat"
  geosite: "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat"
  mmdb: "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/country.mmdb"

profile:
  store-selected: true

dns:
  enable: true
  ipv6: false
  enhanced-mode: redir-host
  default-nameserver:
    - 223.5.5.5
    - 119.29.29.29
  nameserver:
    - https://dns.alidns.com/dns-query
    - 223.5.5.5
  fallback:
    - https://1.1.1.1/dns-query
    - https://dns.google/dns-query
  fallback-filter:
    geoip: true
    geoip-code: CN
    geosite:
      - gfw
  nameserver-policy:
%s
proxies:
%s
proxy-groups:
%s

rules:
%s%s  - GEOIP,private,DIRECT,no-resolve
  - GEOSITE,cn,DIRECT
  - GEOIP,CN,DIRECT,no-resolve
  - GEOSITE,gfw,PROXY
  - MATCH,PROXY
`, serverPolicyLine, clashVlessProxy(user, rt, "chrome", true), clashProxyGroup(user), bypassRules, foreignRules)
}

func singBoxChinaDirectDomains() []string {
	return []string{
		".cn",
		".baidu.com",
		".qq.com",
		".weixin.qq.com",
		".taobao.com",
		".alipay.com",
		".bilibili.com",
		".163.com",
		".126.com",
	}
}

func singBoxForeignProxyDomains() []string {
	return []string{
		".google.com",
		".googleapis.com",
		".googlevideo.com",
		".gstatic.com",
		".ggpht.com",
		".youtube.com",
		".ytimg.com",
		".facebook.com",
		".twitter.com",
		".x.com",
		".instagram.com",
		".github.com",
		".openai.com",
		".chatgpt.com",
	}
}

func singBoxClientConfig(user vpnUser, rt vpnRuntime) string {
	tlsBlock := map[string]any{
		"enabled":     true,
		"server_name": rt.SNI,
		"utls":        map[string]any{"enabled": true, "fingerprint": "chrome"},
		"reality": map[string]any{
			"enabled":    true,
			"public_key": rt.PublicKey,
			"short_id":   rt.ShortID,
		},
	}
	vlessOutbound := map[string]any{
		"type":        "vless",
		"tag":         user.Name,
		"server":      rt.ServerHost,
		"server_port": rt.Port,
		"uuid":        user.UUID,
		"tls":         tlsBlock,
	}
	if flow := vpnUserFlow(user); flow != "" {
		vlessOutbound["flow"] = flow
		vlessOutbound["packet_encoding"] = "xudp"
	}
	config := map[string]any{
		"log": map[string]any{"level": "info", "timestamp": true},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"type": "local", "tag": "local"},
				map[string]any{
					"type":   "https",
					"tag":    "dns-proxy",
					"server": "1.1.1.1",
					"detour": user.Name,
				},
			},
			"rules": []any{
				map[string]any{
					"domain_suffix": singBoxForeignProxyDomains(),
					"server":        "dns-proxy",
				},
			},
			"final":    "local",
			"strategy": "ipv4_only",
		},
		"inbounds": []any{
			map[string]any{
				"type":                  "tun",
				"tag":                   "tun-in",
				"address":               []string{"172.19.0.1/30"},
				"auto_route":            true,
				"strict_route":          true,
				"auto_redirect":         true,
				"mtu":                   1400,
				"stack":                 "gvisor",
				"route_exclude_address": []string{rt.ServerHost + "/32"},
				"exclude_package":       []string{"io.nekohasekai.sfa"},
			},
		},
		"outbounds": []any{
			vlessOutbound,
			map[string]any{"type": "direct", "tag": "direct"},
		},
		"route": map[string]any{
			"default_domain_resolver": "local",
			"rules": []any{
				map[string]any{"inbound": "tun-in", "action": "sniff"},
				map[string]any{"protocol": "dns", "action": "hijack-dns"},
				map[string]any{"ip_cidr": []string{rt.ServerHost + "/32"}, "outbound": "direct"},
				map[string]any{"ip_is_private": true, "outbound": "direct"},
				map[string]any{"domain_suffix": singBoxChinaDirectDomains(), "outbound": "direct"},
				map[string]any{"domain_suffix": singBoxForeignProxyDomains(), "outbound": user.Name},
			},
			"final":                 user.Name,
			"auto_detect_interface": false,
		},
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	return string(data) + "\n"
}

func vpnUserFlow(user vpnUser) string {
	note := strings.ToLower(user.Note)
	if strings.Contains(note, "noflow") || strings.Contains(note, "compat-noflow") {
		return ""
	}
	return vpnFlow
}

func rocketProfile(user vpnUser, rt vpnRuntime) (map[string]any, error) {
	var singBox map[string]any
	if err := json.Unmarshal([]byte(singBoxClientConfig(user, rt)), &singBox); err != nil {
		return nil, err
	}
	return map[string]any{
		"version":    1,
		"profile_id": user.UUID,
		"name":       user.Name,
		"updated_at": user.UpdatedAt.Format(time.RFC3339),
		"core":       "sing-box",
		"sing_box":   singBox,
	}, nil
}

func writeRocketProfile(w http.ResponseWriter, user vpnUser, rt vpnRuntime) error {
	artifact, err := rocketConfigArtifact(user, rt)
	if err != nil {
		return err
	}
	w.Header().Set("X-VPN-Config-Version", artifact.Version)
	w.Header().Set("X-VPN-Config-Checksum", artifact.Checksum)
	w.Header().Set("X-VPN-Config-Template-Revision", vpnConfigTemplateRevision)
	_, err = fmt.Fprint(w, artifact.Content)
	return err
}

func rocketProfileToken(uuid string) string {
	secret := env("ROCKET_PROFILE_SECRET", env("VPN_ADMIN_PASSWORD", "change-me-before-use"))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("clgrocket:" + uuid))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyRocketProfileToken(uuid, token string) bool {
	expected := rocketProfileToken(uuid)
	return hmac.Equal([]byte(expected), []byte(token))
}

type interfaceTraffic struct {
	Interface string `json:"interface"`
	RXBytes   uint64 `json:"rx_bytes"`
	TXBytes   uint64 `json:"tx_bytes"`
}

type trafficPoint struct {
	Time    time.Time `json:"time"`
	RXBytes uint64    `json:"rx_bytes"`
	TXBytes uint64    `json:"tx_bytes"`
}

type trafficHistory struct {
	Bucket     string         `json:"bucket"`
	Interval   int64          `json:"interval_seconds"`
	Interface  string         `json:"interface"`
	Points     []trafficPoint `json:"points"`
	PointsNote string         `json:"points_note"`
}

func readInterfaceTraffic(name string) (interfaceTraffic, error) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return interfaceTraffic{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, name+":") {
			continue
		}
		fields := strings.Fields(strings.Replace(line, ":", " ", 1))
		if len(fields) < 17 {
			return interfaceTraffic{}, errors.New("unexpected /proc/net/dev format")
		}
		rx, _ := strconv.ParseUint(fields[1], 10, 64)
		tx, _ := strconv.ParseUint(fields[9], 10, 64)
		return interfaceTraffic{Interface: name, RXBytes: rx, TXBytes: tx}, nil
	}
	return interfaceTraffic{Interface: name}, fmt.Errorf("interface %s not found", name)
}

func (m vpnManager) recordTrafficSample(ctx context.Context, traffic interfaceTraffic) error {
	if traffic.Interface == "" {
		return nil
	}
	now := time.Now().UTC()
	_, err := m.db.ExecContext(ctx, `
INSERT INTO vpn_traffic_samples (interface, rx_bytes, tx_bytes, sampled_at)
VALUES (?, ?, ?, ?);
`, traffic.Interface, traffic.RXBytes, traffic.TXBytes, now.Format(time.RFC3339))
	if err != nil {
		return err
	}
	_, _ = m.db.ExecContext(ctx, `DELETE FROM vpn_traffic_samples WHERE sampled_at < ?`, now.AddDate(0, 0, -35).Format(time.RFC3339))
	return nil
}

func (m vpnManager) trafficHistory(ctx context.Context, iface, bucket string) (trafficHistory, error) {
	bucket, interval, window, count := trafficBucket(bucket)
	now := time.Now().UTC()
	end := trafficBucketStart(now, interval).Add(interval)
	start := end.Add(-time.Duration(count) * interval)
	rows, err := m.db.QueryContext(ctx, `
SELECT rx_bytes, tx_bytes, sampled_at
FROM vpn_traffic_samples
WHERE interface = ? AND sampled_at >= ?
ORDER BY sampled_at ASC;
`, iface, start.Add(-interval).Format(time.RFC3339))
	if err != nil {
		return trafficHistory{}, err
	}
	defer rows.Close()

	type rawSample struct {
		rx uint64
		tx uint64
		at time.Time
	}
	samples := make([]rawSample, 0)
	for rows.Next() {
		var rx, tx uint64
		var sampledAt string
		if err := rows.Scan(&rx, &tx, &sampledAt); err != nil {
			return trafficHistory{}, err
		}
		samples = append(samples, rawSample{rx: rx, tx: tx, at: parseSQLiteTime(sampledAt)})
	}
	if err := rows.Err(); err != nil {
		return trafficHistory{}, err
	}

	points := make([]trafficPoint, 0, count)
	index := make(map[int64]int, count)
	for i := 0; i < count; i++ {
		t := start.Add(time.Duration(i) * interval)
		index[t.Unix()] = len(points)
		points = append(points, trafficPoint{Time: t})
	}
	for i := 1; i < len(samples); i++ {
		prev := samples[i-1]
		cur := samples[i]
		if cur.at.Before(start) || cur.rx < prev.rx || cur.tx < prev.tx {
			continue
		}
		key := trafficBucketStart(cur.at, interval).Unix()
		pos, ok := index[key]
		if !ok {
			continue
		}
		points[pos].RXBytes += cur.rx - prev.rx
		points[pos].TXBytes += cur.tx - prev.tx
	}
	return trafficHistory{
		Bucket:     bucket,
		Interval:   int64(interval.Seconds()),
		Interface:  iface,
		Points:     points,
		PointsNote: fmt.Sprintf("按%s聚合最近%s的服务器网卡总流量；用户级精确统计需要 sing-box V2Ray API 或 Xray/面板支持。", trafficBucketLabel(bucket), window),
	}, nil
}

func emptyTrafficHistory(bucket, iface, note string) trafficHistory {
	bucket, interval, _, count := trafficBucket(bucket)
	now := time.Now().UTC()
	end := trafficBucketStart(now, interval).Add(interval)
	start := end.Add(-time.Duration(count) * interval)
	points := make([]trafficPoint, 0, count)
	for i := 0; i < count; i++ {
		points = append(points, trafficPoint{Time: start.Add(time.Duration(i) * interval)})
	}
	return trafficHistory{
		Bucket:     bucket,
		Interval:   int64(interval.Seconds()),
		Interface:  iface,
		Points:     points,
		PointsNote: note,
	}
}

func trafficBucket(bucket string) (string, time.Duration, string, int) {
	switch bucket {
	case "6h":
		return "6h", 6 * time.Hour, "7天", 28
	case "day":
		return "day", 24 * time.Hour, "30天", 30
	default:
		return "hour", time.Hour, "24小时", 24
	}
}

func trafficBucketLabel(bucket string) string {
	switch bucket {
	case "6h":
		return "6小时"
	case "day":
		return "天"
	default:
		return "小时"
	}
}

func trafficBucketStart(t time.Time, interval time.Duration) time.Time {
	t = t.UTC()
	if interval == 6*time.Hour {
		hour := (t.Hour() / 6) * 6
		return time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, time.UTC)
	}
	if interval == 24*time.Hour {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	return t.Truncate(interval)
}
