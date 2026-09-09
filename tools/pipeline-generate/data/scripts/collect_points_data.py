"""生成 AutoCollect 使用的 collect_points.json。"""

from __future__ import annotations

import argparse
import math
import re
import struct
import sys
import urllib.error
import zlib
from collections.abc import Sequence
from pathlib import Path
from typing import Any

from navzone_utils import (
    describe_zones,
    load_nav_zones,
    project_to_pixel,
    resolve_zone,
)
from tablecfg_utils import (
    DATA_DIR,
    DEFAULT_JSON_DATA_DIR,
    TableCfgError,
    assert_record,
    load_json_group,
    should_skip,
    sorted_entries,
    write_dataset,
)

LABEL = "CollectPoints"
OUTPUT_PATH = DATA_DIR / "collect_points.json"
DEFAULT_GAMEPLAY_CONFIG_DIR = DEFAULT_JSON_DATA_DIR / "GameplayConfig"
GAMEPLAY_CONFIG_NAMES = ("WorldEntityRegistry.json", "LevelBasicInfoTable.json")

DATA_BASE_URL = "https://assets.fz.wiki/output_maaend"
GAMEPLAY_CONFIG_BASE_URL: str | None = DATA_BASE_URL

CAMPFIRE_PREFIX = "int_campfire"
DOODAD_PREFIX = "int_doodad_"
# 采集路线一律从篝火传送出发，离所有篝火都超过这个距离的采集物不属于任何采集圈。
CAMPFIRE_RADIUS_PX = 120.0
# 当前接入采集任务的地图；覆盖新地图时把它的 map ID 加进来。
COLLECT_MAPS = ("map02",)

OPEN_WORLD_LEVEL_TYPE = 0
OPEN_WORLD_SCOPE = 1
ENTITY_ID_LEVEL_FACTOR = 10**8
# 只有 mapXX_lvYYY 形状的层级挂在大地图底图上，其余大世界层级没有对应 zone。
MAP_LEVEL_ID = re.compile(r"map\d+_lv\d+")


def build_open_world_levels(level_basic_info: dict[str, Any]) -> dict[int, str]:
    """大世界层级的 idNum → levelId；实体 id 除以 10^8 即所属层级的 idNum。"""
    levels: dict[int, str] = {}
    for level_id, entry_value in sorted_entries(level_basic_info):
        entry = assert_record(entry_value, f"LevelBasicInfoTable[{level_id}]")
        if (
            entry.get("levelType") != OPEN_WORLD_LEVEL_TYPE
            or entry.get("scope") != OPEN_WORLD_SCOPE
        ):
            continue
        if not MAP_LEVEL_ID.fullmatch(level_id):
            continue
        id_num = entry.get("idNum")
        if not isinstance(id_num, int) or isinstance(id_num, bool):
            raise TableCfgError(f"层级 {level_id} 的 idNum {id_num!r} 不是整数")
        if id_num in levels:
            raise TableCfgError(f"idNum {id_num} 同时属于 {levels[id_num]} 和 {level_id}")
        levels[id_num] = level_id
    if not levels:
        raise TableCfgError("LevelBasicInfoTable 里一个大世界层级都没有")
    return levels


def read_position(value: Any, label: str) -> tuple[float, float]:
    record = assert_record(value, label)
    coords: list[float] = []
    for axis in ("x", "z"):
        number = record.get(axis)
        if not isinstance(number, (int, float)) or isinstance(number, bool):
            raise TableCfgError(f"{label} 的 {axis} 不是数值")
        coords.append(float(number))
    return coords[0], coords[1]


def build_entities(
    registry: dict[str, Any],
    levels: dict[int, str],
    zones: dict[str, dict[str, Any]],
    used_zones: dict[str, dict[str, Any]],
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    brief_infos = assert_record(
        registry.get("worldEntityBriefInfos"), "WorldEntityRegistry.worldEntityBriefInfos"
    )
    campfires: list[dict[str, Any]] = []
    doodads: list[dict[str, Any]] = []
    for entity_id, entity_value in sorted_entries(brief_infos):
        entity = assert_record(entity_value, f"世界实体 {entity_id}")
        detail_id = entity.get("detailId")
        if not isinstance(detail_id, str):
            continue
        is_campfire = detail_id.startswith(CAMPFIRE_PREFIX)
        if not is_campfire and not detail_id.startswith(DOODAD_PREFIX):
            continue
        if not entity_id.isdigit():
            raise TableCfgError(f"世界实体 {entity_id} 的 id 不是数字")
        level_id = levels.get(int(entity_id) // ENTITY_ID_LEVEL_FACTOR)
        if level_id is None:
            continue

        map_id = level_id.split("_")[0]
        if map_id not in COLLECT_MAPS:
            continue
        zone = used_zones.get(map_id)
        if zone is None:
            zone = resolve_zone(zones, map_id)
            used_zones[map_id] = zone
        x, z = read_position(entity.get("position"), f"世界实体 {entity_id}.position")
        u, v = project_to_pixel(zone, x, z, f"世界实体 {entity_id}")

        record = {"id": entity_id, "detail_id": detail_id, "map": map_id, "u": u, "v": v}
        if is_campfire:
            campfires.append({**record, "level": level_id})
        else:
            doodads.append(record)
    if not campfires:
        raise TableCfgError("WorldEntityRegistry 里一个大世界篝火都没有")
    return campfires, doodads


def near_any_campfire(
    point: dict[str, Any], campfires_by_map: dict[str, list[tuple[float, float]]]
) -> bool:
    return any(
        math.hypot(u - point["u"], v - point["v"]) <= CAMPFIRE_RADIUS_PX
        for u, v in campfires_by_map.get(point["map"], ())
    )


def build_collect_points_data(
    gameplay_config: dict[str, Any], zones: dict[str, dict[str, Any]]
) -> dict[str, Any]:
    levels = build_open_world_levels(
        assert_record(gameplay_config["LevelBasicInfoTable.json"], "LevelBasicInfoTable")
    )
    used_zones: dict[str, dict[str, Any]] = {}
    campfires, doodads = build_entities(
        assert_record(gameplay_config["WorldEntityRegistry.json"], "WorldEntityRegistry"),
        levels,
        zones,
        used_zones,
    )

    campfires_by_map: dict[str, list[tuple[float, float]]] = {}
    for campfire in campfires:
        campfires_by_map.setdefault(campfire["map"], []).append(
            (campfire["u"], campfire["v"])
        )
    points = [point for point in doodads if near_any_campfire(point, campfires_by_map)]
    if not points:
        raise TableCfgError("没有任何采集物落在篝火附近，请确认几份输入取自同一版本")

    return {
        "text": {
            "campfires": (
                "大世界篝火，detailId 以 int_campfire 开头；传送锚点的地图坐标即这里的 u/v"
            ),
            "points": (
                "大世界可交互采集物，detailId 以 int_doodad_ 开头；"
                f"只保留距最近同图篝火 {CAMPFIRE_RADIUS_PX:.0f} 像素以内的点位"
            ),
            "level": "所属层级 ID，取自 LevelBasicInfoTable；实体 id 除以 10^8 即层级 idNum",
        },
        "maps": describe_zones(used_zones),
        "coord": (
            "u/v = MapLocator 底图像素，原点左上、y 向下；BaseNav 顶点也是这个平面（u, v, height），"
            "直接喂寻路即可。maps 里的 zone / sx / tx / sy / ty 原样取自 BaseNav pack 的 zone 表，"
            "世界坐标转进来是 u = sx*x + tx, v = -sy*z + ty"
        ),
        "campfire_count": len(campfires),
        "count": len(points),
        "campfires": campfires,
        "points": points,
    }


def parse_arguments(args: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="从 BeyondMemoryPack 和本地 BaseNav 生成采集点数据"
    )
    parser.add_argument(
        "--gameplay-config-dir",
        type=Path,
        default=DEFAULT_GAMEPLAY_CONFIG_DIR,
        help=(
            "BeyondMemoryPack 的 GameplayConfig 目录"
            f"（默认：{DEFAULT_GAMEPLAY_CONFIG_DIR}）"
        ),
    )
    parser.add_argument("--nav", default=None, help="nav 数据的本地路径或 URL")
    parser.add_argument("--output", type=Path, default=OUTPUT_PATH, help="输出文件")
    parser.add_argument("--force", action="store_true", help="强制重写输出文件")
    return parser.parse_args(args)


def main(args: Sequence[str] | None = None) -> int:
    options = parse_arguments(args)
    try:
        gameplay_config = load_json_group(
            GAMEPLAY_CONFIG_NAMES,
            options.gameplay_config_dir,
            GAMEPLAY_CONFIG_BASE_URL,
            "GameplayConfig",
            "--gameplay-config-dir",
        )
        zones = load_nav_zones(options.nav)
        data = build_collect_points_data(gameplay_config, zones)
        if should_skip(options.output, data, options.force):
            print(f"[{LABEL}] 生成结果未变化，跳过写入；可使用 --force 强制重写")
            return 0
        write_dataset(options.output, data)
        print(
            f"[{LABEL}] 已生成 {data['campfire_count']} 个篝火、"
            f"{data['count']} 个采集点：{options.output}"
        )
    except (
        OSError,
        ValueError,
        struct.error,
        zlib.error,
        urllib.error.URLError,
    ) as error:
        print(f"[{LABEL}] {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
