import {readFileSync, readdirSync} from "node:fs";
import {resolve} from "node:path";

import {BASE_NAV_ZONE_IMAGE_PARTS} from "../../MapNavigator/web/static/js/model.js";
import {readJsonc} from "../jsonc.mjs";
import {repoRoot} from "../utils/paths.mjs";

export const catalogSource = JSON.parse(
    readFileSync(new URL("../data/collect_points.json", import.meta.url), "utf8"),
);

const PIPELINE_DIR = resolve(repoRoot, "assets", "resource", "pipeline");
const TELEPORT_SOURCE_DIRS = [
    "SceneManager",
    "Interface",
];
const TELEPORT_RECOGNITION = "MapFind";
const TELEPORT_ICON = "TeleportAnchor";
const TELEPORT_PICK_ANCHOR = "__ScenePrivateMapTeleportPickAnchor";
const ENTER_NODE_PREFIX = "SceneEnterWorld";
const BASE_MAP_IMAGE = "Base.png";
// MapFind 的 at 与篝火实体的底图投影同源，实测偏差都在 1 像素内。
const TELEPORT_MATCH_PX = 3;

export function assertArray(value, label) {
    if (!Array.isArray(value)) {
        throw new TypeError(`[AutoCollect] ${label} 必须是数组`);
    }
    return value;
}

export function assertNonEmptyString(value, label) {
    if (typeof value !== "string" || value.trim() === "") {
        throw new TypeError(`[AutoCollect] ${label} 必须是非空字符串`);
    }
    return value;
}

export function distance(left, right) {
    return Math.hypot(left.u - right.u, left.v - right.v);
}

export function buildNodeId(sourceId) {
    return sourceId
        .split(/[^A-Za-z0-9]+/)
        .filter(Boolean)
        .map((part) => `${part[0].toUpperCase()}${part.slice(1)}`)
        .join("");
}

/** BaseNav 地区对应的 MapLocator 区域：MapFind 用区域名，寻路的 ZONE 用底图 ID。 */
function buildZoneIds(navZone, label) {
    const [
        resourceType,
        zone,
        image,
    ] = BASE_NAV_ZONE_IMAGE_PARTS[navZone] ?? [];
    if (resourceType !== "MapLocator" || !zone || image !== BASE_MAP_IMAGE) {
        throw new Error(`[AutoCollect] ${label} 的 BaseNav 地区 ${navZone} 不是大世界底图`);
    }
    return {
        icon: zone,
        navigate: `${zone}_${image.slice(0, -".png".length)}`,
    };
}

export const maps = new Map(
    Object.entries(catalogSource.maps ?? {}).map(([
        mapId,
        map,
    ]) => [
        mapId,
        {
            id: mapId,
            ...buildZoneIds(assertNonEmptyString(map?.zone, `maps.${mapId}.zone`), `地图 ${mapId}`),
        },
    ]),
);
const mapByIconZone = new Map([...maps.values()].map((map) => [
    map.icon,
    map,
]));

function* iterJsonFiles(dir) {
    for (const entry of readdirSync(dir, {withFileTypes: true})) {
        const path = resolve(dir, entry.name);
        if (entry.isDirectory()) {
            yield* iterJsonFiles(path);
        } else if (entry.name.endsWith(".json")) {
            yield path;
        }
    }
}

function* iterPipelineNodes() {
    for (const dir of TELEPORT_SOURCE_DIRS) {
        for (const path of iterJsonFiles(resolve(PIPELINE_DIR, dir))) {
            const document = readJsonc(path);
            if (!document || typeof document !== "object") {
                continue;
            }
            for (const [
                name,
                node,
            ] of Object.entries(document)) {
                if (node && typeof node === "object" && !Array.isArray(node)) {
                    yield [
                        name,
                        node,
                    ];
                }
            }
        }
    }
}

/** MapFind 支持一次判定多个候选点，at 既可能是一个坐标也可能是一组坐标。 */
function readAnchorPositions(at) {
    if (!Array.isArray(at) || at.length === 0) {
        return [];
    }
    const positions = Array.isArray(at[0]) ? at : [at];
    return positions
        .filter((position) => Array.isArray(position) && position.length >= 2)
        .map(([
            u,
            v,
        ]) => ({u, v}));
}

/** 传送锚点：私有 MapFind 节点提供坐标，外层入口节点通过 anchor 绑定它。 */
function buildTeleportNodes() {
    const anchors = new Map();
    const enters = [];
    for (const [
        name,
        node,
    ] of iterPipelineNodes()) {
        const parameter = node.custom_recognition_param ?? {};
        if (node.custom_recognition === TELEPORT_RECOGNITION) {
            anchors.set(name, {
                icon: parameter.icon,
                zone: parameter.zone,
                positions: readAnchorPositions(parameter.at),
            });
        }
        const anchor = node.anchor?.[TELEPORT_PICK_ANCHOR];
        if (typeof anchor === "string" && anchor !== "" && name.startsWith(ENTER_NODE_PREFIX)) {
            enters.push({
                node: name,
                anchor,
                desc: typeof node.desc === "string" ? node.desc : "",
            });
        }
    }
    return {anchors, enters};
}

/** 传送入口 → 落点篝火：MapFind 的 at 落在哪个篝火上，那个篝火就是这条入口的落点。 */
export function buildTeleports() {
    const {anchors, enters} = buildTeleportNodes();
    const campfires = assertArray(catalogSource.campfires, "collect_points.campfires");
    const teleports = [];
    const seenNodes = new Set();

    for (const enter of enters.sort((left, right) => left.node.localeCompare(right.node))) {
        const anchor = anchors.get(enter.anchor);
        if (!anchor) {
            throw new Error(`[AutoCollect] 传送入口 ${enter.node} 绑定的锚点 ${enter.anchor} 不存在`);
        }
        // 同一套入口里还有核心区一类的其他图标，只有传送锚点是采集路线的起点。
        const map = anchor.icon === TELEPORT_ICON ? mapByIconZone.get(anchor.zone) : undefined;
        if (!map) {
            continue;
        }
        const landed = [];
        for (const position of anchor.positions) {
            const pool = campfires.filter((campfire) => campfire.map === map.id);
            const nearest = pool.reduce(
                (best, campfire) =>
                    best === null || distance(campfire, position) < distance(best, position) ? campfire : best,
                null,
            );
            if (nearest && distance(nearest, position) <= TELEPORT_MATCH_PX && !landed.includes(nearest)) {
                landed.push(nearest);
            }
        }
        if (landed.length !== 1) {
            // 落点不唯一的入口无法确定采集起点，交给人工在 routes.json 里处理。
            console.warn(`[AutoCollect] 传送入口 ${enter.node} 匹配到 ${landed.length} 个篝火，已跳过`);
            continue;
        }
        if (seenNodes.has(enter.node)) {
            throw new Error(`[AutoCollect] 传送入口 ${enter.node} 重复`);
        }
        seenNodes.add(enter.node);
        teleports.push({
            campfireId: landed[0].id,
            campfire: landed[0],
            map,
            enterNode: enter.node,
            routeFileId: enter.node.slice(ENTER_NODE_PREFIX.length),
            // desc 形如「从任意界面进入-景玉谷-生态实验站」，后两段就是这个落点的位置。
            name: enter.desc.split("-").slice(1).join("-") || enter.node,
        });
    }
    return teleports.sort((left, right) => left.campfireId.localeCompare(right.campfireId));
}
