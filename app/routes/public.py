from fastapi import APIRouter, Query
from app.database import get_recent_logs, search_logs_by_call

router = APIRouter(prefix="/api", tags=["public"])


@router.get("/recent")
def recent_logs(limit: int = Query(20, ge=1, le=100)):
    return {"logs": get_recent_logs(limit)}


@router.get("/search")
def search_logs(call: str = Query(..., min_length=1)):
    return {"logs": search_logs_by_call(call)}
