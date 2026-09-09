# AutoCollect 路线生成器

`tools/pipeline-generate/data/collect_points.json` 是 zmdmap 数据 CI 生成并由 `fetch:zmdmap` 下载的篝火/采集物坐标目录；本目录的 `routes.json` 参考 AutoDelivery 的 metadata-only 维护方式，自动同步全部篝火和采集物的检索元数据，并只保存人工分簇覆盖。`tools/schema/auto_collect_routes.schema.json` 为其提供 IDE 校验。两者共同生成：

- `assets/resource/pipeline/AutoCollect/Routes/{RouteFileId}.json`：按篝火分组保存可直接试跑的 `AutoCollectRoute...` 节点，文件名取自该篝火的传送入口节点名（如 `WulingSnowyForest1.json`）。

运行：

```powershell
pnpm generate:AutoCollect
```

每条路线是三个节点：`...Start` 用 `SubTask` 调用该篝火的 `SceneEnterWorld...` 传送入口，`...Goto` 用 `MapNavigateAction` 先 `ZONE` 切到大世界底图、再 `NAVMESH` 走到第一颗采集物，随后逐点执行 `COLLECT`，`...End` 收尾。`Start` 节点 `enabled: false`，是节点测试工具里的单路线入口，没有接进 `AutoCollectRoutes` 选项。

分簇规则：一条路线只覆盖同一个篝火 90 像素内、同一 `detail_id` 且不少于 6 个的采集物；采集顺序从离篝火最近的一颗起步，每次走到最近的下一颗，再反转能缩短总长的子段。目前只生成 `map02` 的路线，覆盖新地图时在 `collect_points_data.py` 的 `COLLECT_MAPS` 里加上对应 map ID。

运行生成命令时，`sync-routes.mjs` 会按 `source_id` 刷新篝火的 `name` / `enter_node` 和采集物的 `count`，为数据中的新增项补充 metadata-only 条目，并保留已有的人工字段。篝火支持 `skip` / `radius` / `min_points`，采集物支持 `skip` / `action`；两者都支持仅供阅读的 `description`。需要单独调整某个篝火的覆盖范围时改 `radius` 或 `min_points`，需要挖掘类交互时把采集物的 `action` 改成 `DIG`。
