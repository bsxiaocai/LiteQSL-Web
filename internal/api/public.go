package api

import (
	"net/http"
	"strconv"

	"github.com/bsxiaocai/LiteQSL-Web/internal/database"
	"github.com/bsxiaocai/LiteQSL-Web/internal/qso"
	"github.com/bsxiaocai/LiteQSL-Web/internal/timeutil"
)

// handleStationInfo 返回电台公开信息。
func (s *Server) handleStationInfo(w http.ResponseWriter, r *http.Request) {
	settings, err := database.GetAllSettings(s.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	callsign := settings["callsign"]
	if callsign == "" {
		callsign = "BH7GUL"
	}
	stationName := settings["station_name"]
	if stationName == "" {
		stationName = "QSL & Log Management"
	}
	tz := settings["visitor_timezone"]
	if tz == "" {
		tz = timeutil.Beijing
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"callsign":         callsign,
		"station_name":     stationName,
		"title":            callsign + " " + stationName,
		"visitor_timezone": tz,
	})
}

// handleRecent 返回最近通联（转访客时区）。
func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request) {
	band := r.URL.Query().Get("band")
	mode := r.URL.Query().Get("mode")
	qsoType := r.URL.Query().Get("qso_type")
	page := queryInt(r, "page", 1, 1, 0)
	pageSize := queryInt(r, "page_size", 20, 1, 100)

	result, err := s.qso.Recent(band, mode, qsoType, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	tz := s.visitorTimezone()
	writeJSON(w, http.StatusOK, map[string]any{
		"logs":      convertRecords(result.Logs, tz),
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
		"timezone":  tz,
	})
}

// handleSearch 组合查询（转访客时区）。
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	f := qso.Filters{
		Call:     r.URL.Query().Get("call"),
		Band:     r.URL.Query().Get("band"),
		Mode:     r.URL.Query().Get("mode"),
		DateFrom: r.URL.Query().Get("date_from"),
		DateTo:   r.URL.Query().Get("date_to"),
	}
	page := queryInt(r, "page", 1, 1, 0)
	pageSize := queryInt(r, "page_size", 20, 1, 100)

	result, err := s.qso.List(f, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	tz := s.visitorTimezone()
	writeJSON(w, http.StatusOK, map[string]any{
		"logs":      convertRecords(result.Logs, tz),
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
		"timezone":  tz,
	})
}

// handleBands 返回已使用的波段列表。
func (s *Server) handleBands(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT DISTINCT band FROM logs WHERE band != '' ORDER BY band")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	defer rows.Close()
	bands := []string{}
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		bands = append(bands, b)
	}
	writeJSON(w, http.StatusOK, bands)
}

// handleModes 返回已使用的模式列表。
func (s *Server) handleModes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT DISTINCT mode FROM logs WHERE mode != '' ORDER BY mode")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	defer rows.Close()
	modes := []string{}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			writeError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		modes = append(modes, m)
	}
	writeJSON(w, http.StatusOK, modes)
}

// visitorTimezone 返回访客显示时区（默认北京）。
func (s *Server) visitorTimezone() string {
	settings, err := database.GetAllSettings(s.db)
	if err != nil {
		return timeutil.Beijing
	}
	tz := settings["visitor_timezone"]
	if tz == "" {
		return timeutil.Beijing
	}
	return tz
}

// convertRecords 把一批 UTC 记录转换到目标显示时区。
func convertRecords(records []*qso.QSO, targetTZ string) []*qso.QSO {
	out := make([]*qso.QSO, 0, len(records))
	for _, rec := range records {
		out = append(out, convertRecord(rec, targetTZ))
	}
	return out
}

// convertRecord 转换单条记录到目标时区（EYEBALL 不转换）。
func convertRecord(rec *qso.QSO, targetTZ string) *qso.QSO {
	c := *rec // 浅拷贝，仅修改日期/时间字段
	if rec.QSOType == nil || *rec.QSOType != "EYEBALL" {
		d, t, err := timeutil.ConvertQsoDateTime(qsoStr(rec.QSODate), qsoStr(rec.TimeOn), timeutil.UTC, targetTZ)
		if err == nil {
			c.QSODate = &d
			c.TimeOn = &t
		}
	}
	return &c
}

func qsoStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// queryInt 解析整型查询参数（越界时收敛到边界）。
func queryInt(r *http.Request, name string, def, min, max int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if n < min {
		n = min
	}
	if max > 0 && n > max {
		n = max
	}
	return n
}
