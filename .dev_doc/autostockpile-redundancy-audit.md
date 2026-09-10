# AutoStockpile Go 代码冗余审计报告

- **审计日期**：2026-09-10
- **基线版本**：`0f350e4c`（工作区未提交改动不影响本报告结论）
- **审计对象**：`agent/go-service/autostockpile/`（23 个 Go 文件，3254 行）
- **审计主题**：冗余代码 —— 超前设计（dead/scaffolding/over-engineering）与无效检查（不可达分支、恒真守卫）
- **审计范围证据**：`assets/resource/pipeline/AutoStockpile/*.json`、`assets/tasks/AutoStockpile.json`、`assets/locales/go-service/*.json`、`tools/schema/custom.*.schema.json`、`assets/` 与 `install/`、`docs/`，以及 `agent/go-service/vendor/github.com/MaaXYZ/maa-framework-go/v4`
- **性质**：**只读审计，未修改任何生产代码**（本文件是唯一新增产物）

## 结论摘要

| 等级 | 数量 | 成因归类 |
| --- | --- | --- |
| High | 12 | 可证明的冗余 / 死代码 |
| Medium | 10 | 大概率冗余，取决于约定或未来扩展 |
| Low | 12 | 风格与防御式冗余，可保留但建议收敛 |

四类主要成因：

1. **旧配置入口未清理**：`price_limits` JSON 反序列化链已无任何输入可触达（H1/H2/M3），删除收益最大。
2. **不可达业务分支仍在维护**：`quantityModeSkip` 与 `switch` 默认分支（H6/H7）、真实节点类型之外的类型 switch（M2）、重复实现 SDK 已有解包（M1）。
3. **测试脚手架进了生产路径**：包内零测试，但存在环境变量价格注入与路径函数间接层（H3/H10）。
4. **平凡守卫与边界检查**：单点很小，全包合计约 40 处噪音（H4/H8/H9/H11/H12 及 Low 表）。

---

## 🟥 High —— 可证明的冗余 / 死代码

### H1. 阈值 JSON 配置入口整条链是死的（典型超前设计残留）

- **位置**：`types.go:101-124`（`PriceLimitConfig.UnmarshalJSON`）、`thresholds.go:54-80`（`parsePositiveThresholdValue`）、`thresholds.go:82-97`（`parsePriceLimitValue`）、`types.go:95` 的 `json:"price_limits"` tag
- **依据**：
    - `SelectionConfig` / `PriceLimitConfig` 的唯一构造点是 `strategy.go:68-77` 的公式计算，**全仓库没有任何 `json.Unmarshal` 目标为 `SelectionConfig`/`PriceLimitConfig`**（`params.go`/`options.go` 只解析 `Region`、`server_time`、`allow_data_upload`、`min_buy`）。
    - `price_limits` 字符串全仓库（含 `assets/`、`install/`、`docs/`、`tools/schema`、其它 Go 包）仅命中本包 4 处，其中 3 处就是这条死链自身。
    - 侧证：i18n `autostockpile.abort.ThresholdConfigInvalidFatal` 仍是「价格阈值配置无效，请检查配置，**不要输入0或留空**」，指向已被删除的用户配置入口。
- **建议**：删除 `UnmarshalJSON` + 两个 parse 函数 + tag；若确实要恢复可配置阈值，应在 `docs/zh_cn/developers/tasks/auto-stockpile-maintain.md` 的「地区与价格选项」章节同时补入口说明（该文档目前只描述公式）。

### H2. `parsePriceLimitValue` 的字符串分支永远不可达

- **位置**：`thresholds.go:88-95`
- **依据**：只有「string 反序列化失败」才会进入该函数（`thresholds.go:56-69` 已先试过 string），函数内再试一次 string 必然同样失败，属于二次兜底自己。
- **建议**：随 H1 一起删除。

### H3. 测试价格注入：分支自我覆盖 + 无任何消费者

- **位置**：`goods_scan.go:19, 350-420`（`testPricesEnvVar` / `applyTestPricesIfEnabled`），其中 `379-388` 为冗余分支
- **依据**：
    - `379-388`：初值已是 `(2, 1)`，`else if len(goods) >= 3` 又赋值 `(2, 1)` ⇒ 空操作（`len == 0/1` 已在前面 return）。
    - 环境变量 `MAAEND_AUTOSTOCKPILE_RECOGNITION_TEST_PRICES` 在**整个仓库（含 `tests/`、`tools/`、`docs/`）零引用**，且 `autostockpile` 包**没有任何 `_test.go`**。
- **附带风险**：该函数在写 `RecognitionData` 之前改写商品真实价格（`recognition.go:120`），被改写值会同时进入选品决策与 `debug/record/ElasticGoodsPrices.json` 落盘记录。
- **建议**：删除该函数与 `os` 导入；若维护者确实手工用它在真机调试，至少删掉 `379-388` 的空分支，并在函数注释里写清「仅调试用、会污染落盘数据」。

### H4. `priceCandidate.text` 只写不读

- **位置**：`goods_scan.go:27-31`（定义）、`goods_scan.go:305`（唯一赋值）
- **依据**：全文件无任何 `.text` 读取；绑价日志只用 `.value`（`goods_scan.go:455, 495`）。
- **建议**：删除字段。

### H5. `reconcile.go` 中 `priceChanged` 二次判断恒为真

- **位置**：`reconcile.go:74-91` 与 `reconcile.go:139-141`
- **依据**：`!priceChanged` 已提前 `return true`，所以第 139 行的 `if priceChanged` 在到达时必然成立。
- **建议**：去掉条件，直接 `maafocus.Print(...)`。

### H6. `quantityModeSkip` 整条链路不可达

- **位置**：`quantity.go:43-48`（`overflowTarget <= 0` ⇒ Skip）、`quantity.go:8`、`selector.go:178-197`（Skip 短路块）
- **依据**：
    1. `quota.go:96-101` + `recognition.go:59`：`Quota.Current == 0` 时识别层即以 `QuotaZeroSkip` 终止；`selector.go:74-76` 把它路由到 skip 分支并 `return`，**永不进入 `computeDecision`**。
    2. 故进入决策时恒有 `Current >= 1`；`quantity.go:23` 的 overflow 分支又要求 `Overflow >= 1` ⇒ `overflowTarget = min(Overflow, Current) >= 1`，`overflowTarget <= 0` 不可能成立。
    3. `reconcile.go` 使用同一份 `Quota`（`state.RawRecognitionData`），不变式相同。
- **连带死代码**：`selector.go:178-197` 整块（含 `i18n "autostockpile.hit_but_skip"`、focus 打印、`overrideSkipBranch` 调用），i18n 键 `autostockpile.qty_overflow_invalid` 随之失效。
- **建议**：删除 Skip 模式与 `selector.go:178-197`；或反向处理 —— 若视为真实业务分支予以保留，须在注释写明「当前不可达，为 quota 语义变化预留」。

### H7. `resolveQuantityDecision` 的 `default` 与 `case 1` 同体且不可达

- **位置**：`quantity.go:19-28`
- **依据**：`bypassThresholdFilter == false` 时 `SelectBestProduct` 只接受 `score = threshold - price > 0` ⇒ 必然 `price < threshold`（命中 case 1）；`bypass == true` ⇒ `Overflow > 0` ⇒ 先命中 case 2。「价格 ≥ 阈值 且 Overflow == 0」无法出现（唯一的 `Threshold == 0` 来自 min-buy 降级，而它的 `quantityDecision` 由 `minbuy.go:32-36` 直接给定，不经过本函数）。
- **建议**：改写为 `if data.Quota.Overflow > 0 { overflow } else { threshold }`。

### H8. `len(roi) != 4` 是不可能成立的检查

- **位置**：`overrides.go:125-127`
- **依据**：`maa.Rect = rect.Rect = [4]int`（`vendor/github.com/MaaXYZ/maa-framework-go/v4/internal/rect/rect.go:4`），`recognitionParamROI`（`overrides.go:169`）固定返回 4 元素切片。
- **建议**：删除该判断。

### H9. `filteredRecognitionResults` 的 `len(...) > 0` 判断冗余

- **位置**：`recognition_results.go:41-44`
- **依据**：所有调用方（`recognition_results.go:48, 275`、`quota.go:28, 66`）都只做 `len(x) == 0` 判断，nil 与空切片行为完全一致 ⇒ 等价于 `return detail.Results.Filtered`。
- **建议**：直接返回字段。

### H10. `resolveDailyStoragePathFunc` 是无人改写的间接层

- **位置**：`daily_storage.go:17, 64`
- **依据**：全仓库唯一引用就是同文件这两行；包内无测试（即该「测试缝合点」没有任何测试在用）。
- **建议**：删除变量，直接调用 `resolveDailyStoragePath()`。

### H11. `buildSelectionPipelineOverride` 的 ctx 参数被丢弃

- **位置**：`overrides.go:10`（`_ *maa.Context`），调用点 `selector.go:216`、`reconcile.go:208`
- **建议**：删除参数。

### H12. 三处恒真的下标/边界守卫

- **位置**：`goods_scan.go:445-447`、`goods_scan.go:485-487`、`goods_scan.go:509`
- **依据**：`used := make([]bool, len(prices))`（`goods_scan.go:71`），而 `findBestPriceCandidate` 返回的 `bestIdx` 必落在 `[0, len(prices))` ⇒ `bestIdx < len(used)`、`i < len(used)` 恒真。
- **建议**：保留一处显式断言即可，其余删除。

---

## 🟧 Medium —— 大概率冗余，但取决于约定或未来扩展

### M1. 手工解包 Custom 识别结果，重复实现 SDK 已有能力

- **位置**：`recognition_results.go:20-35`（`extractCustomRecognitionDetailJSON`）、`recognition_results.go:126-138`（`rawJSONToString`）
- **依据**：SDK 已提供 `detail.Results.Best.AsCustom().Detail`（`recognition_result.go:84`），其内部 `detailRawToString`（`recognition_result.go:153-164`）与 `rawJSONToString` **逐行等价**（含 `raw[0] == '"'` 分支）。当前写法在每次 action 里对整串 `DetailJson` 再做一次 `json.Unmarshal`。
- **建议**：改用 `AsCustom()`；若坚持手写，至少消除两处逻辑漂移的风险。

### M2. `recognitionParamROI` 的 7 路类型 switch 属超前设计

- **位置**：`overrides.go:139-170`
- **依据**：目标节点 `AutoStockpileSelectedGoodsClick` 在 `assets/resource/pipeline/AutoStockpile/DecisionLoop.json` 中固定为 `TemplateMatch`，且 override（`overrides.go:15-18`）也只写 `template`/`roi`。`FeatureMatch`/`ColorMatch`/`OCR`/`NeuralNetworkClassify`/`NeuralNetworkDetect`/`CustomRecognition` 六个 case 永不可达。
- **建议**：收缩为 `*maa.TemplateMatchParam`，其余走 `default` 报错（报错分支比「静默支持」更安全）。

### M3. `resolveTierThreshold` 的 `threshold <= 0` 检查在当前两个产出方下不可达

- **位置**：`thresholds.go:21-23`
- **依据**：JSON 路径有 `parsePositiveThresholdValue` 保证 `> 0`（且该路径本身按 H1 已死）；公式路径最小值 `0 + 600 - 250 = 350 > 0`（`strategy.go:9-38`）。真正可达的错误是 `!ok`（`price_limits` 缺该 tier）。
- **注意**：若采纳 H1 的「恢复配置入口」方案，则此检查需保留 —— 两个方案互斥。

### M4. 同一轮识别内 `validateItemMap` 被校验 3 次

- **位置**：`params.go:69`（区域解析时）、`recognition.go:80`、`goods_scan.go:53`
- **依据**：`cachedItemMap` 一经赋值永不改写或清空（`itemmap.go:57-78`），且 `itemMapHasRegion` 成功已蕴含 `IDToName` 非空。
- **建议**：入口校验一次，其余改为直接使用（或降为 `debug` 断言）。

### M5. `Validate()` 建立的不变式在 selector 里被判了两次，且前后不一致

- **位置**：`selector.go:59-62`（`if result.Data != nil` 兜底）与 `selector.go:78`（直接解引用 `result.Data`）
- **依据**：`types.go:127-144` 的 `Validate()` 已保证 `AbortReason == None ⇔ Data != nil`，且 `selector.go:51-57` 已调用它。于是第 60 行的检查冗余，而第 78 行又完全信任该不变式。
- **建议**：统一语义 —— 要么信任不变式（删 59-62 的兜底），要么两处都判。

### M6. `resolveOverflow` 的 `overflowDetected` 与 `overflowAmount > 0` 等价，只喂了一行日志

- **位置**：`quota.go:91-94`，使用点 `recognition.go:42-57, 147`
- **依据**：下游真正消费的是 `Quota.Overflow` 与 `hasOverflow()`（`types.go:146-148`）。
- **建议**：删除布尔返回值，日志用 `overflowAmount > 0`。

### M7. 状态深拷贝做两遍

- **位置**：`state.go:51-60`（`getDecisionState` 已深拷贝）⇒ `reconcile.go:93` 又 `copyRecognitionData`
- **依据**：`getDecisionState` 返回的已是私有副本，再复制一次只多一次切片分配。
- **建议**：reconcile 里直接改 `state.RawRecognitionData`。

### M8. `writeFileAtomic` 与其它包逐行重复

- **位置**：`daily_storage.go:108-143`，与 `creditshopping/storage.go:137` 起**逐行相同**（`diff` 仅差空行）；仓库内还有 `sellproduct/operator/cache.go:395`、`ims/storage.go:71` 的同类实现。
- **建议**：提到公共包（如 `agent/go-service/pkg/fsutil`）复用。

### M9. 导出面大于实际需要（无外部消费者、无测试）

- **位置**：`itemmap.go`（`LoadItemMap`/`GetItemMap`/`InitItemMap`/`MatchGoodsName`/`ParseTierFromID`/`BuildTemplatePath`）、`selector.go:269`（`SelectBestProduct`）、`minbuy.go:41`（`SelectCheapestProduct`）、`abort_reason.go:10, 19`（`ValidateAbortReason`/`LookupAbortReason`）
- **依据**：全仓库 grep（排除 vendor）：这些符号**在本包之外零引用**；`ValidateAbortReason` 只是 `isKnownAbortReason`（`types.go:150-158`）的薄封装，唯一调用者是同文件的 `LookupAbortReason`。
- **建议**：除注册点必需者外收敛为小写；`ValidateAbortReason` 内联。

### M10. `screencapShelf` 的 `img == nil` 检查在接口语义下无效

- **位置**：`shelf_swipe.go:49-51`
- **依据**：`CacheImage()` 失败返回 `(nil, err)`，成功返回 `*image.RGBA`（`vendor/.../controller.go:440-462`）；即便底层是 typed-nil，`image.Image` 接口比较也不为 nil。真正会出错的是前置的 `err`。
- **建议**：删除该判断，或与 `err` 合并处理。

---

## 🟨 Low —— 风格 / 防御式冗余，可保留但建议收敛

| # | 位置 | 结论 |
| --- | --- | --- |
| L1 | `shelf_swipe.go:16-20` | `if err != nil { return err }; return nil` ⇒ 直接 `return err` |
| L2 | `types.go:23-50` | 后缀判定过度泛化：`abortReasonSkipSuffix` + `isSkip` 实际只对应 `QuotaZeroSkip` 一个值；漏写后缀会静默变成「继续执行」，建议改为显式分类表 |
| L3 | `recognition_results.go:67-78, 119-124` | `sources [][]*RecognitionResult` + `resultsFromBest` 包装：每种 policy 只有一个来源，单层切片足够 |
| L4 | `recognition_results.go:37-65` | `filteredOCRCandidates` 与 `ocrTextCandidates` 近重复（一个返回 `[]*OCRResult`、一个返回 `[]string`），可合并为一次提取 + 一次投影 |
| L5 | `selector.go:120-126` | 块内重复调用 `result.hasOverflow()`，直接用 `bypassThresholdFilter` |
| L6 | `selector.go:410-416` | `formatSelectionMode` 第 1 与第 3 个 return 返回同一文案（第 3 个经 min-buy 降级可达，非死码，但语义易误读） |
| L7 | `decision.go:19-21` | `mapComputeDecisionErrorToAbortReason` 的 `err == nil` 分支不可达（两个调用点都在 `if err != nil` 内） |
| L8 | `server_day.go:29-31` | `loc == nil` 兜底不可达（`locationFromUTCOffset` 永不返回 nil） |
| L9 | `daily_storage.go:175-177` | `maxDateCount <= 0` 分支不可达（唯一调用点固定 120） |
| L10 | `selector.go:105` + `daily_storage.go:46-48` | 调用方直接传 `attach.AllowDataUpload` 当开关，被调方再判一次 `enabled` |
| L11 | `merge.go:10-12, 21-23` | `item.ID == ""` 过滤为防御式（两个来源均保证 ID 非空，ID 即 itemMap key） |
| L12 | `params.go:85-90` | `normalizeCustomActionParam` 的 string 分支：当前所有 pipeline 的 `custom_action_param` 都是对象 |

---

## ✅ 已核实为「合理，不建议改」（避免误伤）

- `resolveGoodsRegionFromCustomActionParam` 的 `Region` 校验链（`params.go:54-74`）：`tools/schema/custom.action.schema.json` 中 `AutoStockpile.SelectItem` **没有**任何参数规则，这是唯一防线，必须保留。
- `shelf_swipe.go:102-112` 把次屏 abort/err 降级为「仅用首屏」：刻意降级，非死代码；`restored` + `defer swipeShelfUp` 无重复滑动。
- `daily_storage.go:162-169` 的 `schema_version < 2` UID 迁移：面向历史落盘文件，保留。
- `minbuy.go:23` 的 `minBuyRegion != region`：真实分支（锁定本次运行首个访问的地区）。
- `merge.go` 对 page0 的 `seen` 去重：有效（OCR 同名不同框可产生重复 ID）。
- `nodes.go` 全部 14 个节点名常量：均在 `assets/resource/pipeline/AutoStockpile/*.json` 中被使用，无死常量。
- `go vet ./autostockpile/` 通过（需 `-mod=mod`）。

---

## 建议处理顺序

1. **第一批（纯删除、零行为变化）**：H1 + H2 + H3 + H10 + 已确认的平凡守卫（H4/H8/H9/H11/H12）。
2. **第二批（需产品确认语义）**：H6 + H7 —— 涉及 quota 语义是否还会变化，建议先出 patch 供 review。
3. **第三批（结构性收敛）**：M1/M2/M9（SDK 能力复用与导出面收敛）、M8（公共 `writeFileAtomic`）、M4/M5/M6/M7。
4. **第四批**：Low 表批量清理。

验证命令：`pnpm format:go`、`cd agent/go-service && go build ./...`，全量检查 `pnpm check` / `pnpm test`。

## 附：已知的独立问题（不属本次冗余审计）

工作区 `agent/go-service/vendor/modules.txt` 与 `go.mod` 版本不一致（`modules.txt` 标记 `maa-framework-go@v4.0.0-beta.14` 为 explicit，而 `go.mod` 已要求 `v4.0.0-beta.18`），导致直接 `go vet ./...` 报 `inconsistent vendoring`，需 `go mod vendor` 或修正 `modules.txt`。此项与 AutoStockpile 冗余无关，建议单独处理。
