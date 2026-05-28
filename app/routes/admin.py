from fastapi import APIRouter, Request, HTTPException
from fastapi.responses import Response
from app.auth import check_admin, require_admin, verify_password, hash_password, validate_password_strength
from app.database import (
    insert_log,
    update_log,
    update_qsl_status,
    delete_log,
    get_all_logs,
    insert_logs_batch,
    get_user,
    update_password,
    QSL_STATUSES,
)
from app.adif_parser import parse_adif, export_adif

router = APIRouter(prefix="/api/admin", tags=["admin"])


@router.post("/login")
async def login(request: Request):
    body = await request.json()
    username = body.get("username", "")
    password = body.get("password", "")
    if not username or not password:
        raise HTTPException(status_code=400, detail="请输入用户名和密码")
    user = get_user(username)
    if not user or not verify_password(password, user["password_hash"]):
        raise HTTPException(status_code=401, detail="用户名或密码错误")
    request.session["username"] = username
    return {"ok": True}


@router.post("/logout")
def logout(request: Request):
    request.session.clear()
    return {"ok": True}


@router.get("/check")
def check_login(request: Request):
    return {"logged_in": check_admin(request)}


@router.post("/change-password")
async def change_password(request: Request):
    require_admin(request)
    body = await request.json()
    old_password = body.get("old_password", "")
    new_password = body.get("new_password", "")
    if not old_password or not new_password:
        raise HTTPException(status_code=400, detail="请输入旧密码和新密码")
    username = request.session["username"]
    user = get_user(username)
    if not verify_password(old_password, user["password_hash"]):
        raise HTTPException(status_code=400, detail="旧密码错误")
    valid, msg = validate_password_strength(new_password)
    if not valid:
        raise HTTPException(status_code=400, detail=msg)
    new_hash, _ = hash_password(new_password)
    update_password(username, new_hash)
    return {"ok": True}


@router.get("/qsl-statuses")
def get_statuses():
    return {"statuses": QSL_STATUSES}


@router.post("/logs")
async def add_log(request: Request):
    require_admin(request)
    data = await request.json()
    if not data.get("call"):
        raise HTTPException(status_code=400, detail="呼号不能为空")
    log_id = insert_log(data)
    return {"ok": True, "id": log_id}


@router.put("/logs/{log_id}")
async def edit_log(log_id: int, request: Request):
    require_admin(request)
    data = await request.json()
    if not update_log(log_id, data):
        raise HTTPException(status_code=404, detail="记录不存在")
    return {"ok": True}


@router.put("/logs/{log_id}/status")
async def change_status(log_id: int, request: Request):
    require_admin(request)
    body = await request.json()
    status = body.get("qsl_status", "")
    if status not in QSL_STATUSES:
        raise HTTPException(status_code=400, detail="无效的卡片状态")
    if not update_qsl_status(log_id, status):
        raise HTTPException(status_code=404, detail="记录不存在")
    return {"ok": True}


@router.delete("/logs/{log_id}")
async def remove_log(log_id: int, request: Request):
    require_admin(request)
    if not delete_log(log_id):
        raise HTTPException(status_code=404, detail="记录不存在")
    return {"ok": True}


@router.get("/logs")
def list_all_logs(request: Request):
    require_admin(request)
    return {"logs": get_all_logs()}


@router.post("/import-adif")
async def import_adif(request: Request):
    require_admin(request)
    form = await request.form()
    file = form.get("file")
    if not file:
        raise HTTPException(status_code=400, detail="请上传文件")
    content = await file.read()
    text = content.decode("utf-8", errors="ignore")
    records = parse_adif(text)
    if not records:
        raise HTTPException(status_code=400, detail="未解析到有效记录")
    count = insert_logs_batch(records)
    return {"ok": True, "count": count}


@router.get("/export-adif")
def export_adif_file(request: Request):
    require_admin(request)
    records = get_all_logs()
    content = export_adif(records)
    return Response(
        content=content,
        media_type="text/plain",
        headers={"Content-Disposition": "attachment; filename=export.adi"},
    )
