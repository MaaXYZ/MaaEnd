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
// 数组顺序与调度节点 SeizeDeliveryJobsEndpointFilter 的 next 列表保持一致。
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

export const endpointFilterRows = ENDPOINT_DESTINATIONS.map(({endpoint, landmark, destinationId}) => {
    const destination = destinationById.get(destinationId);
    if (!destination) {
        throw new Error(`[SeizeDeliveryJobs] 终点 ${endpoint} 引用了未知送货终点 ${destinationId}`);
    }
    return {
        EndpointId: endpoint,
        Desc: `在${destination.area.zh_cn}大地图确认「${landmark}」送货终点（${destinationId}）`,
        MapZone: destination.mapZone,
        Icon: ENDPOINT_ICON,
        DestinationMapAt: destination.mapAt,
    };
});

// 调度节点 SeizeDeliveryJobsEndpointFilter 的 next：全部终点识别节点 + 未匹配兜底节点。
// 新增终点时只改上面的 ENDPOINT_DESTINATIONS，这里会自动带上，无需手动维护 next 列表。
export const endpointNodeNames = endpointFilterRows.map((row) => `SeizeDeliveryJobsEndpointFilter${row.EndpointId}`);

export const dispatcherRows = [
    {
        EndpointNodes: [
            ...endpointNodeNames,
            "SeizeDeliveryJobsEndpointNotMatched",
        ],
    },
];

export default endpointFilterRows;
