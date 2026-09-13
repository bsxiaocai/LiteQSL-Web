# LiteQSL-Web v2 接口兼容性对照表

> 目的：记录前端（`static/`）实际调用的每一个后端端点，以及 Go 版与 Python 原版的逐字段对比结果。
> 对比方式：同时运行 Python 原版（端口 8020）与 Go 版（端口 8021），使用**全新数据库**写入相同数据后，
> 对每个端点做规范化 JSON 比对（递归排序键、剔除易变字段 `created_at`）。
> 结论：**全部 26 项检查通过**。

---

## 1. 对比结果汇总

| 类别 | 检查数 | 结果 |
|------|--------|------|
| JSON 端点逐字段对比 | 24 | ✅ 24 项完全一致 |
| CSV 导出 | 1 | ✅ 字节级一致（MD5 相同） |
| ADIF 导出 | 1 | ✅ 归一化后一致（仅版本号不同，见下） |
| **合计** | **26** | **✅ 全部通过** |

对比覆盖的写入数据：NORMAL（北京时区录入）、SAT（上下行频率）、EYEBALL（无频率）各 1 条。

---

## 2. 逐字段对比明细

### 2.1 公开接口（访客页面 `js/public/app.js`）

| 前端调用 | 方法/路径 | 参数 | 对比结果 |
|----------|-----------|------|----------|
| 加载电台信息 | `GET /api/station-info` | — | ✅ 一致 |
| 最近通联 | `GET /api/recent` | `band/mode/qso_type/page/page_size` | ✅ 一致（含 `?band=20m`） |
| 组合查询 | `GET /api/search` | `call/band/mode/date_from/date_to/page/page_size` | ✅ 一致（含 `?call=BH7`、`?band=20m&mode=FT8`） |
| 波段列表 | `GET /api/bands` | — | ✅ 一致 |
| 模式列表 | `GET /api/modes` | — | ✅ 一致 |

### 2.2 认证与会话（`js/admin/auth.js`、`js/admin/app.js`）

| 前端调用 | 方法/路径 | 说明 | 对比结果 |
|----------|-----------|------|----------|
| 登录 | `POST /api/admin/login` | `{username,password}` → `{ok,first_login}` | ✅ 一致 |
| 登录状态 | `GET /api/admin/check` | → `{logged_in,first_login}` | ✅ 一致 |
| CSRF Token | `GET /api/admin/csrf-token` | → `{csrf_token}` | ✅ 一致 |
| 登出 | `POST /api/admin/logout` | → `{ok}` | ✅ 集成测试覆盖 |
| 改密 | `POST /api/admin/change-password` | `{old_password,new_password}` | ✅ 集成测试覆盖 |
| 首次登录状态 | `GET /api/admin/first-login-status` | → `{first_login}` | ✅ 集成测试覆盖 |
| 完成首次登录 | `POST /api/admin/complete-first-login` | 改用户名+密码 | ✅ 集成测试覆盖 |

### 2.3 QSO 记录管理（`js/admin/qso-table.js`、`qso-form.js`、`edit-modal.js`）

| 前端调用 | 方法/路径 | 参数/响应 | 对比结果 |
|----------|-----------|-----------|----------|
| 列表（分页/筛选） | `GET /api/admin/logs` | `page/page_size/call/band/mode/qsl_status/qso_type/date_from/date_to/is_sk/sort_by/sort_order` | ✅ 一致（含 qso_type/qsl_status/is_sk/排序筛选） |
| 新增 | `POST /api/admin/logs` | QSOBody → `{ok,id}`；重复 409；`force` 跳过 | ✅ 一致 |
| 编辑 | `PUT /api/admin/logs/{id}` | QSOBody → `{ok}` | ✅ 集成测试覆盖 |
| 改状态 | `PUT /api/admin/logs/{id}/status` | `{qsl_status}` → `{ok}` | ✅ 集成测试覆盖 |
| 删除 | `DELETE /api/admin/logs/{id}` | → `{ok}`；不存在 404 | ✅ 集成测试覆盖 |
| 批量删除 | `POST /api/admin/logs/batch-delete` | `{ids}` → `{ok,deleted}` | ✅ 集成测试覆盖 |
| 批量改状态 | `POST /api/admin/logs/batch-status` | `{ids,status}` → `{ok,updated}` | ✅ 集成测试覆盖 |
| 批量 SK | `POST /api/admin/logs/batch-sk` | `{ids,is_sk}` → `{ok,updated}` | ✅ 集成测试覆盖 |
| 批量导出 | `POST /api/admin/logs/batch-export` | `{ids,format}` → 文件 | ✅ 集成测试覆盖 |

### 2.4 导入导出（`js/admin/import-export.js`）

| 前端调用 | 方法/路径 | 说明 | 对比结果 |
|----------|-----------|------|----------|
| ADIF 导入 | `POST /api/admin/import-adif` | multipart；重复时 `{ok:false,duplicates,duplicate_count,total}` | ✅ 集成测试覆盖（含 413/重复/强制） |
| ADIF 导出 | `GET /api/admin/export-adif` | 筛选导出 | ✅ 归一化一致 |
| CSV 导出 | `GET /api/admin/export-csv` | UTF-8 BOM + CRLF | ✅ **字节级一致（MD5 相同）** |

### 2.5 备份管理（`js/admin/backup.js`）

| 前端调用 | 方法/路径 | 说明 | 对比结果 |
|----------|-----------|------|----------|
| 备份列表 | `GET /api/admin/backups` | → `{backups:[{filename,size,created_at}]}` | ✅ 一致（**修复后**，见 §4） |
| 创建备份 | `POST /api/admin/backup` | → `{ok,backup}` | ✅ 集成测试覆盖 |
| 下载备份 | `GET /api/admin/backups/{filename}` | 文件下载 | ✅ 集成测试覆盖 |
| 删除备份 | `DELETE /api/admin/backups/{filename}` | → `{ok}` | ✅ 集成测试覆盖 |
| 恢复 | `POST /api/admin/restore` | `{filename}` → `{ok,safety_backup}` | ✅ 集成测试覆盖 |

### 2.6 系统设置与统计（`js/admin/settings.js`、`stats.js`）

| 前端调用 | 方法/路径 | 说明 | 对比结果 |
|----------|-----------|------|----------|
| 读取设置 | `GET /api/admin/settings` | → `{settings:{...}}` | ✅ 一致 |
| 更新设置 | `PUT /api/admin/settings` | `{callsign,station_name,visitor_timezone}` → `{ok,updated}` | ✅ 集成测试覆盖 |
| QSL 状态枚举 | `GET /api/admin/qsl-statuses` | → `{statuses:[6]}` | ✅ 一致 |
| QSO 类型枚举 | `GET /api/admin/qso-types` | → `{types:[4]}` | ✅ 一致 |
| 概览 | `GET /api/admin/stats/summary` | 5 个计数 | ✅ 一致 |
| 波段分布 | `GET /api/admin/stats/by-band` | `[{band,count}]` | ✅ 一致 |
| 模式分布 | `GET /api/admin/stats/by-mode` | `[{mode,count}]` | ✅ 一致 |
| 类型分布 | `GET /api/admin/stats/by-type` | `[{qso_type,count}]` | ✅ 一致 |
| 月度趋势 | `GET /api/admin/stats/by-month?months=12` | `[{month,count}]` | ✅ 一致 |
| 小时分布 | `GET /api/admin/stats/by-hour` | `[{hour,count}]` | ✅ 一致 |
| Top 呼号 | `GET /api/admin/stats/top-calls?limit=20` | `[{call,count}]` | ✅ 一致 |

---

## 3. 静态资源与页面

| 资源 | 结果 |
|------|------|
| `GET /`（访客页） | ✅ 200 `text/html; charset=utf-8` |
| `GET /admin`（管理后台） | ✅ 200 `text/html; charset=utf-8` |
| `GET /static/css/tailwind.js` | ✅ 200 `text/javascript` |
| `GET /static/js/public/app.js` | ✅ 200 `text/javascript`（ES Module 正确 MIME） |
| `GET /static/js/admin/*.js`（9 个模块） | ✅ 全部 200 `text/javascript` |
| `GET /static/js/common/*.js`（6 个模块） | ✅ 全部 200 `text/javascript` |

前端**零改动**即可对接 Go 后端。

---

## 4. 本阶段修复的兼容性缺陷

| # | 问题 | 影响 | 修复 |
|---|------|------|------|
| 1 | `/api/admin/backups` 空列表返回 `null` | 前端 `data.backups.length` 会抛 TypeError | `backup.List` 返回非 nil 空切片 → `[]` |
| 2 | 备份文件名 1 秒精度冲突 | 同秒创建备份时 `VACUUM INTO` 失败；恢复时的安全备份会覆盖待恢复的源备份（Python 同样存在） | 冲突时追加序号后缀 `_1`、`_2` |
| 3 | 空结果 `null` 而非 `[]`（阶段三已修） | 前端遍历报错 | bands/modes/stats/scanRows 初始化空切片 |
| 4 | `created_at` 被序列化为 `2026-09-10T07:38:14Z`（阶段三已修） | 与 Python 的空格格式不一致 | `CAST(created_at AS TEXT)` |

---

## 5. 已知的、有意保留的差异

| 项 | Python | Go | 说明 |
|----|--------|-----|------|
| `/health` 版本号 | `1.3.0` | `2.0.0` | 重构为 v2，有意提升 |
| ADIF `PROGRAMVERSION` | `1.3.0` | `2.0.0` | 同上（`ADIF_VER` 均为 `3.1.5`） |
| Session 格式 | itsdangerous 签名 | HMAC-SHA256 自实现 | 切换后需**重新登录一次**；两者均 SameSite=Lax + HttpOnly + 7 天 |
| JSON 对象键序 | 插入序 | map 字典序 | JSON 对象键序无语义，前端按键访问 |
| 非法日期录入 | HTTP 500 | HTTP 400 | 前端已去横杠/冒号，正常不触发 |

### 5.1 前端侧待改进项（非后端兼容问题，未擅自修改）

| 项 | 说明 | 建议 |
|----|------|------|
| Chart.js 依赖 CDN | `admin.html` 从 `cdn.jsdelivr.net` 加载 Chart.js 4.4.0；离线/内网部署时统计页图表无法渲染 | 参考 Tailwind 本地化，下载为 `static/js/vendor/chart.umd.min.js` 并改一行 `src` |
| 批量状态下拉缺少「已收到」 | `admin.html` 的批量状态下拉只有 5 项（无「已收到」），后端支持 | 前端补一个 `<option>` |
| `/api/admin/qsl-statuses`、`/qso-types` 无鉴权 | 与 Python 一致，位于 `/api/admin` 前缀但未校验登录 | 如需收紧可加 `requireAdmin`（会改变旧行为，需确认） |
