import {destinations} from "../AutoDelivery/model.mjs";

// MapFind 认的是点击「查看位置」后游戏在送货终点坐标上绘制的那个标记图标，
// 七个终点共用同一个图标、靠 at 坐标区分（每个终点的底图坐标各不相同）。
// 图标条目 DeliveryPoint 登记在 assets/resource/image/SceneManager/MapIcons.json，
// 模板图为 assets/resource/image/SeizeDeliveryJobs/DeliveryPoint.png（阈值待游戏内微调）。
const ENDPOINT_ICON = "DeliveryPoint";

// 终点节点后缀 → delivery_destinations.json 里的送货终点 ID。
// 对应关系取自 tools/pipeline-generate/AutoDelivery/routes.json 中各 destination 的 description 字段：
//   deliver_target_map02_lv002_01（苏白易）             → "技术生产办公室（左上）"
//   deliver_target_map02_lv002_02（普里莫·林德）        → "材料研究所（左下）"
//   deliver_target_map02_lv002_03（于施）               → "观测站（右上）"
//   deliver_target_map02_lv002_recycle_01（火锅探店达人）→ "猫头鹰（资源回收站）"
//   deliver_target_map02_lv005_01（赵昭）               → "经纬田区（右）"
//   deliver_target_map02_lv005_02（裴令容）             → 无 description，按排除法 + 几何（u 最小、v 最小 = 左上）对应 "一号丙型辅桩区（左上）"
//   deliver_target_map02_lv005_03（阿禾）               → "三号丙型辅桩区（右上）"
// 也与 assets/tasks/SeizeDeliveryJobs.json 的区域分组一致：武陵城 = lv002 四个，试验园区 = lv005 三个。
// 数组顺序即 candidates 的书写顺序：首个确认到图标的候选胜出，排在其后的不再看。
const ENDPOINT_DESTINATIONS = [
    {
        endpoint: "Owl",
        landmark: "猫头鹰",
        destinationId: "deliver_target_map02_lv002_recycle_01",
    },
    {
        endpoint: "MaterialResearchInstitute",
        landmark: "材料研究所",
        destinationId: "deliver_target_map02_lv002_02",
    },
    {
        endpoint: "Observatory",
        landmark: "观测站",
        destinationId: "deliver_target_map02_lv002_03",
    },
    {
        endpoint: "TechProductionOffice",
        landmark: "技术生产办公室",
        destinationId: "deliver_target_map02_lv002_01",
    },
    {
        endpoint: "No1TypeCAnchorArea",
        landmark: "一号丙型辅桩区",
        destinationId: "deliver_target_map02_lv005_02",
    },
    {
        endpoint: "No3TypeCAnchorArea",
        landmark: "三号丙型辅桩区",
        destinationId: "deliver_target_map02_lv005_03",
    },
    {
        endpoint: "JingweiFieldArea",
        landmark: "经纬田区",
        destinationId: "deliver_target_map02_lv005_01",
    },
];

const destinationById = new Map(
    destinations.map((item) => [
        item.id,
        item,
    ]),
);

// 每个终点解析出节点后缀、说明、地图区域、底图坐标，驱动两处产物：
// candidates 节点的候选数组，以及每个终点的「开关 + 命中落点」叶子节点。
const endpointEntries = ENDPOINT_DESTINATIONS.map(({endpoint, landmark, destinationId}) => {
    const destination = destinationById.get(destinationId);
    if (!destination) {
        throw new Error(`[SeizeDeliveryJobs] 终点 ${endpoint} 引用了未知送货终点 ${destinationId}`);
    }
    return {
        EndpointId: endpoint,
        Desc: `「${landmark}」送货终点（${destinationId}）：candidates 候选开关，命中后前往接取`,
        MapZone: destination.mapZone,
        DestinationMapAt: destination.mapAt,
    };
});

// 叶子节点：candidates 每个候选的开关兼命中落点。enabled 默认关，由 task 选项逐个打开；
// 关着的候选在 MapFind 里连认都不认、直接跳过。节点本身不再做识别，命中后直接前往接取。
export const endpointFilterRows = endpointEntries.map(({EndpointId, Desc}) => ({
    EndpointId,
    Desc,
}));

export const endpointNodeNames = endpointEntries.map((row) => `SeizeDeliveryJobsEndpointFilter${row.EndpointId}`);

// candidates 整组必须同 zone（一个 MapFind 节点只有一个 zone）。若将来新增了别的地图区域的终点，
// 需要为不同 zone 各起一个 candidates 节点，这里直接报错提示，避免默默生成一个跨区认不对的节点。
const zones = [
    ...new Set(endpointEntries.map((entry) => entry.MapZone)),
];
if (zones.length !== 1) {
    throw new Error(`[SeizeDeliveryJobs] candidates 需所有终点同 zone，当前有 ${zones.join(", ")}；请为不同 zone 各起一个 candidates 节点`);
}

// candidates 节点数据（单行）：候选按 ENDPOINT_DESTINATIONS 顺序书写，共享一次缩放与视口求解。
export const candidatesRows = [
    {
        Zone: zones[0],
        Icon: ENDPOINT_ICON,
        Candidates: endpointEntries.map((entry) => ({
            at: entry.DestinationMapAt,
            next: `SeizeDeliveryJobsEndpointFilter${entry.EndpointId}`,
        })),
    },
];

export default endpointFilterRows;
