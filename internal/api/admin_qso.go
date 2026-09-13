package api

import (
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/bsxiaocai/LiteQSL-Web/internal/adif"
	"github.com/bsxiaocai/LiteQSL-Web/internal/qso"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// handleListLogs 分页查询记录（管理接口，返回原始 UTC）。
func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	f := qso.Filters{
		Call:      r.URL.Query().Get("call"),
		Band:      r.URL.Query().Get("band"),
		Mode:      r.URL.Query().Get("mode"),
		QSLStatus: r.URL.Query().Get("qsl_status"),
		QSOType:   r.URL.Query().Get("qso_type"),
		DateFrom:  r.URL.Query().Get("date_from"),
		DateTo:    r.URL.Query().Get("date_to"),
		IsSK:      r.URL.Query().Get("is_sk"),
		SortBy:    r.URL.Query().Get("sort_by"),
		SortOrder: r.URL.Query().Get("sort_order"),
	}
	page := queryInt(r, "page", 1, 1, 0)
	pageSize := queryInt(r, "page_size", 50, 1, 200)

	result, err := s.qso.List(f, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleAddLog 新增 QSO。
func (s *Server) handleAddLog(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var in qso.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.QSOType == "" {
		in.QSOType = "NORMAL"
	}
	if msg := validateRequired(&in, in.QSOType); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	// 自动推导 band（仅 freq 无 band 时）。
	if in.Freq != "" && in.Band == "" {
		if b := qso.FreqToBand(in.Freq); b != "" {
			in.Band = b
		}
	}
	if !validTimezone(in.InputTimezone) {
		writeError(w, http.StatusBadRequest, "无效的录入时区")
		return
	}
	if err := in.Normalize(); err != nil {
		writeError(w, http.StatusBadRequest, "日期或时间格式无效")
		return
	}

	if !in.Force {
		if in.QSOType == "EYEBALL" {
			if existing, _ := s.qso.CheckDuplicateEyeball(in.Call, in.QSODate); existing != nil {
				writeError(w, http.StatusConflict, fmt.Sprintf("重复记录：已存在呼号 %s 在 %s 的 Eyeball QSO 记录 (ID: %d)", in.Call, in.QSODate, existing.ID))
				return
			}
		} else {
			bandForCheck := in.Band
			if bandForCheck == "" && in.Freq != "" {
				bandForCheck = qso.FreqToBand(in.Freq)
			}
			if bandForCheck != "" {
				if existing, _ := s.qso.CheckDuplicate(in.Call, in.QSODate, in.TimeOn, bandForCheck, in.Mode); existing != nil {
					writeError(w, http.StatusConflict, fmt.Sprintf("重复记录：已存在呼号 %s 在 %s %s %s %s 的记录 (ID: %d)", in.Call, in.QSODate, in.TimeOn, bandForCheck, in.Mode, existing.ID))
					return
				}
			}
		}
	}

	id, err := s.qso.InsertNormalized(&in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// handleEditLog 编辑 QSO。
func (s *Server) handleEditLog(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "无效的记录 ID")
		return
	}
	var in qso.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Call == "" {
		writeError(w, http.StatusBadRequest, "呼号不能为空")
		return
	}
	if in.QSODate == "" {
		writeError(w, http.StatusBadRequest, "日期不能为空")
		return
	}
	if in.QSOType == "" {
		in.QSOType = "NORMAL"
	}
	if msg := validateRequired(&in, in.QSOType); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if !validTimezone(in.InputTimezone) {
		writeError(w, http.StatusBadRequest, "无效的录入时区")
		return
	}

	updated, err := s.qso.Update(id, &in)
	if err != nil {
		writeError(w, http.StatusBadRequest, "日期或时间格式无效")
		return
	}
	if !updated {
		writeError(w, http.StatusNotFound, "记录不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleLogStatus 修改单条 QSL 状态。
func (s *Server) handleLogStatus(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "无效的记录 ID")
		return
	}
	var body struct {
		QSLStatus string `json:"qsl_status"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !validStatus(body.QSLStatus) {
		writeError(w, http.StatusBadRequest, "无效的卡片状态")
		return
	}
	updated, err := s.qso.UpdateQSLStatus(id, body.QSLStatus)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if !updated {
		writeError(w, http.StatusNotFound, "记录不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleDeleteLog 删除单条记录。
func (s *Server) handleDeleteLog(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "无效的记录 ID")
		return
	}
	deleted, err := s.qso.Delete(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "记录不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleImportADIF 导入 ADIF 文件。
func (s *Server) handleImportADIF(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "请上传文件")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请上传文件")
		return
	}
	defer file.Close()

	const maxSize = 10 * 1024 * 1024
	content, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if len(content) > maxSize {
		writeError(w, http.StatusRequestEntityTooLarge, "文件大小超过限制（最大 10MB）")
		return
	}

	force := r.FormValue("force") == "true"
	records := adif.Parse(decodeADIFContent(content))
	if len(records) == 0 {
		writeError(w, http.StatusBadRequest, "未解析到有效记录")
		return
	}

	if !force {
		dups, err := s.qso.CheckDuplicatesBatch(records)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if len(dups) > 0 {
			list := make([]map[string]any, 0, len(dups))
			for _, d := range dups {
				list = append(list, map[string]any{"record": d.Record, "existing_id": d.ExistingID})
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":              false,
				"duplicates":      list,
				"duplicate_count": len(dups),
				"total":           len(records),
			})
			return
		}
	}

	count, err := s.qso.InsertBatch(records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": count})
}

// handleExportADIF 筛选导出 ADIF。
func (s *Server) handleExportADIF(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	f := exportFilters(r)
	records, err := s.qso.AllFiltered(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	content := adif.Export(records)
	filename := "qsl_export_" + time.Now().Format("20060102") + ".adi"
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

// handleExportCSV 筛选导出 CSV。
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireAdmin(w, r, false); !ok {
		return
	}
	f := exportFilters(r)
	records, err := s.qso.AllFiltered(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	content, err := qso.ExportCSV(records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	filename := "qsl_export_" + time.Now().Format("20060102") + ".csv"
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// handleBatchDelete 批量删除。
func (s *Server) handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	deleted, err := s.qso.DeleteBatch(body.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": deleted})
}

// handleBatchStatus 批量修改 QSL 状态。
func (s *Server) handleBatchStatus(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var body struct {
		IDs    []int64 `json:"ids"`
		Status string  `json:"status"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	updated, err := s.qso.UpdateStatusBatch(body.IDs, body.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": updated})
}

// handleBatchSK 批量修改 SK 标记。
func (s *Server) handleBatchSK(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var body struct {
		IDs  []int64 `json:"ids"`
		IsSK int     `json:"is_sk"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.IsSK != 0 && body.IsSK != 1 {
		writeError(w, http.StatusBadRequest, "无效的 SK 标记值")
		return
	}
	updated, err := s.qso.UpdateSKBatch(body.IDs, body.IsSK)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": updated})
}

// handleBatchExport 批量导出选中记录。
func (s *Server) handleBatchExport(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := s.requireAdmin(w, r, false)
	if !ok {
		return
	}
	if !s.csrfOK(w, r, sess) {
		return
	}
	var body struct {
		IDs    []int64 `json:"ids"`
		Format string  `json:"format"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Format != "adif" && body.Format != "csv" {
		writeError(w, http.StatusBadRequest, "不支持的导出格式")
		return
	}
	records, err := s.qso.ByIDs(body.IDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if len(records) == 0 {
		writeError(w, http.StatusNotFound, "未找到指定记录")
		return
	}

	if body.Format == "adif" {
		content := adif.Export(records)
		filename := "qsl_export_" + time.Now().Format("20060102") + ".adi"
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
		return
	}
	content, err := qso.ExportCSV(records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	filename := "qsl_export_" + time.Now().Format("20060102") + ".csv"
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// ===== 辅助 =====

func validateRequired(in *qso.Input, qsoType string) string {
	for _, f := range qso.RequiredFields(qsoType) {
		if in.Get(f) == "" {
			return f + " 不能为空"
		}
	}
	return ""
}

func validTimezone(tz string) bool {
	return tz == "UTC" || tz == "Asia/Shanghai"
}

func validStatus(s string) bool {
	for _, st := range qso.QSLStatuses {
		if st == s {
			return true
		}
	}
	return false
}

// exportFilters 从查询参数构建导出筛选条件。
func exportFilters(r *http.Request) qso.Filters {
	return qso.Filters{
		Band:      r.URL.Query().Get("band"),
		Mode:      r.URL.Query().Get("mode"),
		QSLStatus: r.URL.Query().Get("qsl_status"),
		QSOType:   r.URL.Query().Get("qso_type"),
		DateFrom:  r.URL.Query().Get("date_from"),
		DateTo:    r.URL.Query().Get("date_to"),
	}
}

// decodeADIFContent 尝试多种编码解码（UTF-8 优先，GBK 兜底，最后 Latin-1）。
func decodeADIFContent(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	if s, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), string(b)); err == nil {
		return s
	}
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}
