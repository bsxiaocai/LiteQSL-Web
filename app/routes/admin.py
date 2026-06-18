from datetime import datetime
from fastapi import APIRouter, Request, HTTPException, Query
from fastapi.responses import Response, FileResponse
from app.auth import (
    check_admin, require_admin, verify_password, hash_password,
    validate_password_strength, generate_csrf_token, validate_csrf_token,
)
from app.rate_limit import get_client_ip, check_rate_limit, record_failure, clear_attempts
from app.database import (
    insert_log,
    update_log,
    update_qsl_status,
    delete_log,
    get_all_logs,
    get_all_logs_filtered,
    get_logs_paginated,
    insert_logs_batch,
    get_user,
    update_password,
    check_duplicate,
    check_duplicate_eyeball,
    check_duplicates_batch,
    export_csv,
    complete_first_login,
    freq_to_band,
    QSL_STATUSES,
    QSO_TYPES,
    QSO_TYPE_LABELS,
    delete_logs_batch,
    update_logs_status_batch,
    update_logs_sk_batch,
    get_logs_by_ids,
)
from app.adif_parser import parse_adif, export_adif
from app.backup import create_backup, list_backups, get_backup_path, delete_backup, restore_backup

router = APIRouter(prefix="/api/admin", tags=["admin"])


@router.get("/csrf-token")
async def get_csrf_token(request: Request):
    """获取 CSRF Token（需先登录，首次登录期间也可用）"""
    await require_admin(request, allow_first_login=True)
    token = generate_csrf_token(request)
    return {"csrf_token": token}


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

    user = await get_user(username)
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
        await update_password(username, new_hash)
    request.session["username"] = username
    request.session["password_version"] = user.get("password_version", 1)
    return {"ok": True, "first_login": bool(user.get("first_login", 0))}


@router.post("/logout")
def logout(request: Request):
    request.session.clear()
    return {"ok": True}


@router.get("/check")
async def check_login(request: Request):
    if not await check_admin(request):
        return {"logged_in": False}
    user = await get_user(request.session["username"])
    return {"logged_in": True, "first_login": bool(user.get("first_login", 0)) if user else False}


@router.post("/change-password")
async def change_password(request: Request):
    await require_admin(request)
    validate_csrf_token(request)
    body = await request.json()
    old_password = body.get("old_password", "")
    new_password = body.get("new_password", "")
    if not old_password or not new_password:
        raise HTTPException(status_code=400, detail="请输入旧密码和新密码")
    username = request.session["username"]
    user = await get_user(username)
    is_valid, _ = verify_password(old_password, user["password_hash"])
    if not is_valid:
        raise HTTPException(status_code=400, detail="旧密码错误")
    valid, msg = validate_password_strength(new_password)
    if not valid:
        raise HTTPException(status_code=400, detail=msg)
    new_hash = hash_password(new_password)
    await update_password(username, new_hash)
    return {"ok": True}


@router.get("/first-login-status")
async def first_login_status(request: Request):
    await require_admin(request, allow_first_login=True)
    user = await get_user(request.session["username"])
    if not user:
        raise HTTPException(status_code=401, detail="用户不存在")
    return {"first_login": bool(user.get("first_login", 0))}


@router.post("/complete-first-login")
async def complete_first_login_endpoint(request: Request):
    await require_admin(request, allow_first_login=True)
    validate_csrf_token(request)
    body = await request.json()
    old_password = body.get("old_password", "")
    new_username = body.get("new_username", "").strip()
    new_password = body.get("new_password", "")
    confirm_username = body.get("confirm_username", "").strip()
    confirm_password = body.get("confirm_password", "")

    username = request.session["username"]
    user = await get_user(username)
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

    # 更新（先更新数据库，成功后再更新 session）
    new_hash = hash_password(new_password)
    if not await complete_first_login(username, new_username, new_hash):
        raise HTTPException(status_code=400, detail="新用户名已被占用")

    # 数据库更新成功后才更新 session
    request.session["username"] = new_username
    # 更新 session 中的密码版本（complete_first_login 不经过 update_password，手动+1）
    user_after = await get_user(new_username)
    if user_after:
        request.session["password_version"] = user_after.get("password_version", 1)
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
    await require_admin(request)
    validate_csrf_token(request)
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
        if qso_type == "EYEBALL":
            existing = await check_duplicate_eyeball(data["call"], data["qso_date"])
            if existing:
                raise HTTPException(
                    status_code=409,
                    detail=f"重复记录：已存在呼号 {data['call']} 在 {data['qso_date']} 的 Eyeball QSO 记录 (ID: {existing['id']})",
                )
        else:
            # 其他类型：按五字段检测重复（band 为空时用 freq 兜底）
            band_for_check = data.get("band", "")
            if not band_for_check and data.get("freq"):
                band_for_check = freq_to_band(data["freq"])
            if band_for_check:
                existing = await check_duplicate(data["call"], data["qso_date"], data["time_on"], band_for_check, data.get("mode", ""))
                if existing:
                    raise HTTPException(
                        status_code=409,
                        detail=f"重复记录：已存在呼号 {data['call']} 在 {data['qso_date']} {data['time_on']} {band_for_check} {data.get('mode', '')} 的记录 (ID: {existing['id']})",
                    )
    log_id = await insert_log(data)
    return {"ok": True, "id": log_id}


@router.put("/logs/{log_id}")
async def edit_log(log_id: int, request: Request):
    await require_admin(request)
    validate_csrf_token(request)
    data = await request.json()

    # 校验必填字段
    if not data.get("call"):
        raise HTTPException(status_code=400, detail="呼号不能为空")
    if not data.get("qso_date"):
        raise HTTPException(status_code=400, detail="日期不能为空")

    qso_type = data.get("qso_type", "NORMAL")
    if qso_type == "EYEBALL":
        # Eyeball QSO 只需要呼号、日期、卡片状态
        required_fields = ["call", "qso_date", "qsl_status"]
    elif qso_type == "SAT":
        required_fields = ["call", "qso_date", "time_on", "sat_name", "tx_freq", "rx_freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"]
    elif qso_type == "REP":
        required_fields = ["call", "qso_date", "time_on", "tx_freq", "rx_freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"]
    else:
        required_fields = ["call", "qso_date", "time_on", "freq", "mode", "rst_sent", "rst_rcvd", "qsl_status"]

    for field in required_fields:
        if not data.get(field):
            raise HTTPException(status_code=400, detail=f"{field} 不能为空")

    if not await update_log(log_id, data):
        raise HTTPException(status_code=404, detail="记录不存在")
    return {"ok": True}


@router.put("/logs/{log_id}/status")
async def change_status(log_id: int, request: Request):
    await require_admin(request)
    validate_csrf_token(request)
    body = await request.json()
    status = body.get("qsl_status", "")
    if status not in QSL_STATUSES:
        raise HTTPException(status_code=400, detail="无效的卡片状态")
    if not await update_qsl_status(log_id, status):
        raise HTTPException(status_code=404, detail="记录不存在")
    return {"ok": True}


@router.delete("/logs/{log_id}")
async def remove_log(log_id: int, request: Request):
    await require_admin(request)
    validate_csrf_token(request)
    if not await delete_log(log_id):
        raise HTTPException(status_code=404, detail="记录不存在")
    return {"ok": True}


@router.get("/logs")
async def list_all_logs(
    request: Request,
    page: int = Query(1, ge=1),
    page_size: int = Query(50, ge=1, le=200),
    call: str = Query(None),
    band: str = Query(None),
    mode: str = Query(None),
    qsl_status: str = Query(None),
    qso_type: str = Query(None),
    date_from: str = Query(None),
    date_to: str = Query(None),
    is_sk: str = Query(None),
    sort_by: str = Query(None),
    sort_order: str = Query(None),
):
    await require_admin(request)
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
    if date_from:
        filters["date_from"] = date_from
    if date_to:
        filters["date_to"] = date_to
    if is_sk is not None and is_sk != "":
        filters["is_sk"] = is_sk
    if sort_by:
        filters["sort_by"] = sort_by
    if sort_order:
        filters["sort_order"] = sort_order
    return await get_logs_paginated(filters, page, page_size)


@router.post("/import-adif")
async def import_adif(request: Request):
    await require_admin(request)
    validate_csrf_token(request)
    form = await request.form()
    file = form.get("file")
    force = form.get("force", "false").lower() == "true"
    if not file:
        raise HTTPException(status_code=400, detail="请上传文件")
    # 文件大小限制：10MB
    MAX_FILE_SIZE = 10 * 1024 * 1024
    content = await file.read()
    if len(content) > MAX_FILE_SIZE:
        raise HTTPException(status_code=413, detail="文件大小超过限制（最大 10MB）")
    # 尝试多种编码解码（UTF-8 优先，GBK/GB2312 兜底，避免静默丢弃数据）
    text = None
    for encoding in ("utf-8", "gbk", "gb2312", "latin-1"):
        try:
            text = content.decode(encoding)
            break
        except (UnicodeDecodeError, LookupError):
            continue
    if text is None:
        raise HTTPException(status_code=400, detail="文件编码无法识别，请使用 UTF-8 或 GBK 编码")
    records = parse_adif(text)
    if not records:
        raise HTTPException(status_code=400, detail="未解析到有效记录")
    # 重复检测
    if not force:
        duplicates = await check_duplicates_batch(records)
        if duplicates:
            return {
                "ok": False,
                "duplicates": [{"record": d["record"], "existing_id": d["existing"]["id"]} for d in duplicates],
                "duplicate_count": len(duplicates),
                "total": len(records),
            }
    count = await insert_logs_batch(records)
    return {"ok": True, "count": count}


@router.get("/export-adif")
async def export_adif_file(
    request: Request,
    band: str = Query(None),
    mode: str = Query(None),
    qsl_status: str = Query(None),
    qso_type: str = Query(None),
    date_from: str = Query(None),
    date_to: str = Query(None),
):
    await require_admin(request)
    filters = {}
    if band:
        filters["band"] = band
    if mode:
        filters["mode"] = mode
    if qsl_status:
        filters["qsl_status"] = qsl_status
    if qso_type:
        filters["qso_type"] = qso_type
    if date_from:
        filters["date_from"] = date_from
    if date_to:
        filters["date_to"] = date_to
    records = await get_all_logs_filtered(filters if filters else None)
    content = export_adif(records)
    filename = f"qsl_export_{datetime.now().strftime('%Y%m%d')}.adi"
    return Response(
        content=content,
        media_type="text/plain",
        headers={"Content-Disposition": f"attachment; filename={filename}"},
    )


@router.get("/export-csv")
async def export_csv_file(
    request: Request,
    band: str = Query(None),
    mode: str = Query(None),
    qsl_status: str = Query(None),
    qso_type: str = Query(None),
    date_from: str = Query(None),
    date_to: str = Query(None),
):
    await require_admin(request)
    filters = {}
    if band:
        filters["band"] = band
    if mode:
        filters["mode"] = mode
    if qsl_status:
        filters["qsl_status"] = qsl_status
    if qso_type:
        filters["qso_type"] = qso_type
    if date_from:
        filters["date_from"] = date_from
    if date_to:
        filters["date_to"] = date_to
    records = await get_all_logs_filtered(filters if filters else None)
    content = export_csv(records)
    filename = f"qsl_export_{datetime.now().strftime('%Y%m%d')}.csv"
    return Response(
        content=content.encode("utf-8"),
        media_type="text/csv",
        headers={"Content-Disposition": f"attachment; filename={filename}"},
    )


# ===== 数据库备份与恢复 =====

@router.post("/backup")
async def backup_database(request: Request):
    await require_admin(request)
    validate_csrf_token(request)
    result = create_backup()
    return {"ok": True, "backup": result}


@router.get("/backups")
async def backup_list(request: Request):
    await require_admin(request)
    return {"backups": list_backups()}


@router.get("/backups/{filename}")
async def download_backup(filename: str, request: Request):
    await require_admin(request)
    path = get_backup_path(filename)
    if not path:
        raise HTTPException(status_code=404, detail="备份文件不存在")
    return FileResponse(
        path=path,
        filename=filename,
        media_type="application/octet-stream",
    )


@router.delete("/backups/{filename}")
async def remove_backup(filename: str, request: Request):
    await require_admin(request)
    validate_csrf_token(request)
    if not delete_backup(filename):
        raise HTTPException(status_code=404, detail="备份文件不存在")
    return {"ok": True}


@router.post("/restore")
async def restore_database(request: Request):
    await require_admin(request)
    validate_csrf_token(request)
    body = await request.json()
    filename = body.get("filename", "")
    if not filename:
        raise HTTPException(status_code=400, detail="请指定备份文件名")
    result = restore_backup(filename)
    if not result["ok"]:
        raise HTTPException(status_code=400, detail=result["detail"])
    # 恢复后清除 session，强制重新登录（恢复的数据库可能有不同的用户/密码）
    request.session.clear()
    return result


# ===== 系统设置 =====

@router.get("/settings")
async def get_settings(request: Request):
    """获取所有系统设置"""
    await require_admin(request)
    from app.database import get_all_settings
    return {"settings": await get_all_settings()}


@router.put("/settings")
async def update_settings(request: Request):
    """更新系统设置"""
    await require_admin(request)
    validate_csrf_token(request)
    body = await request.json()
    from app.database import update_setting

    # 允许更新的设置项
    allowed_keys = {"callsign", "station_name"}
    updated = []
    for key, value in body.items():
        if key in allowed_keys:
            if not isinstance(value, str):
                raise HTTPException(status_code=400, detail=f"设置项 {key} 必须是字符串")
            value = value.strip().upper() if key == "callsign" else value.strip()
            await update_setting(key, value)
            updated.append(key)

    return {"ok": True, "updated": updated}


# ===== 批量操作 =====

@router.post("/logs/batch-delete")
async def batch_delete_logs(request: Request):
    """批量删除记录"""
    await require_admin(request)
    validate_csrf_token(request)
    body = await request.json()
    ids = body.get("ids", [])
    if not ids:
        raise HTTPException(status_code=400, detail="请选择要删除的记录")
    if not isinstance(ids, list) or not all(isinstance(i, int) for i in ids):
        raise HTTPException(status_code=400, detail="无效的记录 ID")
    try:
        deleted = await delete_logs_batch(ids)
        return {"ok": True, "deleted": deleted}
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/logs/batch-status")
async def batch_update_status(request: Request):
    """批量修改 QSL 状态"""
    await require_admin(request)
    validate_csrf_token(request)
    body = await request.json()
    ids = body.get("ids", [])
    status = body.get("status", "")
    if not ids:
        raise HTTPException(status_code=400, detail="请选择要修改的记录")
    if not status:
        raise HTTPException(status_code=400, detail="请选择目标状态")
    try:
        updated = await update_logs_status_batch(ids, status)
        return {"ok": True, "updated": updated}
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/logs/batch-sk")
async def batch_update_sk(request: Request):
    """批量修改 SK 标记"""
    await require_admin(request)
    validate_csrf_token(request)
    body = await request.json()
    ids = body.get("ids", [])
    is_sk = body.get("is_sk")
    if not ids:
        raise HTTPException(status_code=400, detail="请选择要修改的记录")
    if is_sk not in (0, 1):
        raise HTTPException(status_code=400, detail="无效的 SK 标记值")
    try:
        updated = await update_logs_sk_batch(ids, is_sk)
        return {"ok": True, "updated": updated}
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/logs/batch-export")
async def batch_export(request: Request):
    """批量导出选中记录"""
    await require_admin(request)
    validate_csrf_token(request)
    body = await request.json()
    ids = body.get("ids", [])
    format_type = body.get("format", "adif")
    if not ids:
        raise HTTPException(status_code=400, detail="请选择要导出的记录")
    if format_type not in ("adif", "csv"):
        raise HTTPException(status_code=400, detail="不支持的导出格式")
    records = await get_logs_by_ids(ids)
    if not records:
        raise HTTPException(status_code=404, detail="未找到指定记录")
    if format_type == "adif":
        content = export_adif(records)
        filename = f"qsl_export_{datetime.now().strftime('%Y%m%d')}.adi"
        return Response(
            content=content,
            media_type="text/plain",
            headers={"Content-Disposition": f"attachment; filename={filename}"},
        )
    else:
        content = export_csv(records)
        filename = f"qsl_export_{datetime.now().strftime('%Y%m%d')}.csv"
        return Response(
            content=content.encode("utf-8"),
            media_type="text/csv",
            headers={"Content-Disposition": f"attachment; filename={filename}"},
        )


# ===== 统计仪表盘 =====

@router.get("/stats/summary")
async def stats_summary(request: Request):
    """获取统计数据概览"""
    await require_admin(request)
    async with async_db() as db:
        async with db.execute("SELECT COUNT(*) as cnt FROM logs") as cursor:
            row = await cursor.fetchone()
            total_logs = row["cnt"]

        async with db.execute("SELECT COUNT(DISTINCT call) as cnt FROM logs") as cursor:
            row = await cursor.fetchone()
            total_callsigns = row["cnt"]

        async with db.execute(
            "SELECT COUNT(*) as cnt FROM logs WHERE qso_date >= date('now', 'start of month')"
        ) as cursor:
            row = await cursor.fetchone()
            this_month = row["cnt"]

        async with db.execute(
            "SELECT COUNT(*) as cnt FROM logs WHERE qso_date >= date('now', 'start of year')"
        ) as cursor:
            row = await cursor.fetchone()
            this_year = row["cnt"]

        async with db.execute(
            "SELECT COUNT(*) as cnt FROM logs WHERE qsl_status = '未发送'"
        ) as cursor:
            row = await cursor.fetchone()
            qsl_pending = row["cnt"]

        return {
            "total_logs": total_logs,
            "total_callsigns": total_callsigns,
            "this_month": this_month,
            "this_year": this_year,
            "qsl_pending": qsl_pending,
        }


@router.get("/stats/by-band")
async def stats_by_band(request: Request):
    """按波段统计"""
    await require_admin(request)
    async with async_db() as db:
        async with db.execute(
            "SELECT band, COUNT(*) as count FROM logs WHERE band != '' GROUP BY band ORDER BY count DESC"
        ) as cursor:
            rows = await cursor.fetchall()
            return [{"band": row["band"], "count": row["count"]} for row in rows]


@router.get("/stats/by-mode")
async def stats_by_mode(request: Request):
    """按模式统计"""
    await require_admin(request)
    async with async_db() as db:
        async with db.execute(
            "SELECT mode, COUNT(*) as count FROM logs WHERE mode != '' GROUP BY mode ORDER BY count DESC"
        ) as cursor:
            rows = await cursor.fetchall()
            return [{"mode": row["mode"], "count": row["count"]} for row in rows]


@router.get("/stats/by-type")
async def stats_by_type(request: Request):
    """按 QSO 类型统计"""
    await require_admin(request)
    async with async_db() as db:
        async with db.execute(
            "SELECT qso_type, COUNT(*) as count FROM logs GROUP BY qso_type ORDER BY count DESC"
        ) as cursor:
            rows = await cursor.fetchall()
            return [{"qso_type": row["qso_type"], "count": row["count"]} for row in rows]


@router.get("/stats/by-month")
async def stats_by_month(request: Request, months: int = Query(12, ge=1, le=60)):
    """按月统计通联数量"""
    await require_admin(request)
    async with async_db() as db:
        months_modifier = f"-{int(months)} months"
        async with db.execute(
            "SELECT substr(qso_date, 1, 7) as month, COUNT(*) as count "
            "FROM logs WHERE qso_date >= date('now', ?) "
            "GROUP BY month ORDER BY month",
            (months_modifier,)
        ) as cursor:
            rows = await cursor.fetchall()
            return [{"month": row["month"], "count": row["count"]} for row in rows]


@router.get("/stats/by-hour")
async def stats_by_hour(request: Request):
    """按小时统计通联分布"""
    await require_admin(request)
    async with async_db() as db:
        async with db.execute(
            "SELECT CAST(substr(time_on, 1, 2) AS INTEGER) as hour, COUNT(*) as count "
            "FROM logs WHERE time_on != '' AND length(time_on) >= 2 AND qso_type != 'EYEBALL' "
            "GROUP BY hour ORDER BY hour"
        ) as cursor:
            rows = await cursor.fetchall()
            return [{"hour": row["hour"], "count": row["count"]} for row in rows]


@router.get("/stats/top-calls")
async def stats_top_calls(request: Request, limit: int = Query(20, ge=1, le=100)):
    """Top 通联对象"""
    await require_admin(request)
    async with async_db() as db:
        async with db.execute(
            "SELECT call, COUNT(*) as count FROM logs GROUP BY call ORDER BY count DESC LIMIT ?",
            (limit,)
        ) as cursor:
            rows = await cursor.fetchall()
            return [{"call": row["call"], "count": row["count"]} for row in rows]

