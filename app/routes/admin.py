from datetime import datetime
from fastapi import APIRouter, Request, HTTPException, Query
from fastapi.responses import Response, FileResponse
from app.auth import check_admin, require_admin, verify_password, hash_password, validate_password_strength
from app.rate_limit import get_client_ip, check_rate_limit, record_failure, clear_attempts
from app.database import (
    insert_log,
    update_log,
    update_qsl_status,
    delete_log,
    get_all_logs,
    get_logs_paginated,
    insert_logs_batch,
    get_user,
    update_password,
    check_duplicate,
    check_duplicates_batch,
    export_csv,
    complete_first_login,
    freq_to_band,
    QSL_STATUSES,
    QSO_TYPES,
    QSO_TYPE_LABELS,
)
from app.adif_parser import parse_adif, export_adif
from app.backup import create_backup, list_backups, get_backup_path, delete_backup, restore_backup

router = APIRouter(prefix="/api/admin", tags=["admin"])


@router.post("/login")
async def login(request: Request):
    # 登录频率限制
    ip = get_client_ip(request)
    allowed, retry_after = check_rate_limit(ip)
    if not allowed:
        raise HTTPException(
            status_code=429,
            detail=f"登录失败次数过多，请 {retry_after} 秒后再试",
            headers={"Retry-After": str(retry_after)},
        )

    body = await request.json()
    username = body.get("username", "")
    password = body.get("password", "")
    if not username or not password:
        raise HTTPException(status_code=400, detail="请输入用户名和密码")

    user = get_user(username)
    if not user:
        record_failure(ip)
        raise HTTPException(status_code=401, detail="用户名或密码错误")

    is_valid, needs_upgrade = verify_password(password, user["password_hash"])
    if not is_valid:
        record_failure(ip)
        raise HTTPException(status_code=401, detail="用户名或密码错误")

    # 登录成功
    clear_attempts(ip)
    # 自动升级旧 SHA-256 哈希到 bcrypt
    if needs_upgrade:
        new_hash = hash_password(password)
        update_password(username, new_hash)
    request.session["username"] = username
    return {"ok": True, "first_login": bool(user.get("first_login", 0))}


@router.post("/logout")
def logout(request: Request):
    request.session.clear()
    return {"ok": True}


@router.get("/check")
def check_login(request: Request):
    if not check_admin(request):
        return {"logged_in": False}
    user = get_user(request.session["username"])
    return {"logged_in": True, "first_login": bool(user.get("first_login", 0)) if user else False}


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
    is_valid, _ = verify_password(old_password, user["password_hash"])
    if not is_valid:
        raise HTTPException(status_code=400, detail="旧密码错误")
    valid, msg = validate_password_strength(new_password)
    if not valid:
        raise HTTPException(status_code=400, detail=msg)
    new_hash = hash_password(new_password)
    update_password(username, new_hash)
    return {"ok": True}


@router.get("/first-login-status")
def first_login_status(request: Request):
    require_admin(request)
    user = get_user(request.session["username"])
    if not user:
        raise HTTPException(status_code=401, detail="用户不存在")
    return {"first_login": bool(user.get("first_login", 0))}


@router.post("/complete-first-login")
async def complete_first_login_endpoint(request: Request):
    require_admin(request)
    body = await request.json()
    old_password = body.get("old_password", "")
    new_username = body.get("new_username", "").strip()
    new_password = body.get("new_password", "")
    confirm_username = body.get("confirm_username", "").strip()
    confirm_password = body.get("confirm_password", "")

    username = request.session["username"]
    user = get_user(username)
    if not user:
        raise HTTPException(status_code=401, detail="用户不存在")

    # 验证旧密码
    is_valid, _ = verify_password(old_password, user["password_hash"])
    if not is_valid:
        raise HTTPException(status_code=400, detail="当前密码错误")

    # 校验新用户名
    if len(new_username) < 5:
        raise HTTPException(status_code=400, detail="用户名长度至少为 5 个字符")
    if new_username != confirm_username:
        raise HTTPException(status_code=400, detail="两次输入的用户名不一致")

    # 校验新密码
    valid, msg = validate_password_strength(new_password)
    if not valid:
        raise HTTPException(status_code=400, detail=msg)
    if new_password != confirm_password:
        raise HTTPException(status_code=400, detail="两次输入的密码不一致")

    # 更新
    new_hash = hash_password(new_password)
    if not complete_first_login(username, new_username, new_hash):
        raise HTTPException(status_code=400, detail="新用户名已被占用")

    # 更新 session
    request.session["username"] = new_username
    return {"ok": True}


@router.get("/qsl-statuses")
def get_statuses():
    return {"statuses": QSL_STATUSES}


@router.get("/qso-types")
def get_qso_types():
    """返回 QSO 类型列表（英文枚举 + 中文标签）"""
    return {
        "types": [{"value": t, "label": QSO_TYPE_LABELS.get(t, t)} for t in QSO_TYPES]
    }


@router.post("/logs")
async def add_log(request: Request):
    require_admin(request)
    data = await request.json()

    # 设置默认 qso_type
    if not data.get("qso_type"):
        data["qso_type"] = "NORMAL"

    qso_type = data.get("qso_type", "NORMAL")

    # 根据 QSO 类型设置不同的必填字段
    if qso_type == "EYEBALL":
        # Eyeball 通联：只需要呼号、日期、卡片状态（不需要时间、频率、模式、RST）
        required_fields = ["call", "qso_date", "qsl_status"]
    elif qso_type == "SAT":
        # 卫星通联：需要呼号、日期、时间、卫星名称、上行/下行频率、模式、RST
        required_fields = ["call", "qso_date", "time_on", "sat_name", "tx_freq", "rx_freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"]
    elif qso_type == "REP":
        # 中继通联：需要呼号、日期、时间、上行/下行频率、模式、RST
        required_fields = ["call", "qso_date", "time_on", "tx_freq", "rx_freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"]
    else:
        # 标准通联：需要所有字段
        required_fields = ["call", "qso_date", "time_on", "freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"]

    for field in required_fields:
        if not data.get(field):
            raise HTTPException(status_code=400, detail=f"{field} 不能为空")

    # 自动推导 band（如果只有 freq 没有 band）
    if data.get("freq") and not data.get("band"):
        auto_band = freq_to_band(data["freq"])
        if auto_band:
            data["band"] = auto_band

    # 重复检测（force=true 时跳过）
    if not data.get("force"):
        # 需要 band 来做重复检测
        if data.get("band"):
            existing = check_duplicate(data["call"], data["qso_date"], data["time_on"], data["band"], data.get("mode", ""))
            if existing:
                raise HTTPException(
                    status_code=409,
                    detail=f"重复记录：已存在呼号 {data['call']} 在 {data['qso_date']} {data['time_on']} {data['band']} {data.get('mode', '')} 的记录 (ID: {existing['id']})",
                )
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
def list_all_logs(
    request: Request,
    page: int = Query(1, ge=1),
    page_size: int = Query(50, ge=1, le=200),
    call: str = Query(None),
    band: str = Query(None),
    mode: str = Query(None),
    qsl_status: str = Query(None),
    qso_type: str = Query(None),
):
    require_admin(request)
    filters = {}
    if call:
        filters["call"] = call
    if band:
        filters["band"] = band
    if mode:
        filters["mode"] = mode
    if qsl_status:
        filters["qsl_status"] = qsl_status
    if qso_type:
        filters["qso_type"] = qso_type
    return get_logs_paginated(filters, page, page_size)


@router.post("/import-adif")
async def import_adif(request: Request):
    require_admin(request)
    form = await request.form()
    file = form.get("file")
    force = form.get("force", "false").lower() == "true"
    if not file:
        raise HTTPException(status_code=400, detail="请上传文件")
    content = await file.read()
    text = content.decode("utf-8", errors="ignore")
    records = parse_adif(text)
    if not records:
        raise HTTPException(status_code=400, detail="未解析到有效记录")
    # 重复检测
    if not force:
        duplicates = check_duplicates_batch(records)
        if duplicates:
            return {
                "ok": False,
                "duplicates": [{"record": d["record"], "existing_id": d["existing"]["id"]} for d in duplicates],
                "duplicate_count": len(duplicates),
                "total": len(records),
            }
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


@router.get("/export-csv")
def export_csv_file(
    request: Request,
    band: str = Query(None),
    mode: str = Query(None),
    qsl_status: str = Query(None),
    qso_type: str = Query(None),
):
    require_admin(request)
    filters = {}
    if band:
        filters["band"] = band
    if mode:
        filters["mode"] = mode
    if qsl_status:
        filters["qsl_status"] = qsl_status
    if qso_type:
        filters["qso_type"] = qso_type
    if filters:
        records = get_logs_paginated(filters, page=1, page_size=99999)["logs"]
    else:
        records = get_all_logs()
    content = export_csv(records)
    filename = f"qsl_export_{datetime.now().strftime('%Y%m%d')}.csv"
    return Response(
        content=content.encode("utf-8"),
        media_type="text/csv",
        headers={"Content-Disposition": f"attachment; filename={filename}"},
    )


# ===== 数据库备份与恢复 =====

@router.post("/backup")
def backup_database(request: Request):
    require_admin(request)
    result = create_backup()
    return {"ok": True, "backup": result}


@router.get("/backups")
def backup_list(request: Request):
    require_admin(request)
    return {"backups": list_backups()}


@router.get("/backups/{filename}")
def download_backup(filename: str, request: Request):
    require_admin(request)
    path = get_backup_path(filename)
    if not path:
        raise HTTPException(status_code=404, detail="备份文件不存在")
    return FileResponse(
        path=path,
        filename=filename,
        media_type="application/octet-stream",
    )


@router.delete("/backups/{filename}")
def remove_backup(filename: str, request: Request):
    require_admin(request)
    if not delete_backup(filename):
        raise HTTPException(status_code=404, detail="备份文件不存在")
    return {"ok": True}


@router.post("/restore")
async def restore_database(request: Request):
    require_admin(request)
    body = await request.json()
    filename = body.get("filename", "")
    if not filename:
        raise HTTPException(status_code=400, detail="请指定备份文件名")
    result = restore_backup(filename)
    if not result["ok"]:
        raise HTTPException(status_code=400, detail=result["detail"])
    return result
