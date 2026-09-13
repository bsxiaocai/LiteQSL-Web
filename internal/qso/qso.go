// Package qso 提供通联记录（logs 表）的数据访问、筛选、分页、波段推导与 CSV 导出。
//
// 行为对齐 v1.x app/database.py 中与 logs 相关的函数；字段名、SQL 语义、
// 返回结构（分页 JSON 字段）均保持一致。可空文本列以指针表示，使 JSON 输出
// null 与 v1.x一致。
package qso

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/bsxiaocai/LiteQSL-Web/internal/timeutil"
)

// ===== 常量（对齐 v1.x） =====

// QSLStatuses 是可用的 QSL 卡片状态。
var QSLStatuses = []string{"无法考证", "未发送", "已发送", "已收到", "无需发送", "电子确认"}

// QSOTypes 是 QSO 类型枚举（存英文，前端显示中文）。
var QSOTypes = []string{"NORMAL", "SAT", "REP", "EYEBALL"}

// QSOTypeLabels 是 QSO 类型的中文标签。
var QSOTypeLabels = map[string]string{
	"NORMAL":  "一般通联",
	"SAT":     "卫星通联",
	"REP":     "中继通联",
	"EYEBALL": "Eyeball通联",
}

// freqBandRange 描述一个业余频段（单位 MHz，左闭右开）。
type freqBandRange struct {
	low, high float64
	band      string
}

// freqBandRanges 与 v1.x FREQ_BAND_RANGES 一致（ITU Region 3）。
var freqBandRanges = []freqBandRange{
	{1.800, 2.000, "160m"},
	{3.500, 4.000, "80m"},
	{5.300, 5.400, "60m"},
	{7.000, 7.300, "40m"},
	{10.100, 10.150, "30m"},
	{14.000, 14.350, "20m"},
	{18.068, 18.168, "17m"},
	{21.000, 21.450, "15m"},
	{24.890, 25.000, "12m"},
	{28.000, 29.700, "10m"},
	{50.000, 54.000, "6m"},
	{144.000, 148.000, "2m"},
	{430.000, 440.000, "70cm"},
	{1240.000, 1300.000, "23cm"},
}

// BandFreqMap 是波段到代表频率的映射（CSV 导出兜底）。
var BandFreqMap = map[string]string{
	"160m": "1.800", "80m": "3.500", "60m": "5.300",
	"40m": "7.000", "30m": "10.100", "20m": "14.000",
	"17m": "18.068", "15m": "21.000", "12m": "24.890",
	"10m": "28.000", "6m": "50.000", "2m": "144.000",
	"70cm": "430.000", "23cm": "1240.000",
}

// ===== 数据结构 =====

// QSO 是 logs 表的一条记录。可空列用指针以区分 null 与空字符串。
type QSO struct {
	ID        int64   `json:"id"`
	Call      string  `json:"call"`
	QSODate   *string `json:"qso_date"`
	TimeOn    *string `json:"time_on"`
	Band      *string `json:"band"`
	Mode      *string `json:"mode"`
	RstSent   *string `json:"rst_sent"`
	RstRcvd   *string `json:"rst_rcvd"`
	QSLStatus *string `json:"qsl_status"`
	Comment   *string `json:"comment"`
	QSOType   *string `json:"qso_type"`
	Freq      *string `json:"freq"`
	TxFreq    *string `json:"tx_freq"`
	RxFreq    *string `json:"rx_freq"`
	SatName   *string `json:"sat_name"`
	SatMode   *string `json:"sat_mode"`
	IsSK      *int    `json:"is_sk"`
	QTH       *string `json:"qth"`
	CreatedAt *string `json:"created_at"`
}

// Input 是新增/编辑 QSO 的请求数据（字段与 v1.x 一致）。
type Input struct {
	Call          string `json:"call"`
	QSODate       string `json:"qso_date"`
	TimeOn        string `json:"time_on"`
	Band          string `json:"band"`
	Mode          string `json:"mode"`
	RstSent       string `json:"rst_sent"`
	RstRcvd       string `json:"rst_rcvd"`
	QSLStatus     string `json:"qsl_status"`
	Comment       string `json:"comment"`
	QSOType       string `json:"qso_type"`
	Freq          string `json:"freq"`
	TxFreq        string `json:"tx_freq"`
	RxFreq        string `json:"rx_freq"`
	SatName       string `json:"sat_name"`
	SatMode       string `json:"sat_mode"`
	IsSK          int    `json:"is_sk"`
	QTH           string `json:"qth"`
	InputTimezone string `json:"input_timezone"`
	Force         bool   `json:"force"`
}

// RequiredFields 返回指定 QSO 类型的必填字段（规则与 v1.x 一致）。
func RequiredFields(qsoType string) []string {
	switch qsoType {
	case "EYEBALL":
		return []string{"call", "qso_date", "qsl_status"}
	case "SAT":
		return []string{"call", "qso_date", "time_on", "sat_name", "tx_freq", "rx_freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"}
	case "REP":
		return []string{"call", "qso_date", "time_on", "tx_freq", "rx_freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"}
	default:
		return []string{"call", "qso_date", "time_on", "freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"}
	}
}

// Get 按字段名返回 Input 的字符串值（用于必填校验）。
func (in *Input) Get(field string) string {
	switch field {
	case "call":
		return in.Call
	case "qso_date":
		return in.QSODate
	case "time_on":
		return in.TimeOn
	case "band":
		return in.Band
	case "mode":
		return in.Mode
	case "rst_sent":
		return in.RstSent
	case "rst_rcvd":
		return in.RstRcvd
	case "qsl_status":
		return in.QSLStatus
	case "comment":
		return in.Comment
	case "qso_type":
		return in.QSOType
	case "freq":
		return in.Freq
	case "tx_freq":
		return in.TxFreq
	case "rx_freq":
		return in.RxFreq
	case "sat_name":
		return in.SatName
	case "sat_mode":
		return in.SatMode
	case "qth":
		return in.QTH
	}
	return ""
}

// Paginated 是分页查询结果（结构与 v1.x 一致）。
type Paginated struct {
	Logs     []*QSO `json:"logs"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// Filters 是列表/导出筛选条件。
type Filters struct {
	Call      string
	Band      string
	Mode      string
	QSLStatus string
	QSOType   string
	DateFrom  string
	DateTo    string
	IsSK      string // "" 表示不筛选，"0"/"1" 表示筛选
	SortBy    string
	SortOrder string
}

// Duplicate 表示一条重复记录。
type Duplicate struct {
	Record     *Input
	ExistingID int64
}

// ===== 工具函数 =====

// FreqToBand 根据频率（MHz）推导波段；无法识别时返回空字符串。
func FreqToBand(freq string) string {
	if freq == "" {
		return ""
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(freq), 64)
	if err != nil {
		return ""
	}
	for _, r := range freqBandRanges {
		if f >= r.low && f < r.high {
			return r.band
		}
	}
	return ""
}

// escapeLike 转义 SQLite LIKE 通配符（%、_、\），防止 LIKE 模式注入。
func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intPtr(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func ns(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

func ni(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// ===== 规范化（对齐 _auto_fill_freq_band + normalize_qso_to_utc） =====

// Normalize 规范化输入数据：呼号大写、Eyeball 清理、频率→波段、时区转 UTC。
// 顺序与 v1.x 一致：先做时间归一化，再做频率/波段补全。
// 注意：本方法会就地修改，且不可重复调用（时间转换非幂等）。
func (in *Input) Normalize() error {
	// 1) 时间转 UTC（与 normalize_qso_to_utc 一致，EYEBALL 不转换）
	if in.QSOType != "EYEBALL" {
		src := in.InputTimezone
		if src != "Asia/Shanghai" {
			src = "UTC"
		}
		d, t, err := timeutil.ConvertQsoDateTime(in.QSODate, in.TimeOn, src, timeutil.UTC)
		if err != nil {
			return err
		}
		in.QSODate, in.TimeOn = d, t
	}

	// 2) _auto_fill_freq_band
	in.Call = strings.ToUpper(strings.TrimSpace(in.Call))

	if in.QSOType == "EYEBALL" {
		in.TimeOn = ""
		in.Mode = ""
		in.RstSent = ""
		in.RstRcvd = ""
		in.Freq = ""
		in.Band = ""
		in.TxFreq = ""
		in.RxFreq = ""
		in.SatName = ""
		in.SatMode = ""
	}

	if in.Freq != "" {
		if b := FreqToBand(in.Freq); b != "" {
			in.Band = b
		} else if in.Band == "" {
			in.Band = ""
		}
	}
	return nil
}

// ===== 数据访问 =====

// Store 是 QSO 数据访问层。
type Store struct {
	db *sql.DB
}

// New 创建 Store。
func New(db *sql.DB) *Store { return &Store{db: db} }

// selectCols 是 SELECT 使用的列（显式，不依赖列序）。
// created_at 用 CAST AS TEXT 强制按字符串返回，避免 modernc.org/sqlite
// 将声明为 TIMESTAMP 的列自动解析为 time.Time（其序列化格式与 v1.x 不同）。
const selectCols = "id, call, qso_date, time_on, band, mode, rst_sent, rst_rcvd, qsl_status, comment, qso_type, freq, tx_freq, rx_freq, sat_name, sat_mode, is_sk, qth, CAST(created_at AS TEXT)"

// insertCols 是 INSERT/UPDATE 使用的列。
const insertCols = "call, qso_date, time_on, band, mode, rst_sent, rst_rcvd, qsl_status, comment, qso_type, freq, tx_freq, rx_freq, sat_name, sat_mode, is_sk, qth"

// scanQSO 把一行扫描为 QSO。
func scanQSO(sc interface{ Scan(dest ...any) error }) (*QSO, error) {
	var q QSO
	var qsDate, timeOn, band, mode, rstSent, rstRcvd, qslStatus, comment, qsoType sql.NullString
	var freq, txFreq, rxFreq, satName, satMode, qth, createdAt sql.NullString
	var isSk sql.NullInt64
	err := sc.Scan(
		&q.ID, &q.Call,
		&qsDate, &timeOn, &band, &mode, &rstSent, &rstRcvd, &qslStatus, &comment,
		&qsoType, &freq, &txFreq, &rxFreq, &satName, &satMode, &isSk, &qth, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	q.QSODate = ns(qsDate)
	q.TimeOn = ns(timeOn)
	q.Band = ns(band)
	q.Mode = ns(mode)
	q.RstSent = ns(rstSent)
	q.RstRcvd = ns(rstRcvd)
	q.QSLStatus = ns(qslStatus)
	q.Comment = ns(comment)
	q.QSOType = ns(qsoType)
	q.Freq = ns(freq)
	q.TxFreq = ns(txFreq)
	q.RxFreq = ns(rxFreq)
	q.SatName = ns(satName)
	q.SatMode = ns(satMode)
	q.IsSK = ni(isSk)
	q.QTH = ns(qth)
	q.CreatedAt = ns(createdAt)
	return &q, nil
}

// inputValues 返回 Input 对应的 INSERT 参数（17 列，与 v1.x 一致）。
func inputValues(in *Input) []any {
	isSK := 0
	if in.IsSK != 0 {
		isSK = 1
	}
	qsl := in.QSLStatus
	if qsl == "" {
		qsl = "未发送"
	}
	qsoType := in.QSOType
	if qsoType == "" {
		qsoType = "NORMAL"
	}
	return []any{
		in.Call, in.QSODate, in.TimeOn, in.Band, in.Mode,
		in.RstSent, in.RstRcvd, qsl, in.Comment, qsoType,
		in.Freq, in.TxFreq, in.RxFreq, in.SatName, in.SatMode, isSK, in.QTH,
	}
}

// Insert 新增一条 QSO（内部先规范化），返回新记录 id。
func (s *Store) Insert(in *Input) (int64, error) {
	if err := in.Normalize(); err != nil {
		return 0, err
	}
	return s.InsertNormalized(in)
}

// InsertNormalized 插入一条已规范化的 QSO（不再做规范化）。
func (s *Store) InsertNormalized(in *Input) (int64, error) {
	res, err := s.db.Exec(
		"INSERT INTO logs ("+insertCols+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		inputValues(in)...,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertBatch 批量插入 ADIF 记录（单事务），返回插入数量。
func (s *Store) InsertBatch(records []*Input) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO logs (" + insertCols + ") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, rec := range records {
		if err := rec.Normalize(); err != nil {
			return 0, err
		}
		if _, err := stmt.Exec(inputValues(rec)...); err != nil {
			return 0, err
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

// Update 更新一条 QSO，返回是否存在该记录。
func (s *Store) Update(id int64, in *Input) (bool, error) {
	if err := in.Normalize(); err != nil {
		return false, err
	}
	res, err := s.db.Exec(
		"UPDATE logs SET call=?, qso_date=?, time_on=?, band=?, mode=?, rst_sent=?, rst_rcvd=?, qsl_status=?, comment=?, qso_type=?, freq=?, tx_freq=?, rx_freq=?, sat_name=?, sat_mode=?, is_sk=?, qth=? WHERE id=?",
		append(inputValues(in), id)...,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdateQSLStatus 更新单条记录的 QSL 状态。
func (s *Store) UpdateQSLStatus(id int64, status string) (bool, error) {
	res, err := s.db.Exec("UPDATE logs SET qsl_status=? WHERE id=?", status, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Delete 删除一条记录。
func (s *Store) Delete(id int64) (bool, error) {
	res, err := s.db.Exec("DELETE FROM logs WHERE id=?", id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// CheckDuplicate 检测重复记录（呼号+日期+时间+波段+模式 五字段）。
func (s *Store) CheckDuplicate(call, date, timeOn, band, mode string) (*QSO, error) {
	row := s.db.QueryRow(
		"SELECT "+selectCols+" FROM logs WHERE call = ? AND qso_date = ? AND time_on = ? AND band = ? AND mode = ? LIMIT 1",
		call, date, timeOn, band, mode,
	)
	q, err := scanQSO(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return q, nil
}

// CheckDuplicateEyeball 检测重复的 Eyeball 记录（呼号+日期）。
func (s *Store) CheckDuplicateEyeball(call, date string) (*QSO, error) {
	row := s.db.QueryRow(
		"SELECT "+selectCols+" FROM logs WHERE call = ? AND qso_date = ? AND qso_type = 'EYEBALL' LIMIT 1",
		call, date,
	)
	q, err := scanQSO(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return q, nil
}

// CheckDuplicatesBatch 批量检测重复记录。
func (s *Store) CheckDuplicatesBatch(records []*Input) ([]Duplicate, error) {
	var dups []Duplicate
	for _, rec := range records {
		filled := *rec
		fillBandOnly(&filled)
		existing, err := s.CheckDuplicate(filled.Call, filled.QSODate, filled.TimeOn, filled.Band, filled.Mode)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			dups = append(dups, Duplicate{Record: rec, ExistingID: existing.ID})
		}
	}
	return dups, nil
}

// fillBandOnly 仅执行“呼号大写 + 频率→波段 + Eyeball 清理”（对齐 _auto_fill_freq_band）。
func fillBandOnly(in *Input) {
	in.Call = strings.ToUpper(strings.TrimSpace(in.Call))
	if in.QSOType == "EYEBALL" {
		in.TimeOn, in.Mode, in.RstSent, in.RstRcvd = "", "", "", ""
		in.Freq, in.Band, in.TxFreq, in.RxFreq, in.SatName, in.SatMode = "", "", "", "", "", ""
	}
	if in.Freq != "" {
		if b := FreqToBand(in.Freq); b != "" {
			in.Band = b
		} else if in.Band == "" {
			in.Band = ""
		}
	}
}

// buildWhere 根据筛选条件构建 WHERE 子句与参数。
func buildWhere(f Filters) (string, []any) {
	conds := []string{"1=1"}
	args := []any{}
	if f.Call != "" {
		conds = append(conds, "call LIKE ? ESCAPE '\\'")
		args = append(args, "%"+escapeLike(f.Call)+"%")
	}
	if f.Band != "" {
		conds = append(conds, "band = ?")
		args = append(args, f.Band)
	}
	if f.Mode != "" {
		conds = append(conds, "mode = ?")
		args = append(args, f.Mode)
	}
	if f.QSLStatus != "" {
		conds = append(conds, "qsl_status = ?")
		args = append(args, f.QSLStatus)
	}
	if f.QSOType != "" {
		conds = append(conds, "qso_type = ?")
		args = append(args, f.QSOType)
	}
	if f.DateFrom != "" {
		conds = append(conds, "qso_date >= ?")
		args = append(args, strings.ReplaceAll(f.DateFrom, "-", ""))
	}
	if f.DateTo != "" {
		conds = append(conds, "qso_date <= ?")
		args = append(args, strings.ReplaceAll(f.DateTo, "-", ""))
	}
	if f.IsSK != "" {
		if v, err := strconv.Atoi(f.IsSK); err == nil {
			conds = append(conds, "is_sk = ?")
			args = append(args, v)
		}
	}
	return strings.Join(conds, " AND "), args
}

// List 分页查询记录（管理接口，返回原始 UTC）。
func (s *Store) List(f Filters, page, pageSize int) (*Paginated, error) {
	where, args := buildWhere(f)

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM logs WHERE "+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	sortBy := f.SortBy
	sortOrder := f.SortOrder
	if _, ok := map[string]bool{"qso_date": true, "time_on": true, "call": true, "band": true, "mode": true, "created_at": true}[sortBy]; !ok {
		sortBy = "qso_date"
	}
	if strings.ToLower(sortOrder) != "asc" && strings.ToLower(sortOrder) != "desc" {
		sortOrder = "desc"
	}
	order := sortBy + " " + sortOrder
	if sortBy == "qso_date" {
		order = "qso_date " + sortOrder + ", time_on " + sortOrder
	}

	offset := (page - 1) * pageSize
	rows, err := s.db.Query(
		"SELECT "+selectCols+" FROM logs WHERE "+where+" ORDER BY "+order+" LIMIT ? OFFSET ?",
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	return &Paginated{Logs: logs, Total: total, Page: page, PageSize: pageSize}, nil
}

// Recent 分页查询最近记录（公开接口，仅 band/mode/qso_type 筛选）。
func (s *Store) Recent(band, mode, qsoType string, page, pageSize int) (*Paginated, error) {
	conds := []string{"1=1"}
	args := []any{}
	if band != "" {
		conds = append(conds, "band = ?")
		args = append(args, band)
	}
	if mode != "" {
		conds = append(conds, "mode = ?")
		args = append(args, mode)
	}
	if qsoType != "" {
		conds = append(conds, "qso_type = ?")
		args = append(args, qsoType)
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM logs WHERE "+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	offset := (page - 1) * pageSize
	rows, err := s.db.Query(
		"SELECT "+selectCols+" FROM logs WHERE "+where+" ORDER BY qso_date DESC, time_on DESC LIMIT ? OFFSET ?",
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	return &Paginated{Logs: logs, Total: total, Page: page, PageSize: pageSize}, nil
}

// AllFiltered 获取筛选后的全部记录（用于导出），按日期/时间倒序。
func (s *Store) AllFiltered(f Filters) ([]*QSO, error) {
	// 导出仅使用这些维度（不含 is_sk、sort）。
	where, args := buildWhere(Filters{
		Call: f.Call, Band: f.Band, Mode: f.Mode, QSLStatus: f.QSLStatus,
		QSOType: f.QSOType, DateFrom: f.DateFrom, DateTo: f.DateTo,
	})
	rows, err := s.db.Query(
		"SELECT "+selectCols+" FROM logs WHERE "+where+" ORDER BY qso_date DESC, time_on DESC",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// ByIDs 按 ID 列表获取记录。
func (s *Store) ByIDs(ids []int64) ([]*QSO, error) {
	if len(ids) == 0 {
		return []*QSO{}, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(
		"SELECT "+selectCols+" FROM logs WHERE id IN ("+ph+") ORDER BY qso_date DESC, time_on DESC",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// DeleteBatch 批量删除，返回删除数量。
func (s *Store) DeleteBatch(ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > 500 {
		return 0, fmt.Errorf("单次批量删除上限为 500 条")
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	res, err := s.db.Exec("DELETE FROM logs WHERE id IN ("+ph+")", args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// UpdateStatusBatch 批量更新 QSL 状态。
func (s *Store) UpdateStatusBatch(ids []int64, status string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > 500 {
		return 0, fmt.Errorf("单次批量操作上限为 500 条")
	}
	valid := false
	for _, st := range QSLStatuses {
		if st == status {
			valid = true
			break
		}
	}
	if !valid {
		return 0, fmt.Errorf("无效的 QSL 状态: %s", status)
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := []any{status}
	for _, id := range ids {
		args = append(args, id)
	}
	res, err := s.db.Exec("UPDATE logs SET qsl_status = ? WHERE id IN ("+ph+")", args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// UpdateSKBatch 批量更新 SK 标记。
func (s *Store) UpdateSKBatch(ids []int64, isSK int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > 500 {
		return 0, fmt.Errorf("单次批量操作上限为 500 条")
	}
	if isSK != 0 && isSK != 1 {
		return 0, fmt.Errorf("无效的 SK 标记: %d", isSK)
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := []any{isSK}
	for _, id := range ids {
		args = append(args, id)
	}
	res, err := s.db.Exec("UPDATE logs SET is_sk = ? WHERE id IN ("+ph+")", args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func scanRows(rows *sql.Rows) ([]*QSO, error) {
	out := []*QSO{}
	for rows.Next() {
		q, err := scanQSO(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ===== CSV 导出 =====

// csvHeader 与 v1.x export_csv 表头一致。
var csvHeader = []string{
	"CALL", "DATE", "TIME", "BAND", "FREQ", "MODE",
	"RST_SENT", "RST_RCVD", "QSL_STATUS", "COMMENT",
	"QSO_TYPE", "TX_FREQ", "RX_FREQ", "SAT_NAME", "SAT_MODE", "IS_SK", "QTH",
}

// ExportCSV 将记录导出为 CSV（UTF-8 BOM + CRLF，与 v1.x 一致）。
func ExportCSV(records []*QSO) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\ufeff") // UTF-8 BOM
	w := csv.NewWriter(&buf)
	w.UseCRLF = true // 与 v1.x 的 csv.writer 默认 \r\n 保持一致

	if err := w.Write(csvHeader); err != nil {
		return nil, err
	}
	for _, rec := range records {
		freq := strPtr(rec.Freq)
		if freq == "" {
			freq = BandFreqMap[strPtr(rec.Band)]
		}
		qsoType := strPtr(rec.QSOType)
		if qsoType == "" {
			qsoType = "NORMAL"
		}
		row := []string{
			rec.Call,
			strPtr(rec.QSODate),
			strPtr(rec.TimeOn),
			strPtr(rec.Band),
			freq,
			strPtr(rec.Mode),
			strPtr(rec.RstSent),
			strPtr(rec.RstRcvd),
			strPtr(rec.QSLStatus),
			strPtr(rec.Comment),
			qsoType,
			strPtr(rec.TxFreq),
			strPtr(rec.RxFreq),
			strPtr(rec.SatName),
			strPtr(rec.SatMode),
			strconv.Itoa(intPtr(rec.IsSK)),
			strPtr(rec.QTH),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
