package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/bsxiaocai/LiteQSL-Web/internal/config"
	"github.com/bsxiaocai/LiteQSL-Web/internal/database"
)

// 本文件用 httptest 覆盖前端（static/js）实际调用的全部后端端点，
// 断言路径、方法与响应结构，作为第四阶段「前端兼容」的回归依据。

type env struct {
	ts     *httptest.Server
	db     *sql.DB
	srv    *Server
	client *http.Client
	csrf   string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "qsl.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Init(db); err != nil {
		t.Fatalf("init db: %v", err)
	}

	cfg := &config.Config{
		Host:                "127.0.0.1",
		DBPath:              dbPath,
		StaticDir:           filepath.Join("..", "..", "static"),
		SecretKey:           "test-secret-key",
		LoginMaxAttempts:    1000,
		LoginLockoutSeconds: 600,
		MaxBackups:          20,
		SessionCookieName:   "session",
		SessionMaxAgeSec:    604800,
		LogLevel:            "error",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(cfg, db, logger)
	ts := httptest.NewServer(srv.Handler())

	// 统一清理：关闭 HTTP 服务与（可能被「恢复」替换过的）数据库连接，再触发 GC
	// 释放 Windows 上延迟关闭的文件句柄，保证 TempDir 能正常清理。
	t.Cleanup(func() {
		ts.Close()
		_ = srv.db.Close()
		_ = db.Close()
		runtime.GC()
	})

	jar, _ := cookiejar.New(nil)
	return &env{ts: ts, db: db, srv: srv, client: &http.Client{Jar: jar}}
}

// allowAdmin 跳过首次登录限制，便于测试 CRUD。
func (e *env) allowAdmin(t *testing.T) {
	t.Helper()
	if _, err := e.db.Exec("UPDATE users SET first_login = 0"); err != nil {
		t.Fatalf("reset first_login: %v", err)
	}
}

// do 发送请求并返回状态码与响应体。
func (e *env) do(t *testing.T, method, path string, body any, withCSRF bool) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.ts.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withCSRF {
		req.Header.Set("X-CSRF-Token", e.csrf)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func (e *env) doJSON(t *testing.T, method, path string, body any, withCSRF bool) map[string]any {
	t.Helper()
	status, data := e.do(t, method, path, body, withCSRF)
	if status < 200 || status >= 300 {
		t.Fatalf("%s %s -> %d: %s", method, path, status, data)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s %s: unmarshal %q: %v", method, path, data, err)
	}
	return m
}

// login 登录并获取 CSRF Token。
func (e *env) login(t *testing.T, username, password string) map[string]any {
	t.Helper()
	m := e.doJSON(t, http.MethodPost, "/api/admin/login",
		map[string]string{"username": username, "password": password}, false)
	e.csrf = e.doJSON(t, http.MethodGet, "/api/admin/csrf-token", nil, false)["csrf_token"].(string)
	return m
}

func (e *env) addQSO(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	return e.doJSON(t, http.MethodPost, "/api/admin/logs", body, true)
}

func sampleQSO() map[string]any {
	return map[string]any{
		"call": "bh7aa", "qso_date": "20260101", "time_on": "0100",
		"input_timezone": "Asia/Shanghai", "qso_type": "NORMAL",
		"freq": "14.074", "mode": "FT8", "rst_sent": "59", "rst_rcvd": "59",
		"qsl_status": "未发送",
	}
}

// ===== 测试 =====

func TestHealthAndStaticAssets(t *testing.T) {
	e := newEnv(t)
	status, data := e.do(t, http.MethodGet, "/health", nil, false)
	if status != 200 || !strings.Contains(string(data), `"status":"ok"`) {
		t.Fatalf("health -> %d %s", status, data)
	}
	// 前端引用的全部静态资源。
	for _, p := range []string{
		"/", "/admin", "/static/css/tailwind.js", "/static/js/public/app.js",
		"/static/js/admin/app.js", "/static/js/common/index.js", "/static/js/admin/qso-table.js",
		"/static/js/admin/stats.js", "/static/js/admin/backup.js",
	} {
		if s, _ := e.do(t, http.MethodGet, p, nil, false); s != 200 {
			t.Errorf("GET %s -> %d, want 200", p, s)
		}
	}
}

func TestPublicEndpoints(t *testing.T) {
	e := newEnv(t)
	if m := e.doJSON(t, http.MethodGet, "/api/station-info", nil, false); m["callsign"] == "" {
		t.Fatalf("station-info missing callsign: %v", m)
	}
	for _, p := range []string{"/api/recent", "/api/search?call=BH7AA", "/api/bands", "/api/modes"} {
		if s, data := e.do(t, http.MethodGet, p, nil, false); s != 200 {
			t.Errorf("GET %s -> %d: %s", p, s, data)
		}
	}
}

func TestAuthAndFirstLoginFlow(t *testing.T) {
	e := newEnv(t)

	// 未登录时 check 返回 logged_in=false（200，不是 401）。
	if m := e.doJSON(t, http.MethodGet, "/api/admin/check", nil, false); m["logged_in"] != false {
		t.Fatalf("check before login: %v", m)
	}
	// 未登录访问受保护接口 -> 401。
	if s, _ := e.do(t, http.MethodGet, "/api/admin/logs", nil, false); s != 401 {
		t.Fatalf("logs before login -> %d, want 401", s)
	}

	// 默认管理员登录。
	m := e.login(t, "admin", "Admin123!")
	if m["ok"] != true || m["first_login"] != true {
		t.Fatalf("login: %v", m)
	}
	// 首次登录未完成时，管理操作被拦截为 403。
	if s, _ := e.do(t, http.MethodPost, "/api/admin/logs", sampleQSO(), true); s != 403 {
		t.Fatalf("add before first-login -> %d, want 403", s)
	}
	// 完成首次登录。
	fm := e.doJSON(t, http.MethodPost, "/api/admin/complete-first-login", map[string]any{
		"old_password": "Admin123!", "new_username": "op12345", "new_password": "OpPass123!",
		"confirm_username": "op12345", "confirm_password": "OpPass123!",
	}, true)
	if fm["ok"] != true {
		t.Fatalf("complete-first-login: %v", fm)
	}
	// 用新凭据重新登录后 first_login=false。
	m2 := e.login(t, "op12345", "OpPass123!")
	if m2["first_login"] != false {
		t.Fatalf("re-login: %v", m2)
	}
	// 改密码。
	if s, data := e.do(t, http.MethodPost, "/api/admin/change-password",
		map[string]string{"old_password": "OpPass123!", "new_password": "NewPass456!"}, true); s != 200 {
		t.Fatalf("change-password -> %d: %s", s, data)
	}
	// 旧密码失效。
	if _, data := e.do(t, http.MethodPost, "/api/admin/login",
		map[string]string{"username": "op12345", "password": "OpPass123!"}, false); !strings.Contains(string(data), "用户名或密码错误") {
		t.Fatalf("old password should fail: %s", data)
	}
	// 登出后受保护接口 401。
	e.doJSON(t, http.MethodPost, "/api/admin/logout", nil, false)
	if s, _ := e.do(t, http.MethodGet, "/api/admin/logs", nil, false); s != 401 {
		t.Fatalf("logs after logout -> %d, want 401", s)
	}
}

func TestLogCRUDAndDuplicate(t *testing.T) {
	e := newEnv(t)
	e.allowAdmin(t)
	e.login(t, "admin", "Admin123!")

	added := e.addQSO(t, sampleQSO())
	if added["ok"] != true {
		t.Fatalf("add: %v", added)
	}

	// 列表：分页结构 + 时区归一化（北京 0100 -> UTC 前一天 1700）。
	list := e.doJSON(t, http.MethodGet, "/api/admin/logs?page=1&page_size=50", nil, false)
	logs := list["logs"].([]any)
	if len(logs) != 1 {
		t.Fatalf("list len = %d", len(logs))
	}
	rec := logs[0].(map[string]any)
	if rec["call"] != "BH7AA" || rec["qso_date"] != "20251231" || rec["time_on"] != "1700" || rec["band"] != "20m" {
		t.Fatalf("normalized record: %v", rec)
	}
	if _, ok := rec["created_at"].(string); !ok {
		t.Fatalf("created_at should be a string: %v", rec["created_at"])
	}

	// 重复检测 -> 409。
	if s, data := e.do(t, http.MethodPost, "/api/admin/logs", sampleQSO(), true); s != 409 || !strings.Contains(string(data), "重复记录") {
		t.Fatalf("duplicate -> %d: %s", s, data)
	}
	// force 跳过重复检测。
	forced := sampleQSO()
	forced["force"] = true
	if m := e.addQSO(t, forced); m["ok"] != true {
		t.Fatalf("forced add: %v", m)
	}

	// 编辑。
	edit := sampleQSO()
	edit["qsl_status"] = "已发送"
	if s, data := e.do(t, http.MethodPut, "/api/admin/logs/1", edit, true); s != 200 {
		t.Fatalf("edit -> %d: %s", s, data)
	}
	// 单条状态。
	if s, data := e.do(t, http.MethodPut, "/api/admin/logs/1/status",
		map[string]string{"qsl_status": "无需发送"}, true); s != 200 {
		t.Fatalf("status -> %d: %s", s, data)
	}
	// 字段筛选。
	filtered := e.doJSON(t, http.MethodGet, "/api/admin/logs?qsl_status=无需发送", nil, false)
	if filtered["total"].(float64) != 1 {
		t.Fatalf("filter total = %v", filtered["total"])
	}
	// 删除。
	if s, data := e.do(t, http.MethodDelete, "/api/admin/logs/2", nil, true); s != 200 {
		t.Fatalf("delete -> %d: %s", s, data)
	}
	if s, _ := e.do(t, http.MethodDelete, "/api/admin/logs/999", nil, true); s != 404 {
		t.Fatalf("delete missing -> %d, want 404", s)
	}
}

func TestADIFImportAndExport(t *testing.T) {
	e := newEnv(t)
	e.allowAdmin(t)
	e.login(t, "admin", "Admin123!")

	adifText := "ADIF Export from test\n<EOH>\n" +
		"<CALL:5>BH7ZZ <QSO_DATE:8>20260201 <TIME_ON:4>1200 <BAND:3>20m <MODE:3>FT8 " +
		"<RST_SENT:2>59 <RST_RCVD:2>59 <QSL_SENT:1>Y <EOR>"

	// 首次导入。
	status, data := e.importADIF(t, "test.adi", adifText, false)
	if status != 200 || !strings.Contains(string(data), `"count":1`) {
		t.Fatalf("import -> %d: %s", status, data)
	}
	// 再次导入同一内容 -> 返回重复列表（ok=false）。
	status, data = e.importADIF(t, "test.adi", adifText, false)
	if status != 200 || !strings.Contains(string(data), `"duplicates"`) || !strings.Contains(string(data), `"existing_id"`) {
		t.Fatalf("duplicate import -> %d: %s", status, data)
	}
	// force 强制导入。
	status, data = e.importADIF(t, "test.adi", adifText, true)
	if status != 200 || !strings.Contains(string(data), `"count":1`) {
		t.Fatalf("forced import -> %d: %s", status, data)
	}

	// 导出 ADIF / CSV。
	if s, body := e.do(t, http.MethodGet, "/api/admin/export-adif", nil, false); s != 200 || !strings.Contains(string(body), "<EOH>") {
		t.Fatalf("export-adif -> %d: %s", s, body)
	}
	if s, body := e.do(t, http.MethodGet, "/api/admin/export-csv", nil, false); s != 200 ||
		!bytes.HasPrefix(body, []byte("\xef\xbb\xbf")) || !strings.Contains(string(body), "CALL,DATE,TIME") {
		t.Fatalf("export-csv -> %d: %s", s, body)
	}

	// 批量导出（按 ID）。
	e.addQSO(t, sampleQSO())
	ids := e.doJSON(t, http.MethodGet, "/api/admin/logs", nil, false)["logs"].([]any)
	firstID := int(ids[0].(map[string]any)["id"].(float64))
	req, _ := http.NewRequest(http.MethodPost, e.ts.URL+"/api/admin/logs/batch-export",
		strings.NewReader(`{"ids":[`+strconv.Itoa(firstID)+`],"format":"csv"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", e.csrf)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("batch-export: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !bytes.HasPrefix(b, []byte("\xef\xbb\xbf")) {
		t.Fatalf("batch-export -> %d: %s", resp.StatusCode, b)
	}
}

// importADIF 以 multipart 方式导入 ADIF。
func (e *env) importADIF(t *testing.T, filename, content string, force bool) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", filename)
	_, _ = fw.Write([]byte(content))
	if force {
		_ = w.WriteField("force", "true")
	}
	_ = w.Close()

	req, err := http.NewRequest(http.MethodPost, e.ts.URL+"/api/admin/import-adif", &buf)
	if err != nil {
		t.Fatalf("new import request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-CSRF-Token", e.csrf)
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("import request: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func TestBatchOperations(t *testing.T) {
	e := newEnv(t)
	e.allowAdmin(t)
	e.login(t, "admin", "Admin123!")

	ids := []int{}
	for i := 0; i < 3; i++ {
		body := sampleQSO()
		body["call"] = "BH7" + string(rune('A'+i))
		body["time_on"] = "0" + string(rune('1'+i)) + "00"
		m := e.addQSO(t, body)
		ids = append(ids, int(m["id"].(float64)))
	}

	// 批量改状态。
	if m := e.doJSON(t, http.MethodPost, "/api/admin/logs/batch-status",
		map[string]any{"ids": ids, "status": "已发送"}, true); m["updated"].(float64) != 3 {
		t.Fatalf("batch-status: %v", m)
	}
	// 批量标 SK。
	if m := e.doJSON(t, http.MethodPost, "/api/admin/logs/batch-sk",
		map[string]any{"ids": ids, "is_sk": 1}, true); m["updated"].(float64) != 3 {
		t.Fatalf("batch-sk: %v", m)
	}
	// 非法状态 -> 400。
	if s, _ := e.do(t, http.MethodPost, "/api/admin/logs/batch-status",
		map[string]any{"ids": ids, "status": "不存在"}, true); s != 400 {
		t.Fatalf("invalid batch status -> %d, want 400", s)
	}
	// 批量删除。
	if m := e.doJSON(t, http.MethodPost, "/api/admin/logs/batch-delete",
		map[string]any{"ids": ids}, true); m["deleted"].(float64) != 3 {
		t.Fatalf("batch-delete: %v", m)
	}
}

func TestBackupAndRestore(t *testing.T) {
	e := newEnv(t)
	e.allowAdmin(t)
	e.login(t, "admin", "Admin123!")
	e.addQSO(t, sampleQSO())

	// 创建备份。
	bm := e.doJSON(t, http.MethodPost, "/api/admin/backup", nil, true)
	bk := bm["backup"].(map[string]any)
	filename := bk["filename"].(string)
	if !strings.HasSuffix(filename, ".db") {
		t.Fatalf("backup filename: %v", filename)
	}
	if _, ok := bk["size"].(float64); !ok {
		t.Fatalf("backup size should be a number: %v", bk["size"])
	}
	// 列表。
	lm := e.doJSON(t, http.MethodGet, "/api/admin/backups", nil, false)
	if len(lm["backups"].([]any)) != 1 {
		t.Fatalf("backups: %v", lm)
	}
	// 下载。
	if s, body := e.do(t, http.MethodGet, "/api/admin/backups/"+filename, nil, false); s != 200 || len(body) == 0 {
		t.Fatalf("download backup -> %d (%d bytes)", s, len(body))
	}
	// 路径穿越防护。
	if s, _ := e.do(t, http.MethodGet, "/api/admin/backups/..%2F..%2Fqsl.db", nil, false); s == 200 {
		t.Fatalf("path traversal should be rejected")
	}

	// 恢复（先改数据，再恢复回备份点）。
	if s, _ := e.do(t, http.MethodPut, "/api/admin/settings",
		map[string]string{"callsign": "BG7ZZZ"}, true); s != 200 {
		t.Fatalf("settings before restore failed")
	}
	if m := e.doJSON(t, http.MethodPost, "/api/admin/restore",
		map[string]string{"filename": filename}, true); m["ok"] != true {
		t.Fatalf("restore: %v", m)
	}
	// 恢复后 callsign 回到备份时的值。
	si := e.doJSON(t, http.MethodGet, "/api/station-info", nil, false)
	if si["callsign"] != "BH7GUL" {
		t.Fatalf("callsign after restore = %v, want BH7GUL", si["callsign"])
	}
	// 恢复会清除会话，需重新登录。
	e.login(t, "admin", "Admin123!")
	// 删除备份。
	if m := e.doJSON(t, http.MethodDelete, "/api/admin/backups/"+filename, nil, true); m["ok"] != true {
		t.Fatalf("delete backup: %v", m)
	}
}

func TestSettingsAndStats(t *testing.T) {
	e := newEnv(t)
	e.allowAdmin(t)
	e.login(t, "admin", "Admin123!")
	e.addQSO(t, sampleQSO())

	// 设置读写。
	sm := e.doJSON(t, http.MethodGet, "/api/admin/settings", nil, false)
	if sm["settings"].(map[string]any)["visitor_timezone"] != "Asia/Shanghai" {
		t.Fatalf("settings: %v", sm)
	}
	if m := e.doJSON(t, http.MethodPut, "/api/admin/settings",
		map[string]string{"callsign": "bg7test", "visitor_timezone": "UTC"}, true); len(m["updated"].([]any)) != 2 {
		t.Fatalf("settings put: %v", m)
	}
	if si := e.doJSON(t, http.MethodGet, "/api/station-info", nil, false); si["callsign"] != "BG7TEST" || si["visitor_timezone"] != "UTC" {
		t.Fatalf("station-info after settings: %v", si)
	}
	// 非法时区 -> 400。
	if s, _ := e.do(t, http.MethodPut, "/api/admin/settings",
		map[string]string{"visitor_timezone": "Mars/Base"}, true); s != 400 {
		t.Fatalf("invalid timezone -> %d, want 400", s)
	}

	// 枚举。
	if m := e.doJSON(t, http.MethodGet, "/api/admin/qsl-statuses", nil, false); len(m["statuses"].([]any)) != 6 {
		t.Fatalf("qsl-statuses: %v", m)
	}
	if m := e.doJSON(t, http.MethodGet, "/api/admin/qso-types", nil, false); len(m["types"].([]any)) != 4 {
		t.Fatalf("qso-types: %v", m)
	}

	// 统计（前端 7 个调用）。
	sum := e.doJSON(t, http.MethodGet, "/api/admin/stats/summary", nil, false)
	for _, k := range []string{"total_logs", "total_callsigns", "this_month", "this_year", "qsl_pending"} {
		if _, ok := sum[k].(float64); !ok {
			t.Fatalf("stats summary missing %s: %v", k, sum)
		}
	}
	for _, p := range []string{
		"/api/admin/stats/by-band", "/api/admin/stats/by-mode", "/api/admin/stats/by-type",
		"/api/admin/stats/by-month?months=12", "/api/admin/stats/by-hour", "/api/admin/stats/top-calls?limit=20",
	} {
		s, data := e.do(t, http.MethodGet, p, nil, false)
		if s != 200 || !strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
			t.Errorf("GET %s -> %d: %s", p, s, data)
		}
	}
	// 空结果必须是 []（不是 null）。
	if s, data := e.do(t, http.MethodGet, "/api/admin/stats/top-calls?limit=20", nil, false); s != 200 || string(bytes.TrimSpace(data)) == "null" {
		t.Fatalf("empty stats must be []: %s", data)
	}
}

func TestImportSizeLimit(t *testing.T) {
	e := newEnv(t)
	e.allowAdmin(t)
	e.login(t, "admin", "Admin123!")
	big := strings.Repeat("X", 11*1024*1024)
	status, _ := e.importADIF(t, "big.adi", big, false)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized import -> %d, want 413", status)
	}
}
