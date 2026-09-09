import {readFileSync, writeFileSync} from "node:fs";
import {resolve} from "node:path";
import {pathToFileURL} from "node:url";

import {assertArray, assertNonEmptyString, buildTeleports, catalogSource} from "./teleports.mjs";

const routesPath = new URL("./routes.json", import.meta.url);

const CAMPFIRE_METADATA_KEYS = new Set([
    "source_id",
    "name",
    "enter_node",
]);
const COLLECTIBLE_METADATA_KEYS = new Set([
    "source_id",
    "count",
]);

function buildRouteIndex(routes, label) {
    const routeById = new Map();
    for (const [
        index,
        route,
    ] of assertArray(routes, label).entries()) {
        const sourceId = assertNonEmptyString(route.source_id, `${label}[${index}].source_id`);
        if (routeById.has(sourceId)) {
            throw new Error(`[AutoCollect] ${label} 存在重复项：${sourceId}`);
        }
        routeById.set(sourceId, route);
    }
    return routeById;
}

function preserveRouteFields(synced, route, metadataKeys) {
    if (!route) {
        return synced;
    }
    for (const [
        key,
        value,
    ] of Object.entries(route)) {
        if (!metadataKeys.has(key)) {
            synced[key] = value;
        }
    }
    return synced;
}

function syncItems(metadataList, routes, label, metadataKeys) {
    const routeById = buildRouteIndex(routes, label);
    const synced = metadataList.map((metadata) => {
        const route = routeById.get(metadata.source_id);
        routeById.delete(metadata.source_id);
        return preserveRouteFields(metadata, route, metadataKeys);
    });

    for (const route of routeById.values()) {
        console.warn(`[AutoCollect] ${label}条目 ${route.source_id} 未匹配到当前游戏数据，已保留等待人工处理。`);
        synced.push(route);
    }

    return synced.sort((left, right) => left.source_id.localeCompare(right.source_id));
}

export function buildSyncedRouteConfig(routes) {
    const campfires = syncItems(
        buildTeleports().map((teleport) => ({
            source_id: teleport.campfireId,
            name: teleport.name,
            enter_node: teleport.enterNode,
        })),
        routes.campfires,
        "篝火",
        CAMPFIRE_METADATA_KEYS,
    );

    const counts = new Map();
    for (const point of assertArray(catalogSource.points, "collect_points.points")) {
        counts.set(point.detail_id, (counts.get(point.detail_id) ?? 0) + 1);
    }
    const collectibles = syncItems(
        [...counts.entries()]
            .sort(([left], [right]) => left.localeCompare(right))
            .map(([
                detailId,
                count,
            ]) => ({source_id: detailId, count})),
        routes.collectibles,
        "采集物",
        COLLECTIBLE_METADATA_KEYS,
    );

    return {
        campfires,
        collectibles,
    };
}

export function syncRouteConfig() {
    const originalText = readFileSync(routesPath, "utf8");
    const routes = JSON.parse(originalText);
    const syncedText = `${JSON.stringify(buildSyncedRouteConfig(routes), null, 4)}\n`;

    if (syncedText === originalText.replace(/\r\n/g, "\n")) {
        console.log("[AutoCollect] routes.json 元数据未变化");
        return;
    }
    writeFileSync(routesPath, syncedText, "utf8");
    console.log("[AutoCollect] 已同步 routes.json 的篝火与采集物元数据，并保留人工字段");
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
    syncRouteConfig();
}
