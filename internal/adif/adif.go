// Package adif 提供 ADIF 文件的解析与导出，行为对齐 v1.x app/adif_parser.py。
package adif

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bsxiaocai/LiteQSL-Web/internal/qso"
	"github.com/bsxiaocai/LiteQSL-Web/internal/version"
)

// qslStatusToADIF 与 v1.x QSL_STATUS_TO_ADIF 一致：(qsl_sent, qsl_rcvd, eqsl_rcvd)。
var qslStatusToADIF = map[string][3]string{
	"无法考证": {"I", "I", ""},
	"未发送":  {"N", "N", ""},
	"已发送":  {"Y", "N", ""},
	"已收到":  {"N", "Y", ""},
	"无需发送": {"I", "N", ""},
	"电子确认": {"N", "N", "Y"},
}

var (
	eorRe   = regexp.MustCompile(`(?i)<EOR>`)
	fieldRe = regexp.MustCompile(`(?i)<(\w+):(\d+)[^>]*>([^<]*)`)
)

// Parse 解析 ADIF 文本，返回 QSO 记录列表（对齐 parse_adif）。
func Parse(content string) []*qso.Input {
	var records []*qso.Input

	parts := eorRe.Split(content, -1)
	for _, part := range parts {
		matches := fieldRe.FindAllStringSubmatch(part, -1)
		if len(matches) == 0 {
			continue
		}

		rec := &qso.Input{}
		var legacyStatus, qslSent, qslRcvd string
		electronicConfirmed := false
		propMode := ""

		for _, m := range matches {
			tag := strings.ToLower(m[1])
			value := strings.TrimSpace(m[3])

			switch tag {
			case "call":
				rec.Call = value
			case "qso_date":
				rec.QSODate = trunc(value, 8)
			case "time_on":
				rec.TimeOn = trunc(value, 4)
			case "band":
				if value != "" {
					rec.Band = strings.ToLower(value)
				}
			case "mode":
				rec.Mode = value
			case "rst_sent":
				rec.RstSent = value
			case "rst_rcvd":
				rec.RstRcvd = value
			case "app_liteqsl_status":
				rec.QSLStatus = value
			case "qsl_status":
				legacyStatus = value
			case "qsl_sent":
				qslSent = strings.ToUpper(value)
			case "qsl_rcvd":
				qslRcvd = strings.ToUpper(value)
			case "eqsl_qsl_rcvd", "lotw_qsl_rcvd":
				if strings.ToUpper(value) == "Y" {
					electronicConfirmed = true
				}
			case "comment", "notes":
				rec.Comment = value
			case "freq":
				rec.Freq = normalizeMHz(value)
			case "freq_rx":
				rec.RxFreq = normalizeMHz(value)
			case "tx_freq":
				rec.TxFreq = normalizeLegacyFrequency(value)
			case "rx_freq":
				rec.RxFreq = normalizeLegacyFrequency(value)
			case "sat_name":
				rec.SatName = value
			case "sat_mode":
				rec.SatMode = value
			case "prop_mode":
				propMode = value
			}
		}

		if rec.Call == "" {
			continue
		}

		// 推导 QSL 状态（优先级与 v1.x 一致）。
		if rec.QSLStatus == "" {
			switch {
			case electronicConfirmed:
				rec.QSLStatus = "电子确认"
			case qslRcvd == "Y":
				rec.QSLStatus = "已收到"
			case qslSent == "Y":
				rec.QSLStatus = "已发送"
			case qslSent == "I":
				rec.QSLStatus = "无需发送"
			case legacyStatus != "":
				rec.QSLStatus = legacyStatus
			default:
				rec.QSLStatus = "未发送"
			}
		}

		// 推导 QSO 类型。
		pm := strings.ToUpper(propMode)
		if rec.QSOType == "" {
			switch {
			case pm == "SAT" || rec.SatName != "":
				rec.QSOType = "SAT"
			case pm == "RPT" || (rec.TxFreq != "" && rec.RxFreq != ""):
				rec.QSOType = "REP"
			default:
				rec.QSOType = "NORMAL"
			}
		}

		// freq → band。
		if rec.Freq != "" && rec.Band == "" {
			if b := qso.FreqToBand(rec.Freq); b != "" {
				rec.Band = b
			}
		}

		// SAT：仅 freq 无 tx_freq 时默认 tx_freq = freq。
		if rec.QSOType == "SAT" && rec.Freq != "" && rec.TxFreq == "" {
			rec.TxFreq = rec.Freq
		}

		records = append(records, rec)
	}
	return records
}

func trunc(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s
}

// normalizeMHz 规范化标准 ADIF 频率（MHz），对齐 _normalize_mhz。
func normalizeMHz(value string) string {
	if value == "" {
		return ""
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return ""
	}
	return trimFloat(strconv.FormatFloat(f, 'f', 6, 64))
}

// normalizeLegacyFrequency 读取 v1.x 遗留的 kHz 频率字段，归一化为 MHz。
func normalizeLegacyFrequency(value string) string {
	normalized := normalizeMHz(value)
	if normalized == "" {
		return ""
	}
	number, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return ""
	}
	if number > 2000 {
		return trimFloat(strconv.FormatFloat(number/1000, 'f', 6, 64))
	}
	return normalized
}

// trimFloat 去除小数末尾的 0 与小数点（沿用 v1.x 的去尾零处理）。
func trimFloat(s string) string {
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// Export 导出记录为 ADIF 字符串（对齐 export_adif）。
func Export(records []*qso.QSO) string {
	var lines []string
	lines = append(lines, "ADIF Export from LiteQSL-Web")
	lines = append(lines, "Generated UTC: "+time.Now().UTC().Format("20060102 150405"))
	lines = append(lines, fmt.Sprintf("<ADIF_VER:%d>%s", utf8.RuneCountInString(version.ADIFVersion), version.ADIFVersion))
	lines = append(lines, "<PROGRAMID:11>LiteQSL-Web")
	lines = append(lines, fmt.Sprintf("<PROGRAMVERSION:%d>%s", utf8.RuneCountInString(version.AppVersion), version.AppVersion))
	lines = append(lines, "<EOH>")
	lines = append(lines, "")

	for _, rec := range records {
		// Eyeball QSO 不写入 ADIF。
		if qsoType(rec) == "EYEBALL" {
			continue
		}

		var parts []string
		appendField(&parts, "CALL", rec.Call)
		appendField(&parts, "QSO_DATE", qsoPtr(rec.QSODate))
		appendField(&parts, "TIME_ON", qsoPtr(rec.TimeOn))
		appendField(&parts, "BAND", qsoPtr(rec.Band))
		appendField(&parts, "MODE", qsoPtr(rec.Mode))
		appendField(&parts, "RST_SENT", qsoPtr(rec.RstSent))
		appendField(&parts, "RST_RCVD", qsoPtr(rec.RstRcvd))

		status := qsoPtr(rec.QSLStatus)
		if status == "" {
			status = "未发送"
		}
		mapped, ok := qslStatusToADIF[status]
		if !ok {
			mapped = qslStatusToADIF["未发送"]
		}
		appendField(&parts, "QSL_SENT", mapped[0])
		appendField(&parts, "QSL_RCVD", mapped[1])
		appendField(&parts, "EQSL_QSL_RCVD", mapped[2])
		appendField(&parts, "APP_LITEQSL_STATUS", status)
		appendField(&parts, "COMMENT", qsoPtr(rec.Comment))

		tx := qsoPtr(rec.TxFreq)
		rx := qsoPtr(rec.RxFreq)
		freqMHz := qsoPtr(rec.Freq)
		if freqMHz == "" {
			freqMHz = tx
		}
		appendField(&parts, "FREQ", normalizeMHz(freqMHz))
		appendField(&parts, "FREQ_RX", normalizeMHz(rx))

		switch qsoType(rec) {
		case "SAT":
			appendField(&parts, "PROP_MODE", "SAT")
			if rec.SatName != nil && *rec.SatName != "" {
				appendField(&parts, "SAT_NAME", *rec.SatName)
			}
			appendField(&parts, "SAT_MODE", qsoPtr(rec.SatMode))
		case "REP":
			appendField(&parts, "PROP_MODE", "RPT")
		}

		parts = append(parts, "<EOR>")
		lines = append(lines, strings.Join(parts, " "))
	}
	return strings.Join(lines, "\n")
}

// appendField 仅当值非空时追加 ADIF 字段。长度按字符数计算（沿用 v1.x 的按字符数计长方式）。
func appendField(parts *[]string, tag, value string) {
	if value == "" {
		return
	}
	*parts = append(*parts, fmt.Sprintf("<%s:%d>%s", tag, utf8.RuneCountInString(value), value))
}

func qsoPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func qsoType(rec *qso.QSO) string {
	if rec.QSOType == nil {
		return "NORMAL"
	}
	return *rec.QSOType
}
