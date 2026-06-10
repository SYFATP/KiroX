package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"reg_go/internal/browser"
	"reg_go/internal/email"
	"reg_go/internal/proxy"
	"reg_go/internal/storage"
	"reg_go/internal/task"
	"reg_go/internal/updater"
)

type webServer struct {
	app      *App
	password string
	session  string
}

func runWebServer(addr, password string) {
	app := NewApp()
	log.SetOutput(&logWriter{app: app})
	log.SetFlags(log.Ltime)
	proxy.InitPool(storage.GetDataDir())
	go updater.CleanupTemp()
	defer storage.FlushAccountsSync()

	s := &webServer{app: app, password: password, session: randomToken()}

	staticFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatalf("加载前端资源失败: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/session", s.handleSession)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/", s.handleAPI)
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	log.Printf("[Web] KiroX Web UI listening on http://%s", addr)
	if password == "" {
		log.Printf("[Web] 未设置登录密码，仅建议在本机/受信反代后使用")
	}
	log.Fatal(http.ListenAndServe(addr, mux))
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}

func (s *webServer) authed(r *http.Request) bool {
	if s.password == "" {
		return true
	}
	c, err := r.Cookie("kirox_session")
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.session)) == 1
}

func (s *webServer) writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *webServer) handleSession(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, map[string]interface{}{"authRequired": s.password != "", "authenticated": s.authed(r)})
}

func (s *webServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.password == "" {
		s.writeJSON(w, map[string]interface{}{"success": true})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.password)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "kirox_session",
		Value:    s.session,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})
	s.writeJSON(w, map[string]interface{}{"success": true})
}

func (s *webServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "kirox_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	s.session = randomToken()
	s.writeJSON(w, map[string]interface{}{"success": true})
}

func (s *webServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authed(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	var args []json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil && err.Error() != "EOF" {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	result, err := s.call(method, args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, result)
}

func decodeArg[T any](args []json.RawMessage, i int) (T, error) {
	var v T
	if i >= len(args) {
		return v, nil
	}
	if err := json.Unmarshal(args[i], &v); err != nil {
		return v, err
	}
	return v, nil
}

func (s *webServer) call(method string, args []json.RawMessage) (interface{}, error) {
	a := s.app
	switch method {
	case "GetStatus":
		return a.GetStatus(), nil
	case "GetLogs":
		return a.GetLogs(), nil
	case "GetOverview":
		return a.GetOverview(), nil
	case "GetTaskStatus":
		return a.GetTaskStatus(), nil
	case "VerifyLicense":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.VerifyLicense(v), nil
	case "CheckLicense":
		return a.CheckLicense(), nil
	case "GetLicenseInfo":
		return a.GetLicenseInfo(), nil
	case "LogoutLicense":
		return a.LogoutLicense(), nil
	case "GetMoeMailConfigs":
		return a.GetMoeMailConfigs(), nil
	case "SaveMoeMailConfigs":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.SaveMoeMailConfigs(v), nil
	case "TestMoeMailConnection":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.TestMoeMailConnection(v), nil
	case "GetCloudMailConfigs":
		return a.GetCloudMailConfigs(), nil
	case "SaveCloudMailConfigs":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.SaveCloudMailConfigs(v), nil
	case "TestCloudMailConnection":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.TestCloudMailConnection(v), nil
	case "AddOutlookAccounts":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.AddOutlookAccounts(v), nil
	case "GetOutlookAccounts":
		return a.GetOutlookAccounts(), nil
	case "DeleteOutlookAccount":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.DeleteOutlookAccount(v), nil
	case "ClearOutlookAccounts":
		return a.ClearOutlookAccounts(), nil
	case "ClearRegisteredOutlookAccounts":
		return a.ClearRegisteredOutlookAccounts(), nil
	case "ImportOutlookFile":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.ImportOutlookFile(v), nil
	case "GetDataDir":
		return a.GetDataDir(), nil
	case "SetDataDir":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.SetDataDir(v), nil
	case "ResetDataDir":
		return a.ResetDataDir(), nil
	case "GetResultOutputDir":
		return a.GetResultOutputDir(), nil
	case "SetResultOutputDir":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.SetResultOutputDir(v), nil
	case "ResetResultOutputDir":
		return a.ResetResultOutputDir(), nil
	case "GetProxy":
		return a.GetProxy(), nil
	case "SetProxy":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.SetProxy(v), nil
	case "DetectProxy":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.DetectProxy(v), nil
	case "ResetProxy":
		return a.ResetProxy(), nil
	case "GetLanguage":
		return a.GetLanguage(), nil
	case "GetKiroRsConfig":
		return a.GetKiroRsConfig(), nil
	case "SaveKiroRsConfig":
		v, err := decodeArg[storage.KiroRsConfig](args, 0)
		if err != nil {
			return nil, err
		}
		return a.SaveKiroRsConfig(v), nil
	case "TestKiroRsConnection":
		v, err := decodeArg[storage.KiroRsConfig](args, 0)
		if err != nil {
			return nil, err
		}
		return a.TestKiroRsConnection(v), nil
	case "SetLanguage":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.SetLanguage(v), nil
	case "GetOSLanguage":
		return a.GetOSLanguage(), nil
	case "StartTask":
		v, err := decodeArg[task.StartTaskRequest](args, 0)
		if err != nil {
			return nil, err
		}
		return a.StartTask(v), nil
	case "StopTask":
		return a.StopTask(), nil
	case "CheckUpdate":
		return a.CheckUpdate(), nil
	case "DownloadUpdate":
		return map[string]interface{}{"error": "Web 模式不支持应用内更新"}, nil
	case "ResetFingerprintCache":
		browser.ResetIdentityCache()
		return map[string]interface{}{"success": true}, nil
	case "CancelUpdate":
		return a.CancelUpdate(), nil
	case "LoadOutputAccounts":
		return a.LoadOutputAccounts(), nil
	case "GetSubscriptionPlans":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.GetSubscriptionPlans(v), nil
	case "GetSubscriptionLink":
		em, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		plan, err := decodeArg[string](args, 1)
		if err != nil {
			return nil, err
		}
		return a.GetSubscriptionLink(em, plan), nil
	case "ListProxyPool":
		return a.ListProxyPool(), nil
	case "AddProxyEntry":
		name, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		raw, err := decodeArg[string](args, 1)
		if err != nil {
			return nil, err
		}
		weight, err := decodeArg[int](args, 2)
		if err != nil {
			return nil, err
		}
		return a.AddProxyEntry(name, raw, weight), nil
	case "UpdateProxyEntry":
		id, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		name, err := decodeArg[string](args, 1)
		if err != nil {
			return nil, err
		}
		raw, err := decodeArg[string](args, 2)
		if err != nil {
			return nil, err
		}
		weight, err := decodeArg[int](args, 3)
		if err != nil {
			return nil, err
		}
		enabled, err := decodeArg[bool](args, 4)
		if err != nil {
			return nil, err
		}
		return a.UpdateProxyEntry(id, name, raw, weight, enabled), nil
	case "DeleteProxyEntry":
		id, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.DeleteProxyEntry(id), nil
	case "TestProxyEntry":
		v, err := decodeArg[string](args, 0)
		if err != nil {
			return nil, err
		}
		return a.TestProxyEntry(v), nil
	case "OpenURL":
		return map[string]interface{}{"success": true}, nil
	case "SelectDirectory", "SelectOutlookFile":
		return "", nil
	default:
		return nil, http.ErrNotSupported
	}
}

var _ = email.MoeMailConfig{}
