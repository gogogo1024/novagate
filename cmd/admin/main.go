package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogogo1024/novagate/internal/admin"
)

func main() {
	// Defensive: prevent test binary from starting
	if strings.HasSuffix(filepath.Base(os.Args[0]), ".test") {
		return
	}

	var (
		addr         = flag.String("addr", ":8888", "admin service address")
		redisAddr    = flag.String("redis", "localhost:6379", "redis address")
		milvusAddr   = flag.String("milvus", "localhost:19530", "milvus address")
		readTimeout  = flag.Duration("read-timeout", 10*time.Second, "read timeout")
		writeTimeout = flag.Duration("write-timeout", 10*time.Second, "write timeout")
		idleTimeout  = flag.Duration("idle-timeout", 60*time.Second, "idle timeout")
	)
	flag.Parse()

	// Create admin service
	adminSvc, err := admin.NewService(*redisAddr, *milvusAddr)
	if err != nil {
		log.Fatalf("failed to create admin service: %v", err)
	}

	// Create router
	mux := http.NewServeMux()
	registerRoutes(mux, adminSvc)

	// Start server
	srv := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
		IdleTimeout:  *idleTimeout,
	}

	log.Printf("admin service listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func registerRoutes(mux *http.ServeMux, svc *admin.Service) {
	// Static files
	mux.Handle("/", http.FileServer(http.Dir(filepath.Join("web"))))
	mux.Handle("/static/", http.FileServer(http.Dir(filepath.Join("web", "static"))))

	// API routes with method dispatch
	// Users: GET /api/users (list), POST /api/users (create), DELETE /api/users (delete)
	mux.HandleFunc("/api/users", withJSON(func(w http.ResponseWriter, r *http.Request) error {
		switch r.Method {
		case "GET":
			return svc.ListUsers(w, r)
		case "POST":
			return svc.CreateUser(w, r)
		case "DELETE":
			return svc.DeleteUser(w, r)
		default:
			return fmt.Errorf("method not allowed")
		}
	}))

	// Documents: GET /api/documents (list), POST /api/documents (create), DELETE /api/documents (delete)
	mux.HandleFunc("/api/documents", withJSON(func(w http.ResponseWriter, r *http.Request) error {
		switch r.Method {
		case "GET":
			return svc.ListDocuments(w, r)
		case "POST":
			return svc.CreateDocument(w, r)
		case "DELETE":
			return svc.DeleteDocument(w, r)
		default:
			return fmt.Errorf("method not allowed")
		}
	}))

	// Permissions: GET /api/permissions (list), POST /api/permissions (grant), DELETE /api/permissions (revoke)
	mux.HandleFunc("/api/permissions", withJSON(func(w http.ResponseWriter, r *http.Request) error {
		switch r.Method {
		case "GET":
			return svc.ListPermissions(w, r)
		case "POST":
			return svc.GrantPermission(w, r)
		case "DELETE":
			return svc.RevokePermission(w, r)
		default:
			return fmt.Errorf("method not allowed")
		}
	}))

	// Audit logs
	mux.HandleFunc("/api/audit-logs", withJSON(svc.GetAuditLogs))
}

func withJSON(handler func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := handler(w, r); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"` + err.Error() + `"}`))
		}
	}
}
