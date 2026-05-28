from fastapi import APIRouter, Query
from app.database import get_logs_paginated, search_logs_by_call

router = APIRouter(prefix="/api", tags=["public"])


@router.get("/recent")
def recent_logs(
    limit: int = Query(20, ge=1, le=100),
    band: str = Query(None),
    mode: str = Query(None),
):
    filters = {}
    if band:
        filters["band"] = band
    if mode:
        filters["mode"] = mode
    if filters:
        result = get_logs_paginated(filters, page=1, page_size=limit)
        return {"logs": result["logs"]}
    # 无筛选时使用简单查询
    from app.database import get_recent_logs
    return {"logs": get_recent_logs(limit)}


@router.get("/search")
def search_logs(call: str = Query(..., min_length=1)):
    return {"logs": search_logs_by_call(call)}
