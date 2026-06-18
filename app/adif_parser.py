import re
from datetime import datetime
from app.database import freq_to_band


def parse_adif(content: str) -> list[dict]:
    """Parse ADIF content string into a list of QSO record dicts.

    支持的 ADIF 字段：
    - 标准字段：CALL, QSO_DATE, TIME_ON, BAND, MODE, RST_SENT, RST_RCVD, QSL_STATUS, COMMENT/NOTES
    - 频率字段：FREQ（主频率 MHz）, FREQ_RX（接收频率 MHz）
    - 卫星字段：TX_FREQ, RX_FREQ, SAT_NAME, PROP_MODE
    """
    records = []
    content_upper = content.upper()

    # Split by <eor> or <EOR>
    parts = re.split(r"<EOR>", content_upper)

    for part in parts:
        record = {}
        # Find all <TAG:LENGTH>VALUE patterns
        pattern = r"<(\w+):(\d+)[^>]*>([^<]*)"
        matches = re.findall(pattern, part)

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
            elif tag_lower == "qsl_status":
                record["qsl_status"] = value
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
                # ADIF FREQ 单位是 kHz（如 "14270"），转换为 MHz（如 "14.270"）
                record["freq"] = _khz_to_mhz(value)
            elif tag_lower == "freq_rx":
                # 接收频率（用于跨段通联），同样 kHz → MHz
                record["rx_freq"] = _khz_to_mhz(value)
            # ===== 卫星/中继双频字段 =====
            elif tag_lower == "tx_freq":
                record["tx_freq"] = _khz_to_mhz(value)
            elif tag_lower == "rx_freq":
                record["rx_freq"] = _khz_to_mhz(value)
            elif tag_lower == "sat_name":
                record["sat_name"] = value
            elif tag_lower == "prop_mode":
                record["_prop_mode"] = value  # 暂存，用于推导 qso_type

        if record.get("call"):
            record.setdefault("qso_date", "")
            record.setdefault("time_on", "")
            record.setdefault("band", "")
            record.setdefault("mode", "")
            record.setdefault("rst_sent", "")
            record.setdefault("rst_rcvd", "")
            record.setdefault("qsl_status", "未发送")
            record.setdefault("comment", "")
            record.setdefault("freq", "")
            record.setdefault("tx_freq", "")
            record.setdefault("rx_freq", "")
            record.setdefault("sat_name", "")

            # ===== 自动推导 qso_type =====
            prop_mode = record.pop("_prop_mode", "")
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


def _khz_to_mhz(value: str) -> str:
    """将 ADIF 频率值（kHz 或 MHz 字符串）统一转为 MHz 字符串。

    ADIF 规范中 FREQ 单位为 kHz（如 "14270"），但部分软件导出为 MHz（如 "14.270"）。
    自动判断：如果值 > 1000，视为 kHz 并转换（业余频段最高 1300 MHz，>1000 的纯 MHz 值不合理）。
    """
    if not value:
        return ""
    try:
        f = float(value)
    except (ValueError, TypeError):
        return ""
    # 业余频段最高到 23cm (1300 MHz)，>1000 的值一定是 kHz 单位
    if f > 1000:
        mhz = f / 1000
        # 保留合理精度：去掉多余的尾零
        return f"{mhz:.6f}".rstrip("0").rstrip(".")
    return value


def export_adif(records: list[dict]) -> str:
    """Export a list of QSO record dicts to ADIF format string.

    导出字段包括：
    - 标准字段：CALL, QSO_DATE, TIME_ON, BAND, MODE, RST_SENT, RST_RCVD, QSL_STATUS, COMMENT
    - 频率字段：FREQ（MHz → kHz 转换）
    - 卫星字段：TX_FREQ, RX_FREQ, SAT_NAME, PROP_MODE, FREQ_RX

    注意：Eyeball QSO（线下见面）在 ADIF 导出时被忽略，
    因为 Eyeball QSO 没有真实的频率和模式数据，不适合 ADIF 格式。
    """
    lines = []
    lines.append("ADIF Export from LiteQSL-Web")
    lines.append(f"Generated: {datetime.now().strftime('%Y%m%d %H%M%S')}")
    lines.append("<PROGRAMVERSION:5>1.2.0")
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
        _append_adif_field(parts, "QSL_STATUS", rec.get("qsl_status", ""))
        _append_adif_field(parts, "COMMENT", rec.get("comment", ""))
        _append_adif_field(parts, "GRIDSQUARE", rec.get("gridsquare", ""))
        _append_adif_field(parts, "NAME", rec.get("name", ""))
        _append_adif_field(parts, "COUNTRY", rec.get("country", ""))
        _append_adif_field(parts, "OPERATOR", rec.get("operator", ""))
        _append_adif_field(parts, "MY_CALLSIGN", rec.get("my_callsign", ""))

        # ===== 频率字段（MHz → kHz 转换） =====
        freq_mhz = rec.get("freq", "")
        if freq_mhz:
            freq_khz = _mhz_to_khz(freq_mhz)
            if freq_khz:
                _append_adif_field(parts, "FREQ", freq_khz)

        # ===== 卫星/中继特殊字段 =====
        if qso_type == "SAT":
            _append_adif_field(parts, "PROP_MODE", "SAT")
            if rec.get("sat_name"):
                _append_adif_field(parts, "SAT_NAME", rec["sat_name"])
            tx = rec.get("tx_freq", "")
            rx = rec.get("rx_freq", "")
            if tx:
                _append_adif_field(parts, "TX_FREQ", _mhz_to_khz(tx) or tx)
            if rx:
                _append_adif_field(parts, "RX_FREQ", _mhz_to_khz(rx) or rx)
                _append_adif_field(parts, "FREQ_RX", _mhz_to_khz(rx) or rx)
        elif qso_type == "REP":
            tx = rec.get("tx_freq", "")
            rx = rec.get("rx_freq", "")
            if tx:
                _append_adif_field(parts, "TX_FREQ", _mhz_to_khz(tx) or tx)
            if rx:
                _append_adif_field(parts, "RX_FREQ", _mhz_to_khz(rx) or rx)
                _append_adif_field(parts, "FREQ_RX", _mhz_to_khz(rx) or rx)

        parts.append("<EOR>")
        lines.append(" ".join(parts))

    return "\n".join(lines)


def _append_adif_field(parts: list, tag: str, value: str):
    """向 ADIF 输出追加一个字段（仅当值非空时）"""
    if value:
        parts.append(f"<{tag}:{len(value)}>{value}")


def _mhz_to_khz(value: str) -> str:
    """将 MHz 字符串转为 kHz 字符串（整数或保留适当精度）。

    例如："14.270" → "14270", "7.074" → "7074", "145.825" → "145825"
    """
    if not value:
        return ""
    try:
        f = float(value)
    except (ValueError, TypeError):
        return ""
    khz = f * 1000
    # 如果是整数，去掉小数部分
    if khz == int(khz):
        return str(int(khz))
    return str(khz)
