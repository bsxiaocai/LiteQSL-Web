import re
from datetime import datetime, timezone
from app.database import freq_to_band
from app.version import ADIF_VERSION, APP_VERSION

QSL_STATUS_TO_ADIF = {
    "无法考证": ("I", "I", ""),
    "未发送": ("N", "N", ""),
    "已发送": ("Y", "N", ""),
    "已收到": ("N", "Y", ""),
    "无需发送": ("I", "N", ""),
    "电子确认": ("N", "N", "Y"),
}


def parse_adif(content: str) -> list[dict]:
    """Parse ADIF content string into a list of QSO record dicts.

    支持的 ADIF 字段：
    - 标准字段：CALL, QSO_DATE, TIME_ON, BAND, MODE, RST_SENT, RST_RCVD,
      QSL_SENT, QSL_RCVD, EQSL_QSL_RCVD, LOTW_QSL_RCVD, COMMENT/NOTES
    - 频率字段：FREQ（主频率 MHz）, FREQ_RX（接收频率 MHz）
    - 卫星字段：TX_FREQ, RX_FREQ, SAT_NAME, PROP_MODE
    - 扩展字段：GRIDSQUARE, NAME, COUNTRY, OPERATOR, MY_CALLSIGN/STATION_CALLSIGN
    """
    records = []

    # Split by <eor> or <EOR>（不区分大小写）
    parts = re.split(r"<EOR>", content, flags=re.IGNORECASE)

    for part in parts:
        record = {}
        # Find all <TAG:LENGTH>VALUE patterns（不区分大小写）
        pattern = r"<(\w+):(\d+)[^>]*>([^<]*)"
        matches = re.findall(pattern, part, re.IGNORECASE)

        if not matches:
            continue

        for tag, length, value in matches:
            value = value.strip()
            tag_lower = tag.lower()

            if tag_lower == "call":
                record["call"] = value
            elif tag_lower == "qso_date":
                record["qso_date"] = value[:8] if len(value) >= 8 else value
            elif tag_lower == "time_on":
                record["time_on"] = value[:4] if len(value) >= 4 else value
            elif tag_lower == "band":
                record["band"] = value.lower() if value else ""
            elif tag_lower == "mode":
                record["mode"] = value
            elif tag_lower == "rst_sent":
                record["rst_sent"] = value
            elif tag_lower == "rst_rcvd":
                record["rst_rcvd"] = value
            elif tag_lower == "app_liteqsl_status":
                record["qsl_status"] = value
            elif tag_lower == "qsl_status":
                record["_legacy_qsl_status"] = value
            elif tag_lower == "qsl_sent":
                record["_qsl_sent"] = value.upper()
            elif tag_lower == "qsl_rcvd":
                record["_qsl_rcvd"] = value.upper()
            elif tag_lower in ("eqsl_qsl_rcvd", "lotw_qsl_rcvd"):
                if value.upper() == "Y":
                    record["_electronic_confirmed"] = True
            elif tag_lower == "comment" or tag_lower == "notes":
                record["comment"] = value
            elif tag_lower == "gridsquare":
                record["gridsquare"] = value
            elif tag_lower == "name":
                record["name"] = value
            elif tag_lower == "country":
                record["country"] = value
            elif tag_lower == "operator":
                record["operator"] = value
            elif tag_lower == "my_callsign" or tag_lower == "station_callsign":
                record["my_callsign"] = value
            # ===== 频率字段 =====
            elif tag_lower == "freq":
                record["freq"] = _normalize_mhz(value)
            elif tag_lower == "freq_rx":
                record["rx_freq"] = _normalize_mhz(value)
            # 兼容旧版 LiteQSL-Web 导出的非标准双频字段
            elif tag_lower == "tx_freq":
                record["tx_freq"] = _normalize_legacy_frequency(value)
            elif tag_lower == "rx_freq":
                record["rx_freq"] = _normalize_legacy_frequency(value)
            elif tag_lower == "sat_name":
                record["sat_name"] = value
            elif tag_lower == "sat_mode":
                record["sat_mode"] = value
            elif tag_lower == "prop_mode":
                record["_prop_mode"] = value  # 暂存，用于推导 qso_type

        if record.get("call"):
            record.setdefault("qso_date", "")
            record.setdefault("time_on", "")
            record.setdefault("band", "")
            record.setdefault("mode", "")
            record.setdefault("rst_sent", "")
            record.setdefault("rst_rcvd", "")
            record.setdefault("comment", "")
            record.setdefault("freq", "")
            record.setdefault("tx_freq", "")
            record.setdefault("rx_freq", "")
            record.setdefault("sat_name", "")
            record.setdefault("sat_mode", "")

            if "qsl_status" not in record:
                if record.pop("_electronic_confirmed", False):
                    record["qsl_status"] = "电子确认"
                elif record.pop("_qsl_rcvd", "") == "Y":
                    record["qsl_status"] = "已收到"
                else:
                    qsl_sent = record.pop("_qsl_sent", "")
                    if qsl_sent == "Y":
                        record["qsl_status"] = "已发送"
                    elif qsl_sent == "I":
                        record["qsl_status"] = "无需发送"
                    else:
                        record["qsl_status"] = record.pop("_legacy_qsl_status", "未发送")

            # ===== 自动推导 qso_type =====
            prop_mode = record.pop("_prop_mode", "").upper()
            if not record.get("qso_type"):
                if prop_mode == "SAT" or record.get("sat_name"):
                    record["qso_type"] = "SAT"
                elif prop_mode == "RPT" or record.get("tx_freq") and record.get("rx_freq"):
                    record["qso_type"] = "REP"
                else:
                    record["qso_type"] = "NORMAL"

            # ===== 自动补全：freq → band =====
            if record.get("freq") and not record.get("band"):
                auto_band = freq_to_band(record["freq"])
                if auto_band:
                    record["band"] = auto_band

            # ===== SAT 类型：如果只有 freq 没有 tx/rx，默认 freq → tx_freq =====
            if record.get("qso_type") == "SAT" and record.get("freq") and not record.get("tx_freq"):
                record["tx_freq"] = record["freq"]

            records.append(record)

    return records


def _normalize_mhz(value: str) -> str:
    """Normalize a standard ADIF frequency, whose unit is MHz."""
    if not value:
        return ""
    try:
        normalized = f"{float(value):.6f}".rstrip("0").rstrip(".")
    except (ValueError, TypeError):
        return ""
    return normalized


def _normalize_legacy_frequency(value: str) -> str:
    """Read legacy LiteQSL-Web TX_FREQ/RX_FREQ fields that used kHz."""
    normalized = _normalize_mhz(value)
    if not normalized:
        return ""
    number = float(normalized)
    if number > 2000:
        return f"{number / 1000:.6f}".rstrip("0").rstrip(".")
    return normalized


def export_adif(records: list[dict]) -> str:
    """Export a list of QSO record dicts to ADIF format string.

    导出字段包括：
    - 标准字段：CALL, QSO_DATE, TIME_ON, BAND, MODE, RST_SENT, RST_RCVD,
      QSL_SENT, QSL_RCVD, FREQ, FREQ_RX
    - 卫星字段：SAT_NAME, SAT_MODE, PROP_MODE

    注意：Eyeball QSO（线下见面）在 ADIF 导出时被忽略，
    因为 Eyeball QSO 没有真实的频率和模式数据，不适合 ADIF 格式。
    """
    lines = []
    lines.append("ADIF Export from LiteQSL-Web")
    lines.append(f"Generated UTC: {datetime.now(timezone.utc).strftime('%Y%m%d %H%M%S')}")
    lines.append(f"<ADIF_VER:{len(ADIF_VERSION)}>{ADIF_VERSION}")
    lines.append("<PROGRAMID:11>LiteQSL-Web")
    lines.append(f"<PROGRAMVERSION:{len(APP_VERSION)}>{APP_VERSION}")
    lines.append("<EOH>")
    lines.append("")

    for rec in records:
        # ===== Eyeball QSO 在 ADIF 导出时被忽略 =====
        qso_type = rec.get("qso_type", "NORMAL")
        if qso_type == "EYEBALL":
            continue  # 跳过 Eyeball QSO，不写入 ADIF

        parts = []

        # ===== 标准字段 =====
        _append_adif_field(parts, "CALL", rec.get("call", ""))
        _append_adif_field(parts, "QSO_DATE", rec.get("qso_date", ""))
        _append_adif_field(parts, "TIME_ON", rec.get("time_on", ""))
        _append_adif_field(parts, "BAND", rec.get("band", ""))
        _append_adif_field(parts, "MODE", rec.get("mode", ""))
        _append_adif_field(parts, "RST_SENT", rec.get("rst_sent", ""))
        _append_adif_field(parts, "RST_RCVD", rec.get("rst_rcvd", ""))
        status = rec.get("qsl_status", "未发送")
        qsl_sent, qsl_rcvd, eqsl_rcvd = QSL_STATUS_TO_ADIF.get(
            status,
            QSL_STATUS_TO_ADIF["未发送"],
        )
        _append_adif_field(parts, "QSL_SENT", qsl_sent)
        _append_adif_field(parts, "QSL_RCVD", qsl_rcvd)
        _append_adif_field(parts, "EQSL_QSL_RCVD", eqsl_rcvd)
        _append_adif_field(parts, "APP_LITEQSL_STATUS", status)
        _append_adif_field(parts, "COMMENT", rec.get("comment", ""))
        _append_adif_field(parts, "GRIDSQUARE", rec.get("gridsquare", ""))
        _append_adif_field(parts, "NAME", rec.get("name", ""))
        _append_adif_field(parts, "COUNTRY", rec.get("country", ""))
        _append_adif_field(parts, "OPERATOR", rec.get("operator", ""))
        _append_adif_field(parts, "MY_CALLSIGN", rec.get("my_callsign", ""))

        tx = rec.get("tx_freq", "")
        rx = rec.get("rx_freq", "")
        freq_mhz = rec.get("freq", "") or tx
        _append_adif_field(parts, "FREQ", _normalize_mhz(freq_mhz))
        _append_adif_field(parts, "FREQ_RX", _normalize_mhz(rx))

        # ===== 卫星/中继特殊字段 =====
        if qso_type == "SAT":
            _append_adif_field(parts, "PROP_MODE", "SAT")
            if rec.get("sat_name"):
                _append_adif_field(parts, "SAT_NAME", rec["sat_name"])
            _append_adif_field(parts, "SAT_MODE", rec.get("sat_mode", ""))
        elif qso_type == "REP":
            _append_adif_field(parts, "PROP_MODE", "RPT")

        parts.append("<EOR>")
        lines.append(" ".join(parts))

    return "\n".join(lines)


def _append_adif_field(parts: list, tag: str, value: str):
    """向 ADIF 输出追加一个字段（仅当值非空时）"""
    if value:
        text = str(value)
        parts.append(f"<{tag}:{len(text)}>{text}")
