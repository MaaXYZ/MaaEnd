import {readFileSync} from "node:fs";

import {
    assertArray,
    assertNonEmptyString,
    buildNodeId,
    buildTeleports,
    catalogSource,
    distance,
} from "./teleports.mjs";

const routeSource = JSON.parse(readFileSync(new URL("./routes.json", import.meta.url), "utf8"));

// 一条路线覆盖篝火周围这个半径内、同一种类且够数的采集物。
const CLUSTER_RADIUS_PX = 90;
const CLUSTER_MIN_POINTS = 6;
const DOODAD_PREFIX = "int_doodad_";
const DEFAULT_COLLECT_ACTION = "COLLECT";
const COLLECT_ACTIONS = new Set([
    "COLLECT",
    "DIG",
]);
const COORDINATE_PRECISION = 2;

function buildOverrides(items, label) {
    const overrides = new Map();
    for (const [
        index,
        item,
    ] of assertArray(items, label).entries()) {
        const sourceId = assertNonEmptyString(item.source_id, `${label}[${index}].source_id`);
        if (overrides.has(sourceId)) {
            throw new Error(`[AutoCollect] ${label} 存在重复项：${sourceId}`);
        }
        overrides.set(sourceId, item);
    }
    return overrides;
}

function readSkip(value, label) {
    if (value === undefined) {
        return false;
    }
    if (typeof value !== "boolean") {
        throw new TypeError(`[AutoCollect] ${label}.skip 必须是布尔值`);
    }
    return value;
}

function readPositiveNumber(value, fallback, label) {
    if (value === undefined) {
        return fallback;
    }
    if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
        throw new TypeError(`[AutoCollect] ${label} 必须是正数`);
    }
    return value;
}

function readAction(override, detailId) {
    const action = override?.action ?? DEFAULT_COLLECT_ACTION;
    if (!COLLECT_ACTIONS.has(action)) {
        throw new Error(`[AutoCollect] 采集物 ${detailId} 的 action 无效：${action}`);
    }
    return action;
}

function roundCoordinate(value) {
    return Number(value.toFixed(COORDINATE_PRECISION));
}

/** 篝火周边按种类分组，够数的才值得单独出一条路线。 */
function buildClusters(teleport, points, radius, minPoints) {
    const byDetail = new Map();
    for (const point of points) {
        if (point.map !== teleport.map.id || distance(point, teleport.campfire) > radius) {
            continue;
        }
        const group = byDetail.get(point.detail_id) ?? [];
        group.push(point);
        byDetail.set(point.detail_id, group);
    }
    return [...byDetail.entries()]
        .filter(([, group]) => group.length >= minPoints)
        .sort(([left], [right]) => left.localeCompare(right));
}

/** 从离篝火最近的一颗起步，每次走到最近的下一颗。 */
export function orderByNearest(points, start) {
    const remaining = [...points];
    const ordered = [];
    let current = start;
    while (remaining.length > 0) {
        let pick = 0;
        for (let index = 1; index < remaining.length; index += 1) {
            if (distance(remaining[index], current) < distance(remaining[pick], current)) {
                pick = index;
            }
        }
        [current] = remaining.splice(pick, 1);
        ordered.push(current);
    }
    return relaxCrossings(ordered);
}

/** 贪心一路只看下一颗，末尾要绕回去捡沿途跳过的，线就自己交叉了。
 *  反复反转能缩短总长的子段把交叉消掉，起点保持不动。 */
export function relaxCrossings(ordered) {
    const best = [...ordered];
    let improved = true;
    while (improved) {
        improved = false;
        for (let head = 1; head < best.length - 1; head += 1) {
            for (let tail = head + 1; tail < best.length; tail += 1) {
                const hasNext = tail + 1 < best.length;
                const before =
                    distance(best[head - 1], best[head]) +
                    (hasNext ? distance(best[tail], best[tail + 1]) : 0);
                const after =
                    distance(best[head - 1], best[tail]) +
                    (hasNext ? distance(best[head], best[tail + 1]) : 0);
                if (after < before - 1e-9) {
                    best.splice(head, tail - head + 1, ...best.slice(head, tail + 1).reverse());
                    improved = true;
                }
            }
        }
    }
    return best;
}

function buildPath(teleport, ordered, action) {
    const [start] = ordered;
    return [
        {
            action: "ZONE",
            zone_id: teleport.map.navigate,
        },
        {
            action: "NAVMESH",
            target: [
                roundCoordinate(start.u),
                roundCoordinate(start.v),
            ],
        },
        ...ordered.map((point) => [
            roundCoordinate(point.u),
            roundCoordinate(point.v),
            action,
        ]),
    ];
}

const campfireOverrides = buildOverrides(routeSource.campfires, "routes.campfires");
const collectibleOverrides = buildOverrides(routeSource.collectibles, "routes.collectibles");
const points = assertArray(catalogSource.points, "collect_points.points");
export const teleports = buildTeleports();

export const routes = teleports.flatMap((teleport) => {
    const override = campfireOverrides.get(teleport.campfireId);
    if (readSkip(override?.skip, `篝火 ${teleport.campfireId}`)) {
        return [];
    }
    const radius = readPositiveNumber(
        override?.radius,
        CLUSTER_RADIUS_PX,
        `篝火 ${teleport.campfireId} 的 radius`,
    );
    const minPoints = readPositiveNumber(
        override?.min_points,
        CLUSTER_MIN_POINTS,
        `篝火 ${teleport.campfireId} 的 min_points`,
    );

    return buildClusters(teleport, points, radius, minPoints).flatMap(([
        detailId,
        group,
    ]) => {
        const collectible = collectibleOverrides.get(detailId);
        if (readSkip(collectible?.skip, `采集物 ${detailId}`)) {
            return [];
        }
        const ordered = orderByNearest(group, teleport.campfire);
        const nodeId = `AutoCollectRoute${teleport.routeFileId}${buildNodeId(
            detailId.startsWith(DOODAD_PREFIX) ? detailId.slice(DOODAD_PREFIX.length) : detailId,
        )}`;
        return [
            {
                routeFileId: teleport.routeFileId,
                campfireId: teleport.campfireId,
                detailId,
                name: teleport.name,
                count: ordered.length,
                enterNode: teleport.enterNode,
                startNode: `${nodeId}Start`,
                gotoNode: `${nodeId}Goto`,
                endNode: `${nodeId}End`,
                path: buildPath(teleport, ordered, readAction(collectible, detailId)),
            },
        ];
    });
});

for (const id of campfireOverrides.keys()) {
    if (!teleports.some((teleport) => teleport.campfireId === id)) {
        console.warn(`[AutoCollect] 篝火覆盖 ${id} 没有对应的传送入口，已忽略。`);
    }
}

export function rawJson(value) {
    return {value, raw: JSON.stringify(value, null, 4)};
}
