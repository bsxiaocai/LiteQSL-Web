# LiteQSL-Web 更新日志

## v1.1.1 (2026-05-30)

### Bug 修复

#### 1. 统一公共查询页与管理后台的频率显示逻辑
- 两个界面现在使用相同的 `formatFreqCell` 函数
- 卫星通联：显示卫星名称 + 上下行频率（如 `🛰 SO-50` 和 `145.850 ↑ / 436.795 ↓`）
- 中继通联：显示上下行频率（如 `145.850 ↑ / 436.795 ↓`）
- 普通通联：显示频率和波段（如 `14.270 MHz 20m`）
- Eyeball 通联：显示备注（如有）

### 修改文件清单

| 文件 | 修改原因 |
|------|----------|
| `static/admin.html` | 统一频率显示逻辑，使用 `formatFreqCell` 替代 `formatFreqBandCell` |
| `static/index.html` | 统一频率显示逻辑，优化中继通联显示 |

---

## v1.1.0 (2026-05-29)

### 新增功能

#### 1. 首次登录弹窗优化
- 实现真正的 Modal 行为
- 弹窗打开期间禁止页面滚动
- 禁止操作背景页面
- 点击背景区域无效
- 使用遮罩层拦截所有背景操作
- 用户必须完成修改后才能继续使用后台

#### 2. QSO 类型驱动表单重构
根据 QSO 类型动态显示对应的字段，不同类型的通联显示不同的表单：

**标准 QSO（NORMAL）**
- 显示：呼号、日期、时间、频率、波段、模式、RST、备注

**中继通联（REP）**
- 显示：呼号、日期、时间、上行频率、下行频率、模式、中继名称（可选）、备注
- 隐藏：普通频率字段

**卫星通联（SAT）**
- 显示：呼号、日期、时间、上行频率、下行频率、卫星名称、模式、备注
- 隐藏：普通频率字段

**Eyeball QSO（线下见面）**
- 显示：呼号、日期、QTH（可选）、备注
- 隐藏：时间、频率、波段、模式、RST

**统一命名规范**
- 卫星通联和中继通联统一使用"上行频率"和"下行频率"
- 不再使用"发射频率"、"接收频率"、"Uplink"、"Downlink"等不统一名称

#### 3. 日期时间输入优化
- 日期：使用 `<input type="date">`，前端显示 YYYY-MM-DD 格式
- 时间：使用 `<input type="time">`，前端显示 HH:MM:SS 格式
- 提交时自动转换为数据库格式（YYYYMMDD / HHMMSS）
- 保持向后兼容，不影响现有数据

#### 4. ADIF 导出逻辑调整
- Eyeball QSO 在 ADIF 导出时被自动忽略
- 因为 Eyeball QSO 本质上是线下见面，没有真实的频率和模式数据
- CSV 导出时仍包含所有类型的 QSO（包括 Eyeball）

### 修改文件清单

| 文件 | 修改原因 |
|------|----------|
| `static/admin.html` | 首次登录弹窗优化、QSO 类型驱动表单重构、日期时间输入优化 |
| `app/adif_parser.py` | ADIF 导出时忽略 Eyeball QSO |
| `doc/changelog.md` | 新增更新日志 |

### 技术细节

#### 首次登录弹窗优化
- 添加 CSS 样式：`body.modal-open` 类禁止页面滚动
- 添加遮罩层：`#firstLoginOverlay` 拦截所有背景操作
- 修改 `openFirstLoginModal()` 和 `closeFirstLoginModal()` 函数

#### QSO 类型驱动表单重构
- 录入表单：`#addForm` 根据类型显示/隐藏字段组
  - `#addNormalFields`：普通通联字段
  - `#addSatFields`：卫星通联字段
  - `#addRepFields`：中继通联字段
  - `#addTimeGroup`：时间字段（Eyeball 隐藏）
- 编辑弹窗：`#editModal` 同步修改
  - `#editNormalFields`：普通通联字段
  - `#editSatFields`：卫星通联字段
  - `#editRepFields`：中继通联字段
  - `#editTimeGroup`：时间字段（Eyeball 隐藏）
- 新增 `setupAddFormTypeToggle()` 和 `setupEditFormTypeToggle()` 函数

#### 日期时间输入优化
- 录入表单：`<input type="date">` 和 `<input type="time" step="1">`
- 编辑弹窗：同步修改
- 新增 `dateForInput()` 和 `timeForInput()` 辅助函数
- 提交时自动转换：`replace(/-/g, '')` 和 `replace(/:/g, '')`
- 删除旧的日期时间验证器（不再需要）

#### ADIF 导出逻辑调整
- 在 `export_adif()` 函数中添加 Eyeball QSO 过滤逻辑
- 跳过 `qso_type == "EYEBALL"` 的记录

### 兼容性说明

1. **数据库结构**：未修改
2. **现有 API**：未修改
3. **ADIF 导入导出**：保持兼容，Eyeball QSO 在 ADIF 导出时被忽略
4. **CSV 导出**：保持兼容，包含所有类型的 QSO
5. **现有数据**：可正常读取和显示

### 测试建议

1. **首次登录流程**
   - 使用默认账号登录
   - 验证弹窗无法关闭
   - 验证背景不可操作
   - 完成修改后验证页面刷新

2. **QSO 新建**
   - 测试普通 QSO 新建
   - 测试中继 QSO 新建
   - 测试卫星 QSO 新建
   - 测试 Eyeball QSO 新建

3. **ADIF 导入导出**
   - 导入包含 Eyeball QSO 的 ADIF 文件
   - 导出 ADIF 验证 Eyeball QSO 被忽略
   - 导出 CSV 验证 Eyeball QSO 被包含

4. **卡片管理**
   - 验证所有类型的 QSO 都能正常显示
   - 验证编辑功能正常
   - 验证删除功能正常
