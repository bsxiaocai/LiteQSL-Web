import re
from datetime import datetime


def parse_adif(content: str) -> list[dict]:
    """Parse ADIF content string into a list of QSO record dicts."""
    records = []
    content = content.upper()

    # Split by <eor> or <EOR>
    parts = re.split(r"<EOR>", content)

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
                record["time_on"] = value[:6] if len(value) >= 6 else value
            elif tag_lower == "band":
                record["band"] = value
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

        if record.get("call"):
            record.setdefault("qso_date", "")
            record.setdefault("time_on", "")
            record.setdefault("band", "")
            record.setdefault("mode", "")
            record.setdefault("rst_sent", "")
            record.setdefault("rst_rcvd", "")
            record.setdefault("qsl_status", "未发送")
            record.setdefault("comment", "")
            records.append(record)

    return records


def export_adif(records: list[dict]) -> str:
    """Export a list of QSO record dicts to ADIF format string."""
    lines = []
    lines.append("ADIF Export from LiteQSL-Web")
    lines.append(f"Generated: {datetime.now().strftime('%Y%m%d %H%M%S')}")
    lines.append("<EOH>")
    lines.append("")

    field_map = {
        "call": "CALL",
        "qso_date": "QSO_DATE",
        "time_on": "TIME_ON",
        "band": "BAND",
        "mode": "MODE",
        "rst_sent": "RST_SENT",
        "rst_rcvd": "RST_RCVD",
        "qsl_status": "QSL_STATUS",
        "comment": "COMMENT",
    }

    for rec in records:
        parts = []
        for key, adif_tag in field_map.items():
            val = rec.get(key, "")
            if val:
                parts.append(f"<{adif_tag}:{len(val)}>{val}")
        parts.append("<EOR>")
        lines.append(" ".join(parts))

    return "\n".join(lines)
