from fastapi import APIRouter, Query
from app.database import get_recent_logs_paginated, search_logs_by_call_paginated, get_setting, get_async_db
from app.database import _escape_like

router = APIRouter(prefix="/api", tags=["public"])


@router.get("/station-info")
async def station_info():
    """获取电台公开信息（呼号、站点名称）"""
    callsign = await get_setting("callsign") or "BH7GUL"
    station_name = await get_setting("station_name") or "QSL & Log Management"
    return {
        "callsign": callsign,
        "station_name": station_name,
        "title": f"{callsign} {station_name}",
    }


@router.get("/recent")
async def recent_logs(
    band: str = Query(None),
    mode: str = Query(None),
    qso_type: str = Query(None),
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
):
    return await get_recent_logs_paginated(band=band, mode=mode, qso_type=qso_type, page=page, page_size=page_size)


@router.get("/search")
async def search_logs(
    call: str = Query(None),
    band: str = Query(None),
    mode: str = Query(None),
    date_from: str = Query(None),
    date_to: str = Query(None),
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
):
    """公开搜索接口，支持呼号、波段、模式、日期范围筛选"""
    # 构建筛选条件
    conditions = ["1=1"]
    params = []

    if call:
        conditions.append("call LIKE ? ESCAPE '\\'")
        params.append(f"%{_escape_like(call)}%")
    if band:
        conditions.append("band = ?")
        params.append(band)
    if mode:
        conditions.append("mode = ?")
        params.append(mode)
    if date_from:
        conditions.append("qso_date >= ?")
        params.append(date_from.replace("-", ""))
    if date_to:
        conditions.append("qso_date <= ?")
        params.append(date_to.replace("-", ""))

    where = " AND ".join(conditions)
    db = await get_async_db()
    try:
        async with db.execute(f"SELECT COUNT(*) as cnt FROM logs WHERE {where}", params) as cursor:
            row = await cursor.fetchone()
            total = row["cnt"]
        offset = (page - 1) * page_size
        async with db.execute(
            f"SELECT * FROM logs WHERE {where} ORDER BY qso_date DESC, time_on DESC LIMIT ? OFFSET ?",
            params + [page_size, offset],
        ) as cursor:
            rows = await cursor.fetchall()
            return {
                "logs": [dict(r) for r in rows],
                "total": total,
                "page": page,
                "page_size": page_size,
            }
    finally:
        await db.close()


@router.get("/bands")
async def get_bands():
    """获取可用的波段列表"""
    db = await get_async_db()
    try:
        async with db.execute(
            "SELECT DISTINCT band FROM logs WHERE band != '' ORDER BY band"
        ) as cursor:
            rows = await cursor.fetchall()
            return [row["band"] for row in rows]
    finally:
        await db.close()


@router.get("/modes")
async def get_modes():
    """获取可用的模式列表"""
    db = await get_async_db()
    try:
        async with db.execute(
            "SELECT DISTINCT mode FROM logs WHERE mode != '' ORDER BY mode"
        ) as cursor:
            rows = await cursor.fetchall()
            return [row["mode"] for row in rows]
    finally:
        await db.close()
