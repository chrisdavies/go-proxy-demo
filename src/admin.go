// This file contains the proxy-admin endpoints served at `/goproxy/`

package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// adminService controls the management of proxy and redirect rules
type adminService struct {
	*Config
	db *sql.DB
}

var (
	// Rudimentary domain validation regular expression
	domainReg = regexp.MustCompile("^[a-z\\-_0-9.]+$")

	// Validate a prefix looks like {domain}/{type}/{numeric-id}
	prefixReg = regexp.MustCompile("^[a-z\\-_0-9.]+\\/[a-z\\-_0-9]+\\/[0-9]+$")
)

// writeJsonError writes a basic json error to the writer
func writeJsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		w.Write([]byte(`{"message":"An unknown error occurred."}`))
		return
	}
	w.Write(b)
}

// writeJsonOK writes an empty json response
func writeJsonOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

// migrateUp migrates the SQLite database
func migrateUp(db *sql.DB) error {
	_, err := db.Exec(`
		BEGIN;
		CREATE TABLE IF NOT EXISTS proxy_rules (
			domain  TEXT NOT NULL PRIMARY KEY,
			version TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS redirect_rules (
			prefix     TEXT NOT NULL PRIMARY KEY,
			to_url     TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		COMMIT;
	`)
	return err
}

// periodicOptimize runs SQLite optimizations every hour just to make sure the
// db stays relatively healthy.
func periodicOptimize(db *sql.DB) {
	for range time.Tick(time.Hour) {
		log.Println("[sqlite-optimize][start]")
		// Limit the depth of optimization analysis that SQLite will perform when
		// we call optimize.
		if _, err := db.Exec(`PRAGMA analysis_limit=500`); err != nil {
			log.Println("[error][sqlite-optimize]", err)
		}
		if _, err := db.Exec(`PRAGMA optimize`); err != nil {
			log.Println("[error][sqlite-optimize]", err)
		}
		log.Println("[sqlite-optimize][done]")
	}
}

// AdminService constructs a new instance of the admin service
func AdminService(config *Config) (*adminService, error) {
	db, err := sql.Open("sqlite3", "./goproxy.sqlite?_timeout=5000&_journal=WAL&_sync=1")
	if err != nil {
		return nil, err
	}
	if err = migrateUp(db); err != nil {
		db.Close()
		return nil, err
	}

	go periodicOptimize(db)

	result := &adminService{
		Config: config,
		db:     db,
	}
	return result, nil
}

// authorize verifies that the request's Authorization header contains the API key
func (admin *adminService) authorize(w http.ResponseWriter, r *http.Request) bool {
	if admin.APIKey == r.Header.Get("Authorization") {
		return true
	}

	writeJsonError(w, http.StatusForbidden, "Invalid Authorization header.")
	return false
}

// validMethod verifies that the requested HTTP method is supported
func (admin *adminService) validMethod(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, method := range methods {
		if r.Method == method {
			return true
		}
	}

	writeJsonError(w, http.StatusMethodNotAllowed, "Unsupported HTTP method.")
	return false
}

// isValidDomain verifies that the specified domain is roughly valid
func isValidDomain(domain string) bool {
	return domainReg.MatchString(domain)
}

// isValidRedirectPrefix verifies that the redirect prefix is roughly valid
func isValidRedirectPrefix(prefix string) bool {
	return prefixReg.MatchString(prefix)
}

// hostname returns the host without port, as URL.Hostname() always returns ""
// for some reason.
func hostname(r *http.Request) string {
	return strings.SplitN(r.Host, ":", 2)[0]
}

// GetProxyRule fetches the proxy rule for the specified request. If none is
// found, returns an empty string. If one is found, returns the appropriate
// domain to which the request should be proxied.
func (admin *adminService) GetProxyRule(r *http.Request) (string, error) {
	var version string
	row := admin.db.QueryRow(`
		SELECT version FROM proxy_rules WHERE domain=@domain
	`, sql.Named("domain", hostname(r)))

	err := row.Scan(&version)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if version == "v1" {
		return admin.V1Host, nil
	}
	if version == "v2" {
		return admin.V2Host, nil
	}
	return "", nil
}

// GetRedirectRule fetches the redirect rule for the specified request. If none is
// found, returns an empty string. If one is found, returns the appropriate
// url to which the request should be redirected.
func (admin *adminService) GetRedirectRule(r *http.Request) (string, error) {
	var toUrl string
	prefix := prefixReg.FindString(hostname(r) + r.URL.Path)
	if prefix == "" {
		return "", nil
	}

	row := admin.db.QueryRow(`
		SELECT to_url FROM redirect_rules WHERE prefix=@prefix
	`, sql.Named("prefix", prefix))

	err := row.Scan(&toUrl)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return toUrl, nil
}

// ProxyRules handles the add / remove proxy rules API endpoint
func (admin *adminService) ProxyRules(w http.ResponseWriter, r *http.Request) {
	if !admin.authorize(w, r) || !admin.validMethod(w, r, http.MethodPost, http.MethodDelete) {
		return
	}

	query := r.URL.Query()
	domain := query.Get("domain")
	version := query.Get("version")

	if !isValidDomain(domain) {
		writeJsonError(w, http.StatusBadRequest, "Query param domain is required")
		return
	}

	if r.Method == http.MethodPost && version != "v1" && version != "v2" {
		writeJsonError(w, http.StatusBadRequest, "Query param version must be v1 or v2")
		return
	}

	if r.Method == http.MethodPost {
		admin.db.Exec(`
			INSERT INTO proxy_rules (domain, version, updated_at)
			VALUES (@domain, @version, @updated_at)
		`,
			sql.Named("domain", domain),
			sql.Named("version", version),
			sql.Named("updated_at", time.Now().Format(time.RFC3339)),
		)
	} else {
		admin.db.Exec(`
			DELETE FROM proxy_rules
			WHERE domain=@domain
		`,
			sql.Named("domain", domain),
		)
	}

	writeJsonOK(w)
}

// RedirectRules handles the add / remove redirect rules API endpoint
func (admin *adminService) RedirectRules(w http.ResponseWriter, r *http.Request) {
	if !admin.authorize(w, r) || !admin.validMethod(w, r, http.MethodPost, http.MethodDelete) {
		return
	}

	query := r.URL.Query()
	prefix := query.Get("prefix")
	toUrl := query.Get("to-url")

	if r.Method == http.MethodPost {
		if _, err := url.ParseRequestURI(toUrl); err != nil {
			writeJsonError(w, http.StatusBadRequest, "Invalid to-url")
			return
		}
	}

	if !isValidRedirectPrefix(prefix) {
		writeJsonError(w, http.StatusBadRequest, "Prefix must look like {domain}/{type}/{number}")
		return
	}

	if r.Method == http.MethodPost {
		admin.db.Exec(`
			INSERT INTO redirect_rules (prefix, to_url, updated_at)
			VALUES (@prefix, @to_url, @updated_at)
		`,
			sql.Named("prefix", prefix),
			sql.Named("to_url", toUrl),
			sql.Named("updated_at", time.Now().Format(time.RFC3339)),
		)
	} else {
		admin.db.Exec(`
			DELETE FROM redirect_rules
			WHERE prefix=@prefix
		`,
			sql.Named("prefix", prefix),
		)
	}

	writeJsonOK(w)
}
