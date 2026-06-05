from fastapi import APIRouter, Query
from app.database import get_recent_logs_paginated, search_logs_by_call_paginated, get_setting

router = APIRouter(prefix="/api", tags=["public"])


@router.get("/station-info")
def station_info():
    """获取电台公开信息（呼号、站点名称）"""
    callsign = get_setting("callsign") or "BH7GUL"
    station_name = get_setting("station_name") or "QSL & Log Management"
    return {
        "callsign": callsign,
        "station_name": station_name,
        "title": f"{callsign} {station_name}",
    }


@router.get("/recent")
def recent_logs(
    band: str = Query(None),
    mode: str = Query(None),
    qso_type: str = Query(None),
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
):
    return get_recent_logs_paginated(band=band, mode=mode, qso_type=qso_type, page=page, page_size=page_size)


@router.get("/search")
def search_logs(
    call: str = Query(..., min_length=1),
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
):
    return search_logs_by_call_paginated(call, page=page, page_size=page_size)
