from datetime import datetime, timedelta, timezone

UTC = timezone.utc
BEIJING = timezone(timedelta(hours=8))

TIMEZONE_UTC = "UTC"
TIMEZONE_BEIJING = "Asia/Shanghai"
VALID_TIMEZONES = {TIMEZONE_UTC, TIMEZONE_BEIJING}


def normalize_timezone(value: str | None, default: str = TIMEZONE_UTC) -> str:
    return value if value in VALID_TIMEZONES else default


def convert_qso_datetime(
    qso_date: str,
    time_on: str,
    source_timezone: str,
    target_timezone: str,
) -> tuple[str, str]:
    """Convert a QSO date/time pair between UTC and Beijing time."""
    if not qso_date or len(qso_date) < 8 or not time_on or len(time_on) < 4:
        return qso_date or "", time_on or ""

    source_timezone = normalize_timezone(source_timezone)
    target_timezone = normalize_timezone(target_timezone)
    if source_timezone == target_timezone:
        return qso_date[:8], time_on[:4]

    source_tz = UTC if source_timezone == TIMEZONE_UTC else BEIJING
    target_tz = UTC if target_timezone == TIMEZONE_UTC else BEIJING
    value = datetime.strptime(qso_date[:8] + time_on[:4], "%Y%m%d%H%M")
    converted = value.replace(tzinfo=source_tz).astimezone(target_tz)
    return converted.strftime("%Y%m%d"), converted.strftime("%H%M")


def normalize_qso_to_utc(data: dict) -> dict:
    """Return a copy whose QSO date/time is normalized to UTC."""
    normalized = dict(data)
    source_timezone = normalize_timezone(
        normalized.pop("input_timezone", None),
        TIMEZONE_UTC,
    )
    if normalized.get("qso_type") != "EYEBALL":
        normalized["qso_date"], normalized["time_on"] = convert_qso_datetime(
            normalized.get("qso_date", ""),
            normalized.get("time_on", ""),
            source_timezone,
            TIMEZONE_UTC,
        )
    return normalized


def convert_record_timezone(record: dict, target_timezone: str) -> dict:
    converted = dict(record)
    if converted.get("qso_type") != "EYEBALL":
        converted["qso_date"], converted["time_on"] = convert_qso_datetime(
            converted.get("qso_date", ""),
            converted.get("time_on", ""),
            TIMEZONE_UTC,
            normalize_timezone(target_timezone, TIMEZONE_BEIJING),
        )
    return converted
