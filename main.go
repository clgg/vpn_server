package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type app struct {
	db      *sql.DB
	version string
	vpn     vpnManager
}

type item struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func main() {
	dbPath := env("DB_PATH", "/var/lib/go-sqlite-api/app.db")
	addr := env("ADDR", "127.0.0.1:8080")
	version := env("VERSION", "dev")

	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatal(err)
	}

	a := &app{
		db:      db,
		version: version,
		vpn:     newVPNManager(db),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.index)
	mux.HandleFunc("GET /vpn-admin", a.vpnAdminPage)
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("GET /api/version", a.versionInfo)
	mux.HandleFunc("GET /api/auth/me", a.authMe)
	mux.HandleFunc("POST /api/auth/login", a.authLogin)
	mux.HandleFunc("POST /api/auth/logout", a.authLogout)
	mux.HandleFunc("GET /api/items", a.listItems)
	mux.HandleFunc("POST /api/items", a.createItem)
	mux.HandleFunc("PATCH /api/items/", a.updateItem)
	mux.HandleFunc("DELETE /api/items/", a.deleteItem)
	mux.HandleFunc("GET /api/vpn/status", a.requireLogin(a.vpnStatus))
	mux.HandleFunc("GET /api/vpn/users", a.requireLogin(a.vpnUsers))
	mux.HandleFunc("POST /api/vpn/users", a.requireAdmin(a.vpnCreateUser))
	mux.HandleFunc("PATCH /api/vpn/users/", a.requireAdmin(a.vpnUpdateUser))
	mux.HandleFunc("DELETE /api/vpn/users/", a.requireAdmin(a.vpnDeleteUser))
	mux.HandleFunc("POST /api/vpn/apply", a.requireAdmin(a.vpnApply))
	mux.HandleFunc("GET /api/vpn/traffic", a.requireLogin(a.vpnTrafficHistory))
	mux.HandleFunc("PATCH /api/vpn/devices/", a.requireLogin(a.vpnUpdateDevice))
	mux.HandleFunc("GET /api/vpn/configs/", a.requireLogin(a.vpnConfig))
	mux.HandleFunc("POST /api/vpn/configs/", a.requireLogin(a.vpnConfigPost))
	mux.HandleFunc("GET /api/vpn/public/", a.vpnPublicConfig)
	mux.HandleFunc("GET /api/rocket/profiles/", a.rocketProfile)
	mux.HandleFunc("OPTIONS /", options)
	mux.HandleFunc("OPTIONS /api/", options)

	server := &http.Server{
		Addr:              addr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("go-sqlite-api listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	done INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)
	if err != nil {
		return err
	}
	return migrateVPN(db)
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	if err := a.db.PingContext(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"db":     "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *app) versionInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "go-sqlite-api",
		"version": a.version,
	})
}

func (a *app) listItems(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `
SELECT id, title, done, created_at, updated_at
FROM items
ORDER BY id DESC
LIMIT 100;
`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	items := make([]item, 0)
	for rows.Next() {
		var it item
		var done int
		var createdAt, updatedAt string
		if err := rows.Scan(&it.ID, &it.Title, &done, &createdAt, &updatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		it.Done = done == 1
		it.CreatedAt = parseSQLiteTime(createdAt)
		it.UpdatedAt = parseSQLiteTime(updatedAt)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *app) createItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}

	res, err := a.db.ExecContext(r.Context(), `INSERT INTO items (title) VALUES (?)`, req.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	id, _ := res.LastInsertId()
	it, err := a.getItem(r, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, it)
}

func (a *app) updateItem(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req struct {
		Title *string `json:"title"`
		Done  *bool   `json:"done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Title == nil && req.Done == nil {
		writeError(w, http.StatusBadRequest, errors.New("title or done is required"))
		return
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			writeError(w, http.StatusBadRequest, errors.New("title cannot be empty"))
			return
		}
		_, err = a.db.ExecContext(r.Context(), `UPDATE items SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, title, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	if req.Done != nil {
		done := 0
		if *req.Done {
			done = 1
		}
		_, err = a.db.ExecContext(r.Context(), `UPDATE items SET done = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, done, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	it, err := a.getItem(r, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (a *app) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	res, err := a.db.ExecContext(r.Context(), `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("item %d not found", id))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) getItem(r *http.Request, id int64) (item, error) {
	var it item
	var done int
	var createdAt, updatedAt string
	err := a.db.QueryRowContext(r.Context(), `
SELECT id, title, done, created_at, updated_at FROM items WHERE id = ?;
`, id).Scan(&it.ID, &it.Title, &done, &createdAt, &updatedAt)
	if err != nil {
		return item{}, err
	}
	it.Done = done == 1
	it.CreatedAt = parseSQLiteTime(createdAt)
	it.UpdatedAt = parseSQLiteTime(updatedAt)
	return it, nil
}

func idFromPath(path string) (int64, error) {
	part := strings.TrimPrefix(path, "/api/items/")
	part = strings.Trim(part, "/")
	if part == "" {
		return 0, errors.New("id is required")
	}
	return strconv.ParseInt(part, 10, 64)
}

func parseSQLiteTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func options(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
