// Package timeutil 提供 UTC 与北京时间之间的 QSO 日期/时间转换。
//
// 数据库统一以 UTC 存储 qso_date（YYYYMMDD）与 time_on（HHMM）。
// 仅支持两个时区：UTC 与 Asia/Shanghai（UTC+8，无夏令时），因此无需时区数据库。
package timeutil

import (
	"fmt"
	"time"
)

const (
	UTC     = "UTC"
	Beijing = "Asia/Shanghai"
)

// validTimezones 与 v1.x VALID_TIMEZONES 保持一致。
var validTimezones = map[string]bool{
	UTC:     true,
	Beijing: true,
}

// NormalizeTimezone 返回合法时区名，非法值时回退到默认时区。
func NormalizeTimezone(value, def string) string {
	if validTimezones[value] {
		return value
	}
	return def
}

// tzOffset 返回时区相对 UTC 的秒数偏移。
func tzOffset(tz string) int {
	if tz == Beijing {
		return 8 * 60 * 60
	}
	return 0
}

// ConvertQsoDateTime 在源时区与目标时区之间转换 QSO 日期/时间。
//
// 返回 (date, time, err)。与 v1.x行为对齐：
//   - 输入过短时原样返回、err 为 nil；
//   - 日期/时间无法解析时返回 err（等价 v1.x 的 ValueError），由调用方决定跳过。
func ConvertQsoDateTime(qsoDate, timeOn, sourceTZ, targetTZ string) (string, string, error) {
	if len(qsoDate) < 8 || len(timeOn) < 4 {
		return qsoDate, timeOn, nil
	}

	src := NormalizeTimezone(sourceTZ, UTC)
	dst := NormalizeTimezone(targetTZ, UTC)
	if src == dst {
		return qsoDate[:8], timeOn[:4], nil
	}

	t, err := time.Parse("200601021504", qsoDate[:8]+timeOn[:4])
	if err != nil {
		return "", "", fmt.Errorf("invalid qso datetime: %w", err)
	}

	// 解析结果本身无时区，直接按固定偏移相减/相加完成转换。
	shift := tzOffset(dst) - tzOffset(src)
	t = t.Add(time.Duration(shift) * time.Second)

	return t.Format("20060102"), t.Format("1504"), nil
}

// NormalizeQsoToUTC 把录入数据按 input_timezone 归一化为 UTC 存储。
//
// 返回一个去除 input_timezone 字段的新 map（string → string）。
// EYEBALL 类型不转换时间。
func NormalizeQsoToUTC(data map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(data))
	for k, v := range data {
		if k != "input_timezone" {
			out[k] = v
		}
	}

	src := NormalizeTimezone(data["input_timezone"], UTC)
	if data["qso_type"] != "EYEBALL" {
		d, t, err := ConvertQsoDateTime(data["qso_date"], data["time_on"], src, UTC)
		if err != nil {
			return nil, err
		}
		out["qso_date"], out["time_on"] = d, t
	}
	return out, nil
}

// ConvertRecordTimezone 把 UTC 记录转换到目标显示时区（返回浅拷贝）。
// EYEBALL 类型不转换时间。
func ConvertRecordTimezone(record map[string]string, targetTZ string) map[string]string {
	out := make(map[string]string, len(record))
	for k, v := range record {
		out[k] = v
	}
	if out["qso_type"] != "EYEBALL" {
		d, t, err := ConvertQsoDateTime(out["qso_date"], out["time_on"], UTC, NormalizeTimezone(targetTZ, Beijing))
		if err == nil {
			out["qso_date"], out["time_on"] = d, t
		}
	}
	return out
}
