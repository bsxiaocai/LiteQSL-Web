# LiteQSL-Web 短期改进计划 (v1.2 ~ v1.5)

> 📅 制定日期：2026-06-03
> 🎯 目标：解决技术债务，提升可维护性和用户体验

---

## 一、前端模块化重构

### 1.1 问题现状

| 问题 | 影响 |
|------|------|
| `admin.html` 1777 行，所有 JS 内联 | 难以维护、无法复用、IDE 支持差 |
| `index.html` 与 `admin.html` 大量重复代码 | 修改需同步两处，易遗漏 |
| 无构建流程 | 无法使用 ES Module、无法 Tree Shaking |
| Tailwind CDN 引入 | 无法自定义主题，依赖外部服务 |

### 1.2 目标架构

```
static/
├── index.html              # 公开页面（精简，引用外部 JS）
├── admin.html              # 管理页面（精简，引用外部 JS）
├── css/
│   └── app.css             # 自定义样式（可选，补充 Tailwind）
├── js/
│   ├── common/
│   │   ├── utils.js        # escapeHtml, showToast, setLoading
│   │   ├── constants.js    # BAND_FREQ, QSO_TYPE_LABELS, STATUS_OPTIONS
│   │   ├── formatters.js   # formatFreqCell, formatDate, formatTime, freqToBand
│   │   ├── pagination.js   # 通用分页渲染器
│   │   └── clock.js        # 双时钟组件
│   ├── public/
│   │   ├── app.js          # 公开页面主逻辑
│   │   ├── search.js       # 呼号搜索
│   │   └── recent.js       # 最近通联列表
│   └── admin/
│       ├── app.js          # 管理页面主逻辑
│       ├── auth.js         # 登录/登出/CSRF
│       ├── qso-form.js     # QSO 录入表单（含类型切换）
│       ├── qso-table.js    # 通联记录表格
│       ├── edit-modal.js   # 编辑弹窗
│       ├── import-export.js # ADIF/CSV 导入导出
│       └── backup.js       # 备份管理
```

### 1.3 技术方案

**方案选择：原生 ES Module（推荐）**

| 方案 | 优点 | 缺点 |
|------|------|------|
| **原生 ES Module** | 零构建、浏览器原生支持、简单 | 不支持旧浏览器（IE） |
| Vite 构建 | Tree Shaking、HMR、TypeScript | 增加构建步骤、依赖 Node.js |
| Webpack | 成熟生态 | 配置复杂、构建慢 |

> 💡 业余无线电用户群体普遍使用现代浏览器，原生 ES Module 是最佳平衡点。

**实施步骤：**

```
Phase 1: 提取公共模块
├── 创建 static/js/common/ 目录
├── 提取 utils.js, constants.js, formatters.js
├── 两个 HTML 页面改为 <script type="module"> 引入
└── 验证功能无回归

Phase 2: 拆分 admin 功能模块
├── 创建 static/js/admin/ 目录
├── 按功能拆分 auth.js, qso-form.js, qso-table.js 等
├── admin.html 只保留 HTML 结构和入口调用
└── 验证功能无回归

Phase 3: 拆分 public 功能模块
├── 创建 static/js/public/ 目录
├── 拆分 search.js, recent.js
├── index.html 只保留 HTML 结构和入口调用
└── 验证功能无回归

Phase 4: Tailwind 本地化（可选）
├── 安装 Tailwind CLI（npx tailwindcss）
├── 创建 tailwind.config.js 自定义主题
├── 构建生成 static/css/tailwind.css
└── 替换 CDN 引用
```

### 1.4 预估工作量

| 阶段 | 工时 | 风险 |
|------|------|------|
| Phase 1 | 2-3h | 低 |
| Phase 2 | 4-6h | 中（需仔细验证所有交互） |
| Phase 3 | 1-2h | 低 |
| Phase 4 | 2-3h | 低 |
| **合计** | **9-14h** | |

---

## 二、异步数据库迁移

### 2.1 问题现状

当前使用同步 `sqlite3` 模块，在 FastAPI 异步框架中会**阻塞事件循环**：

```python
# 当前代码（database.py）
def get_logs_paginated(filters, page, page_size):
    conn = get_db()
    cursor = conn.execute(...)  # ← 同步阻塞
    ...
```

当数据库查询慢或并发请求多时，会导致整个服务卡顿。

### 2.2 目标架构

```python
# 迁移后
async def get_logs_paginated(filters, page, page_size):
    async with aiosqlite.connect(DATABASE_PATH) as db:
        async with db.execute(...) as cursor:  # ← 异步非阻塞
            ...
```

### 2.3 技术方案

**依赖变更：**

```diff
# requirements.txt
fastapi==0.115.12
uvicorn==0.34.2
python-multipart==0.0.20
itsdangerous==2.2.0
bcrypt>=4.0.0
+ aiosqlite>=0.20.0
```

**代码变更清单：**

| 文件 | 变更内容 |
|------|---------|
| `app/database.py` | 所有函数改为 `async def`，使用 `aiosqlite` |
| `app/backup.py` | SQLite backup API 适配异步 |
| `app/routes/public.py` | 路由函数添加 `await` |
| `app/routes/admin.py` | 路由函数添加 `await` |

**关键实现细节：**

```python
# database.py 改造示例

# 旧代码
def get_db():
    conn = sqlite3.connect(DATABASE_PATH)
    conn.row_factory = sqlite3.Row
    return conn

# 新代码
async def get_db():
    db = await aiosqlite.connect(DATABASE_PATH)
    db.row_factory = aiosqlite.Row
    return db

# 旧代码
def get_logs_paginated(filters, page, page_size):
    conn = get_db()
    cursor = conn.execute("SELECT COUNT(*) ...")
    total = cursor.fetchone()[0]
    cursor = conn.execute("SELECT * ... LIMIT ? OFFSET ?", ...)
    logs = [dict(row) for row in cursor.fetchall()]
    conn.close()
    return {"logs": logs, "total": total, ...}

# 新代码
async def get_logs_paginated(filters, page, page_size):
    async with aiosqlite.connect(DATABASE_PATH) as db:
        db.row_factory = aiosqlite.Row
        async with db.execute("SELECT COUNT(*) ...") as cursor:
            total = (await cursor.fetchone())[0]
        async with db.execute("SELECT * ... LIMIT ? OFFSET ?", ...) as cursor:
            logs = [dict(row) for row in await cursor.fetchall()]
        return {"logs": logs, "total": total, ...}
```

### 2.4 注意事项

| 事项 | 说明 |
|------|------|
| **SQLite 写锁** | SQLite 是单写多读，异步不等于并发写入，仍需注意写操作序列化 |
| **连接池** | aiosqlite 不需要连接池，每次 `async with` 获取连接即可 |
| **备份操作** | `sqlite3.Connection.backup()` 是同步阻塞的，需用 `asyncio.to_thread()` 包装 |
| **测试** | 需要全面测试所有 CRUD 操作、ADIF 导入、备份恢复 |

### 2.5 预估工作量

| 阶段 | 工时 | 风险 |
|------|------|------|
| database.py 改造 | 3-4h | 中 |
| routes 改造 | 2-3h | 中 |
| backup.py 改造 | 1h | 低 |
| 测试验证 | 2-3h | 中 |
| **合计** | **8-11h** | |

---

## 三、统计仪表盘

### 3.1 功能需求

为管理后台添加数据统计和可视化，帮助了解通联概况。

**统计维度：**

| 维度 | 图表类型 | 说明 |
|------|---------|------|
| 通联数量趋势 | 折线图 | 按月/年统计通联数量 |
| 波段分布 | 饼图 | 各波段通联占比 |
| 模式分布 | 饼图 | SSB/CW/FT8 等占比 |
| QSO 类型分布 | 饼图 | 普通/卫星/中继/面对面占比 |
| QSL 状态 | 柱状图 | 各状态数量 |
| 通联时间分布 | 热力图/柱状图 | 按小时/星期分布 |
| Top 通联对象 | 表格 | 呼号出现次数排行 |
| DXCC 实体统计 | 表格 | 按国家/地区统计（可选） |

### 3.2 技术方案

**前端图表库选择：**

| 方案 | 大小 | 优点 | 缺点 |
|------|------|------|------|
| **Chart.js (推荐)** | ~200KB | 简单易用、CDN 引入、文档完善 | 高级定制稍弱 |
| ECharts | ~800KB | 功能强大、中文友好 | 体积大 |
| ApexCharts | ~130KB | 现代、响应式 | 生态稍小 |
| 纯 Canvas 手绘 | 0 | 无依赖 | 开发成本高 |

> 💡 推荐 Chart.js，通过 CDN 引入，与现有技术栈一致。

**后端 API 设计：**

```
GET /api/admin/stats/summary
Response: {
    "total_logs": 1234,
    "total_callsigns": 567,
    "this_month": 45,
    "this_year": 234,
    "qsl_pending": 89
}

GET /api/admin/stats/by-band
Response: [
    {"band": "20m", "count": 456},
    {"band": "40m", "count": 234},
    ...
]

GET /api/admin/stats/by-mode
Response: [
    {"mode": "SSB", "count": 567},
    {"mode": "CW", "count": 234},
    ...
]

GET /api/admin/stats/by-type
Response: [
    {"qso_type": "NORMAL", "count": 900},
    {"qso_type": "SAT", "count": 123},
    ...
]

GET /api/admin/stats/by-month?months=12
Response: [
    {"month": "2025-07", "count": 45},
    {"month": "2025-08", "count": 67},
    ...
]

GET /api/admin/stats/by-hour
Response: [
    {"hour": 0, "count": 12},
    {"hour": 1, "count": 8},
    ...
]

GET /api/admin/stats/top-calls?limit=20
Response: [
    {"call": "BV2AAA", "count": 45},
    {"call": "JA1BBB", "count": 34},
    ...
]
```

**前端页面设计：**

```
admin.html 新增「统计」标签页
├── 概览卡片（总通联数、本月通联、待确认 QSL 等）
├── 通联趋势折线图（近 12 个月）
├── 左右布局：
│   ├── 波段分布饼图
│   ├── 模式分布饼图
│   └── QSO 类型分布饼图
├── 通联时间热力图（按小时×星期）
└── Top 20 通联对象表格
```

### 3.3 数据库查询优化

当前 `logs` 表已有索引，统计查询可直接使用：

```sql
-- 按波段统计
SELECT band, COUNT(*) as count FROM logs GROUP BY band ORDER BY count DESC;

-- 按月统计
SELECT substr(qso_date, 1, 7) as month, COUNT(*) as count
FROM logs
WHERE qso_date >= date('now', '-12 months')
GROUP BY month ORDER BY month;

-- Top 通联对象
SELECT call, COUNT(*) as count FROM logs
GROUP BY call ORDER BY count DESC LIMIT 20;
```

> ⚠️ 数据量大时（>10 万条），考虑添加统计缓存或定时预计算。

### 3.4 预估工作量

| 阶段 | 工时 | 风险 |
|------|------|------|
| 后端统计 API | 3-4h | 低 |
| 前端图表集成 | 4-6h | 低 |
| 响应式布局适配 | 2h | 低 |
| **合计** | **9-12h** | |

---

## 四、高级搜索与筛选

### 4.1 问题现状

| 当前限制 | 用户影响 |
|---------|---------|
| 仅支持呼号子串搜索 | 无法精确查找特定通联 |
| 无日期范围筛选 | 无法查找某段时间的记录 |
| 无组合筛选 | 无法同时按呼号+波段+日期筛选 |
| 导出不支持筛选 | ADIF 导出只能全量，无法导出部分数据 |
| 公开页面搜索功能弱 | 访客只能按呼号搜索 |

### 4.2 后端改造

**数据库层新增函数：**

```python
# database.py 新增

async def search_logs_advanced(
    call: str = None,           # 呼号（LIKE 子串）
    band: str = None,           # 波段（精确）
    mode: str = None,           # 模式（精确）
    qso_type: str = None,       # QSO 类型（精确）
    qsl_status: str = None,     # QSL 状态（精确）
    date_from: str = None,      # 起始日期 (YYYY-MM-DD)
    date_to: str = None,        # 结束日期 (YYYY-MM-DD)
    is_sk: int = None,          # 是否 SK (0/1)
    sort_by: str = 'qso_date',  # 排序字段
    sort_order: str = 'desc',   # 排序方向
    page: int = 1,
    page_size: int = 50
) -> dict:
    """高级搜索，支持多条件组合筛选"""
    ...
```

**路由层参数扩展：**

```python
# routes/admin.py 改造

@router.get("/api/admin/logs")
async def list_logs(
    page: int = Query(1, ge=1),
    page_size: int = Query(50, ge=1, le=200),
    call: Optional[str] = None,
    band: Optional[str] = None,
    mode: Optional[str] = None,
    qsl_status: Optional[str] = None,
    qso_type: Optional[str] = None,
    date_from: Optional[str] = None,      # 新增
    date_to: Optional[str] = None,        # 新增
    is_sk: Optional[int] = None,          # 新增
    sort_by: Optional[str] = None,        # 新增
    sort_order: Optional[str] = None,     # 新增
):
    ...
```

**导出接口同步扩展：**

```python
# routes/admin.py 改造

@router.get("/api/admin/export-adif")
async def export_adif(
    band: Optional[str] = None,
    mode: Optional[str] = None,
    qsl_status: Optional[str] = None,
    qso_type: Optional[str] = None,
    date_from: Optional[str] = None,      # 新增
    date_to: Optional[str] = None,        # 新增
):
    ...

@router.get("/api/admin/export-csv")
async def export_csv(
    band: Optional[str] = None,
    mode: Optional[str] = None,
    qsl_status: Optional[str] = None,
    qso_type: Optional[str] = None,
    date_from: Optional[str] = None,      # 新增
    date_to: Optional[str] = None,        # 新增
):
    ...
```

**公开页面搜索扩展：**

```python
# routes/public.py 改造

@router.get("/api/search")
async def search_logs(
    call: Optional[str] = None,
    band: Optional[str] = None,           # 新增
    mode: Optional[str] = None,           # 新增
    date_from: Optional[str] = None,      # 新增
    date_to: Optional[str] = None,        # 新增
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
):
    ...
```

### 4.3 前端改造

**管理页面筛选区改造：**

```html
<!-- admin.html 筛选区新增字段 -->
<div class="filter-group">
    <!-- 现有字段 -->
    <input name="filterCall" placeholder="呼号搜索">
    <select name="filterBand">...</select>
    <select name="filterMode">...</select>
    <select name="filterStatus">...</select>
    <select name="filterType">...</select>

    <!-- 新增字段 -->
    <input type="date" name="filterDateFrom" placeholder="起始日期">
    <input type="date" name="filterDateTo" placeholder="结束日期">
    <select name="filterSK">
        <option value="">全部</option>
        <option value="0">正常</option>
        <option value="1">SK</option>
    </select>

    <!-- 操作按钮 -->
    <button onclick="applyFilters()">筛选</button>
    <button onclick="clearFilters()">清空</button>
    <button onclick="exportFiltered()">导出筛选结果</button>
</div>
```

**公开页面搜索区改造：**

```html
<!-- index.html 搜索区扩展 -->
<form id="searchForm">
    <input name="call" placeholder="呼号搜索">
    <select name="band">...</select>
    <select name="mode">...</select>
    <input type="date" name="dateFrom">
    <input type="date" name="dateTo">
    <button type="submit">搜索</button>
</form>
```

**URL 参数同步（可选）：**

```javascript
// 支持 URL 参数同步，方便分享搜索结果
// 例: /admin?band=20m&dateFrom=2026-01-01&dateTo=2026-06-01
function syncFiltersToURL() {
    const params = new URLSearchParams(currentFilters);
    history.replaceState(null, '', `?${params.toString()}`);
}

function loadFiltersFromURL() {
    const params = new URLSearchParams(window.location.search);
    // 填充筛选表单...
}
```

### 4.4 预估工作量

| 阶段 | 工时 | 风险 |
|------|------|------|
| 后端高级搜索 API | 2-3h | 低 |
| 导出接口扩展 | 1-2h | 低 |
| 公开搜索扩展 | 1-2h | 低 |
| 管理页面前端 | 2-3h | 低 |
| 公开页面前端 | 1-2h | 低 |
| URL 参数同步 | 1h | 低 |
| **合计** | **8-12h** | |

---

## 五、批量操作

### 5.1 功能需求

| 操作 | 说明 | 影响范围 |
|------|------|---------|
| **批量选择** | 复选框选择多条记录 | 表格每行 + 全选 |
| **批量删除** | 删除选中的多条记录 | 需确认对话框 |
| **批量修改 QSL 状态** | 将选中记录的状态统一修改 | 需选择目标状态 |
| **批量修改 SK 标记** | 标记/取消标记选中记录为 SK | 需确认对话框 |
| **批量导出** | 仅导出选中的记录 | ADIF/CSV |

### 5.2 后端 API 设计

```python
# routes/admin.py 新增

@router.post("/api/admin/logs/batch-delete")
async def batch_delete_logs(
    request: Request,
    ids: List[int] = Body(..., embed=True),  # [1, 2, 3, ...]
):
    """批量删除记录"""
    # CSRF 验证
    # 验证所有 ID 存在
    # 执行批量删除
    # 返回成功数量

@router.post("/api/admin/logs/batch-status")
async def batch_update_status(
    request: Request,
    ids: List[int] = Body(...),
    status: str = Body(...),
):
    """批量修改 QSL 状态"""
    # CSRF 验证
    # 验证 status 值合法
    # 执行批量更新
    # 返回成功数量

@router.post("/api/admin/logs/batch-sk")
async def batch_update_sk(
    request: Request,
    ids: List[int] = Body(...),
    is_sk: int = Body(...),  # 0 或 1
):
    """批量修改 SK 标记"""
    # CSRF 验证
    # 执行批量更新
    # 返回成功数量

@router.post("/api/admin/logs/batch-export")
async def batch_export(
    request: Request,
    ids: List[int] = Body(...),
    format: str = Body("adif"),  # "adif" 或 "csv"
):
    """批量导出选中记录"""
    # CSRF 验证
    # 查询指定 ID 的记录
    # 按格式导出
    # 返回文件
```

**数据库层批量操作：**

```python
# database.py 新增

async def delete_logs_batch(ids: List[int]) -> int:
    """批量删除，返回删除数量"""
    placeholders = ','.join(['?'] * len(ids))
    async with aiosqlite.connect(DATABASE_PATH) as db:
        cursor = await db.execute(
            f"DELETE FROM logs WHERE id IN ({placeholders})", ids
        )
        await db.commit()
        return cursor.rowcount

async def update_logs_status_batch(ids: List[int], status: str) -> int:
    """批量更新 QSL 状态"""
    placeholders = ','.join(['?'] * len(ids))
    async with aiosqlite.connect(DATABASE_PATH) as db:
        cursor = await db.execute(
            f"UPDATE logs SET qsl_status = ? WHERE id IN ({placeholders})",
            [status] + ids
        )
        await db.commit()
        return cursor.rowcount
```

### 5.3 前端实现

**表格新增复选框：**

```html
<!-- 表头 -->
<thead>
    <tr>
        <th><input type="checkbox" id="selectAll" onchange="toggleSelectAll()"></th>
        <th>呼号</th>
        <th>日期</th>
        ...
    </tr>
</thead>

<!-- 表体 -->
<tbody>
    <tr>
        <td><input type="checkbox" class="log-checkbox" value="123"></td>
        <td>BV2AAA</td>
        ...
    </tr>
</tbody>
```

**批量操作工具栏：**

```html
<!-- 选中记录时显示的操作栏 -->
<div id="batchToolbar" class="hidden fixed bottom-0 left-0 right-0 bg-white shadow-lg p-4">
    <div class="max-w-7xl mx-auto flex items-center justify-between">
        <span>已选择 <strong id="selectedCount">0</strong> 条记录</span>
        <div class="space-x-2">
            <select id="batchStatusSelect">
                <option value="">修改状态为...</option>
                <option value="未发送">未发送</option>
                <option value="已发送">已发送</option>
                ...
            </select>
            <button onclick="batchUpdateStatus()">应用</button>
            <button onclick="batchExportADIF()">导出 ADIF</button>
            <button onclick="batchExportCSV()">导出 CSV</button>
            <button onclick="batchDelete()" class="text-red-600">删除</button>
        </div>
    </div>
</div>
```

**JavaScript 逻辑：**

```javascript
// 全选/取消全选
function toggleSelectAll() {
    const checked = document.getElementById('selectAll').checked;
    document.querySelectorAll('.log-checkbox').forEach(cb => {
        cb.checked = checked;
    });
    updateBatchToolbar();
}

// 获取选中的 ID
function getSelectedIds() {
    return Array.from(document.querySelectorAll('.log-checkbox:checked'))
        .map(cb => parseInt(cb.value));
}

// 更新工具栏显示
function updateBatchToolbar() {
    const ids = getSelectedIds();
    const toolbar = document.getElementById('batchToolbar');
    const count = document.getElementById('selectedCount');
    if (ids.length > 0) {
        toolbar.classList.remove('hidden');
        count.textContent = ids.length;
    } else {
        toolbar.classList.add('hidden');
    }
}

// 批量删除
async function batchDelete() {
    const ids = getSelectedIds();
    if (!ids.length) return;
    if (!confirm(`确定要删除 ${ids.length} 条记录吗？此操作不可撤销。`)) return;

    const resp = await fetch('/api/admin/logs/batch-delete', {
        method: 'POST',
        headers: csrfHeaders({'Content-Type': 'application/json'}),
        body: JSON.stringify({ ids })
    });
    const data = await resp.json();
    if (data.ok) {
        showToast(`成功删除 ${data.deleted} 条记录`, 'success');
        loadLogs();
    }
}
```

### 5.4 安全考虑

| 考虑点 | 措施 |
|--------|------|
| CSRF 防护 | 所有批量操作接口需验证 CSRF Token |
| 数量限制 | 单次批量操作上限 500 条，防止滥用 |
| 确认对话框 | 删除操作需二次确认 |
| 操作日志 | 记录批量操作的执行者和影响范围（可选） |

### 5.5 预估工作量

| 阶段 | 工时 | 风险 |
|------|------|------|
| 后端批量 API | 2-3h | 低 |
| 前端复选框 + 工具栏 | 2-3h | 低 |
| 批量操作逻辑 | 2-3h | 低 |
| 测试验证 | 1-2h | 低 |
| **合计** | **7-11h** | |

---

## 六、自动日期时间

### 6.1 问题现状

当前录入 QSO 时，日期和时间需要手动填写，容易出现：
- 忘记填写日期
- 时区混淆（本地时间 vs UTC）
- 格式错误

### 6.2 功能设计

**自动填充策略：**

| 场景 | 行为 |
|------|------|
| 页面加载 | 日期默认今天，时间默认当前 UTC 时间 |
| 切换到 EYEBALL 类型 | 自动填充日期，清空时间（面对面无精确时间） |
| 用户手动修改 | 不再自动覆盖，尊重用户输入 |
| ADIF 导入 | 以导入数据为准，不自动填充 |

**UI 交互：**

```
┌─────────────────────────────────────────────────┐
│ 日期: [2026-06-03] [今天]                        │
│ 时间: [14:30] [现在 UTC]                         │
├─────────────────────────────────────────────────┤
│ 点击 [今天] → 自动填入今天的日期                   │
│ 点击 [现在 UTC] → 自动填入当前 UTC 时间            │
│ 手动修改后 → 按钮仍可用，点击会覆盖                 │
└─────────────────────────────────────────────────┘
```

### 6.3 实现方案

**前端 JavaScript：**

```javascript
// 日期时间自动填充工具

function getTodayDate() {
    // 返回 YYYY-MM-DD 格式的今天日期（本地时间）
    const now = new Date();
    return now.getFullYear() + '-' +
        String(now.getMonth() + 1).padStart(2, '0') + '-' +
        String(now.getDate()).padStart(2, '0');
}

function getNowUTC() {
    // 返回 HH:MM 格式的当前 UTC 时间
    const now = new Date();
    return String(now.getUTCHours()).padStart(2, '0') + ':' +
        String(now.getUTCMinutes()).padStart(2, '0');
}

function setTodayDate(inputId) {
    document.getElementById(inputId).value = getTodayDate();
}

function setNowUTC(inputId) {
    document.getElementById(inputId).value = getNowUTC();
}

// 页面加载时自动填充
document.addEventListener('DOMContentLoaded', () => {
    setTodayDate('addDate');
    setNowUTC('addTime');
});
```

**表单 HTML 改造：**

```html
<!-- 日期输入 -->
<div class="flex items-end gap-2">
    <div class="flex-1">
        <label class="block text-sm font-medium text-gray-700">日期</label>
        <input type="date" id="addDate" name="qso_date"
               class="mt-1 block w-full border-gray-300 rounded-md shadow-sm">
    </div>
    <button type="button" onclick="setTodayDate('addDate')"
            class="px-3 py-2 text-sm bg-gray-100 hover:bg-gray-200 rounded-md">
        今天
    </button>
</div>

<!-- 时间输入 -->
<div class="flex items-end gap-2">
    <div class="flex-1">
        <label class="block text-sm font-medium text-gray-700">时间 (UTC)</label>
        <input type="time" id="addTime" name="time_on"
               class="mt-1 block w-full border-gray-300 rounded-md shadow-sm">
    </div>
    <button type="button" onclick="setNowUTC('addTime')"
            class="px-3 py-2 text-sm bg-gray-100 hover:bg-gray-200 rounded-md">
        现在 UTC
    </button>
</div>
```

**EYEBALL 类型特殊处理：**

```javascript
// setupAddFormTypeToggle() 中增加
function setupAddFormTypeToggle() {
    // ... 现有代码 ...
    typeSelect.addEventListener('change', () => {
        if (typeSelect.value === 'EYEBALL') {
            // EYEBALL 自动填充日期，清空时间
            setTodayDate('addDate');
            document.getElementById('addTime').value = '';
        }
    });
}
```

### 6.4 编辑弹窗同步

编辑弹窗中的日期时间也需增加「今天」和「现在 UTC」按钮，逻辑相同。

### 6.5 预估工作量

| 阶段 | 工时 | 风险 |
|------|------|------|
| 工具函数实现 | 0.5h | 低 |
| 添加表单改造 | 1h | 低 |
| 编辑弹窗改造 | 1h | 低 |
| EYEBALL 特殊处理 | 0.5h | 低 |
| 测试验证 | 0.5h | 低 |
| **合计** | **3.5h** | |

---

## 七、总体排期建议

### 7.1 优先级排序

| 优先级 | 功能 | 理由 |
|--------|------|------|
| ⭐⭐⭐ P0 | 前端模块化重构 | 技术债务最高，影响后续所有开发 |
| ⭐⭐⭐ P0 | 高级搜索与筛选 | 用户体验提升最明显 |
| ⭐⭐ P1 | 自动日期时间 | 开发量小，体验改善明显 |
| ⭐⭐ P1 | 批量操作 | 提升管理效率 |
| ⭐ P2 | 异步数据库 | 性能优化，当前单用户场景影响不大 |
| ⭐ P2 | 统计仪表盘 | 锦上添花，非核心功能 |

### 7.2 版本规划

```
v1.2.0 (第一阶段 - 已完成) ✅
├── 前端模块化重构 (Phase 1-3) ✅
├── 高级搜索与筛选 ✅
├── 自动日期时间 ✅
├── 呼号配置功能 ✅
├── Tailwind CSS 本地化 ✅
└── 密码重置脚本 ✅

v1.3 (第二阶段 - 已完成) ✅
├── 批量操作 ✅
├── 异步数据库迁移 ✅
└── 统计仪表盘 ✅

v1.4 (第三阶段 - 已完成) ✅
├── 公开页面搜索增强 ✅
├── URL 参数同步 ✅
└── 其他小优化
```

### 7.3 风险提示

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 前端重构引入 Bug | 高 | 每个 Phase 完成后全面测试，保留旧代码备份 |
| 异步迁移数据库连接问题 | 中 | 先在测试环境验证，做好回滚方案 |
| 浏览器兼容性 | 低 | 目标用户群体使用现代浏览器，ES Module 支持率高 |
| 开发周期超预期 | 中 | 按优先级裁剪功能，P0 必保，P2 可延后 |

---

## 八、验收标准

### 8.1 前端模块化重构 ✅

- [x] `index.html` 行数 < 200 行（仅 HTML 结构）→ **169 行**
- [x] `admin.html` 行数 < 800 行（仅 HTML 结构）→ **776 行**
- [x] 公共函数无重复代码 → **提取到 js/common/ 模块**
- [x] 所有功能正常（登录、录入、查询、导入导出、备份）
- [x] 浏览器控制台无报错

### 8.2 异步数据库 ✅

- [x] 所有 API 响应正常
- [x] 并发请求不阻塞
- [x] ADIF 导入大文件（>1MB）不卡顿
- [x] 备份恢复功能正常

### 8.3 统计仪表盘 ✅

- [x] 各图表数据准确
- [x] 图表响应式适配（手机/平板/桌面）
- [x] 数据量大时加载时间 < 3s

### 8.4 高级搜索 ✅

- [x] 支持呼号+波段+模式+日期范围+SK状态组合筛选
- [x] 导出支持筛选条件（ADIF/CSV）
- [x] 公开页面搜索功能增强
- [x] URL 参数同步正常

### 8.5 批量操作 ✅

- [x] 全选/取消全选正常
- [x] 批量删除需二次确认
- [x] 批量修改状态正常
- [x] 单次批量操作上限 500 条

### 8.6 自动日期时间 ✅

- [x] 页面加载自动填充今天的日期和当前 UTC 时间
- [x] 「今天」和「现在 UTC」按钮功能正常
- [x] EYEBALL 类型自动填充日期、清空时间
- [x] 手动修改后不自动覆盖

---

## 九、已完成的额外改进

> 📅 更新日期：2026-06-05

### 9.1 呼号配置功能 ✅

**需求：** 支持在管理界面设置操作员呼号，自动替换访客界面显示的呼号。

**实现内容：**

| 文件 | 变更内容 |
|------|---------|
| `app/database.py` | 新增 `settings` 表、`seed_default_settings()`、`get_setting()`、`get_all_settings()`、`update_setting()` |
| `app/routes/public.py` | 新增 `GET /api/station-info` 公开接口，返回电台呼号和站点名称 |
| `app/routes/admin.py` | 新增 `GET /api/admin/settings` 和 `PUT /api/admin/settings` 管理接口 |
| `static/index.html` | 页面加载时调用 `/api/station-info` 动态更新标题和搜索框 placeholder |
| `static/admin.html` | 新增「系统设置」区域，支持修改呼号和站点名称 |

**功能特性：**
- 访客界面标题自动显示为 `{呼号} {站点名称}` 格式
- 搜索框 placeholder 动态显示当前呼号
- 管理界面可修改呼号和站点名称
- 呼号自动转大写存储
- 默认值为 `BH7GUL` 和 `QSL & Log Management`

### 9.2 Tailwind CSS 本地化 ✅

**需求：** 将 Tailwind CSS CDN 引用改为本地文件，避免依赖外部服务。

**实现内容：**

| 文件 | 变更内容 |
|------|---------|
| `static/css/tailwind.js` | 从 CDN 下载 Tailwind CSS 并保存为本地文件 |
| `static/index.html` | 将 `https://cdn.tailwindcss.com` 改为 `/static/css/tailwind.js` |
| `static/admin.html` | 将 `https://cdn.tailwindcss.com` 改为 `/static/css/tailwind.js` |

**优势：**
- 无需依赖外部 CDN 服务
- 页面加载速度更快（本地文件）
- 离线环境也能正常使用
- 避免 CDN 可用性问题

### 9.3 前端模块化重构 ✅

**需求：** 解决前端代码高度耦合、重复代码多、难以维护的问题。

**实现内容：**

| 目录 | 文件 | 说明 |
|------|------|------|
| `static/js/common/` | `utils.js` | 通用工具函数（escapeHtml, showToast, setLoading, statusColor） |
| | `constants.js` | 常量定义（QSO类型、波段、模式等） |
| | `formatters.js` | 格式化函数（日期、时间、频率、类型标签） |
| | `pagination.js` | 通用分页渲染器 |
| | `clock.js` | 双时钟组件（北京时间+UTC） |
| | `index.js` | 统一导出 |
| `static/js/public/` | `app.js` | 公开页面主逻辑 |
| `static/js/admin/` | `app.js` | 管理后台主入口 |
| | `auth.js` | 登录/登出/CSRF 管理 |
| | `qso-form.js` | QSO 录入表单 |
| | `qso-table.js` | 通联记录表格 |
| | `edit-modal.js` | 编辑弹窗 |
| | `import-export.js` | ADIF/CSV 导入导出 |
| | `backup.js` | 备份管理 |
| | `settings.js` | 系统设置 |

**改进效果：**
- `admin.html` 从 1777 行减少到 776 行（-56%）
- `index.html` 从 478 行减少到 169 行（-65%）
- 消除两个 HTML 文件之间的重复代码
- 使用 ES Module 组织代码，便于维护和扩展

### 9.4 高级搜索与筛选 ✅

**需求：** 支持多条件组合筛选，提升记录管理效率。

**实现内容：**

| 文件 | 变更内容 |
|------|---------|
| `app/database.py` | `get_logs_paginated()` 支持日期范围、SK状态筛选、排序 |
| `app/routes/admin.py` | 列表和导出接口添加 `date_from`、`date_to`、`is_sk`、`sort_by`、`sort_order` 参数 |
| `static/admin.html` | 筛选区新增起始日期、结束日期、SK状态控件 |
| `static/js/admin/qso-table.js` | `loadLogs()` 支持新筛选参数 |

**筛选维度：**
- 呼号（模糊搜索）
- 波段（精确匹配）
- 模式（精确匹配）
- 卡片状态（精确匹配）
- QSO 类型（精确匹配）
- 日期范围（起始日期 ~ 结束日期）
- SK 状态（正常/SK）

### 9.5 自动日期时间 ✅

**需求：** 录入 QSO 时自动填充日期和时间，减少手动输入错误。

**实现内容：**

| 功能 | 说明 |
|------|------|
| 页面加载自动填充 | 日期默认今天，时间默认当前 UTC 时间 |
| 「今天」按钮 | 一键填充当前日期 |
| 「现在 UTC」按钮 | 一键填充当前 UTC 时间 |
| EYEBALL 特殊处理 | 切换到 EYEBALL 类型时自动填充日期、清空时间 |

### 9.6 密码重置脚本 ✅

**需求：** 支持在服务器端重置管理员密码，解决忘记密码的问题。

**实现内容：**

新增 `reset_password.py` 脚本，支持：
- 重置指定用户的密码
- 重置首次登录状态
- 列出所有用户

**使用方法：**
```bash
# 重置 admin 密码为默认值
python reset_password.py

# 列出所有用户
python reset_password.py --list

# 重置指定用户密码
python reset_password.py 用户名 新密码
```

### 9.7 Bug 修复 ✅

**v1.2.0 修复的问题：**

| 问题 | 修复 |
|------|------|
| 记录管理表格错位 | 移除 ID 列，合并 RST 为「收发RST」列 |
| 编辑弹窗无法打开 | 修复 `editIsSk` → `editIsSkCheck` ID 不匹配 |
| 分页控件不显示 | 修复分页容器 ID 不匹配 |
| 401 错误无处理 | `loadLogs()` 增加 401 状态码处理，自动跳转登录页 |

### 9.8 批量操作功能 ✅

**需求：** 支持批量选择、删除、修改状态和导出记录。

**实现内容：**

| 文件 | 变更内容 |
|------|---------|
| `app/database.py` | 新增 `delete_logs_batch()`、`update_logs_status_batch()`、`update_logs_sk_batch()`、`get_logs_by_ids()` |
| `app/routes/admin.py` | 新增批量删除、批量修改状态、批量修改 SK、批量导出 API |
| `static/admin.html` | 表格新增复选框列、批量操作工具栏 |
| `static/js/admin/qso-table.js` | 新增全选、批量操作函数 |

**功能特性：**
- 全选/取消全选功能
- 批量删除（需二次确认）
- 批量修改 QSL 状态
- 批量标记/取消 SK
- 批量导出 ADIF/CSV
- 单次批量操作上限 500 条

### 9.9 异步数据库迁移 ✅

**需求：** 将同步 SQLite 操作改为异步，避免阻塞 FastAPI 事件循环。

**实现内容：**

| 文件 | 变更内容 |
|------|---------|
| `requirements.txt` | 新增 `aiosqlite>=0.20.0` 依赖 |
| `app/database.py` | 所有数据库操作函数改为 `async def`，使用 `aiosqlite` |
| `app/routes/admin.py` | 所有路由函数添加 `await` 调用 |
| `app/routes/public.py` | 所有路由函数添加 `await` 调用 |

**技术方案：**
- 使用 `aiosqlite` 替代 `sqlite3`
- 每个数据库操作使用 `async with` 获取连接
- 使用 `await cursor.fetchone()` 和 `await cursor.fetchall()`
- 保留同步的 `init_db()` 用于启动时初始化

### 9.10 统计仪表盘 ✅

**需求：** 为管理后台添加数据统计和可视化功能。

**实现内容：**

| 文件 | 变更内容 |
|------|---------|
| `app/routes/admin.py` | 新增统计 API：`/stats/summary`、`/stats/by-band`、`/stats/by-mode`、`/stats/by-type`、`/stats/by-month`、`/stats/by-hour`、`/stats/top-calls` |
| `static/admin.html` | 新增统计标签页、概览卡片、图表容器 |
| `static/js/admin/stats.js` | 新增统计模块，使用 Chart.js 渲染图表 |
| `static/js/admin/app.js` | 导入并初始化统计模块 |

**功能特性：**
- 概览卡片：总通联数、唯一呼号、本月通联、本年通联、待确认 QSL
- 通联趋势折线图（近 12 个月）
- 波段分布饼图
- 模式分布饼图
- QSO 类型分布饼图
- 通联时间分布柱状图（按小时 UTC）
- Top 20 通联对象表格
- 使用 Chart.js 图表库

### 9.11 公开页面搜索增强 ✅

**需求：** 公开页面搜索支持更多筛选条件。

**实现内容：**

| 文件 | 变更内容 |
|------|---------|
| `app/routes/public.py` | `/api/search` 接口扩展支持波段、模式、日期范围筛选；新增 `/api/bands` 和 `/api/modes` 接口 |
| `static/index.html` | 搜索表单新增波段、模式、起始日期、结束日期筛选控件 |
| `static/js/public/app.js` | `loadSearch()` 函数支持多条件筛选 |

**筛选维度：**
- 呼号（模糊搜索）
- 波段（精确匹配）
- 模式（精确匹配）
- 日期范围（起始日期 ~ 结束日期）

### 9.12 URL 参数同步 ✅

**需求：** 管理页面筛选条件同步到 URL，支持分享和书签。

**实现内容：**

| 文件 | 变更内容 |
|------|---------|
| `static/js/admin/qso-table.js` | 新增 `syncFiltersToURL()`、`loadFiltersFromURL()`、`getFilterValues()`、`setFilterValues()` 函数 |

**功能特性：**
- 筛选条件自动同步到 URL 参数
- 页面加载时从 URL 读取筛选条件
- 分页页码同步到 URL
- 支持分享筛选结果链接

---

> 📝 本文档由 Claude 生成，供项目维护者审阅修订。请根据实际情况调整优先级和排期。
