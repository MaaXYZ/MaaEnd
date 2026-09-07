#pragma once

#include <chrono>
#include <cmath>
#include <optional>
#include <string>
#include <variant>
#include <vector>

#include "navi_config.h"
#include "navi_position.h"

namespace mapnavigator
{

// 一根滑索架。身份是 level_id + 世界坐标; 执行侧瞄准和落地分类用的是定位器像素坐标与世界高度。
// 运行中现发现的架子没有世界坐标, 只能按像素圈认
struct ZiplineNodeRef
{
    std::string level_id;
    double world_x = 0.0;
    double world_y = 0.0;
    double world_z = 0.0;
    bool has_world = false;
    double x = 0.0;
    double y = 0.0;
    double height = 0.0;

    bool SameTower(const ZiplineNodeRef& other) const
    {
        if (has_world && other.has_world) {
            return level_id == other.level_id
                   && std::hypot(world_x - other.world_x, world_y - other.world_y, world_z - other.world_z) < kZiplineTowerIdentityWu;
        }
        return std::hypot(x - other.x, y - other.y) < kZiplineLandingBandWu;
    }
};

// 一跳的计划, 由滑索规划写进 ZIPLINE 航点
struct ZiplineHopPlan
{
    ZiplineNodeRef mount;
    ZiplineNodeRef landing;
    // 同一上索架上其它索通向的架子。两根索靠得近时游戏可能挂错一根, 落地分类靠它们认出滑到了哪
    std::vector<ZiplineNodeRef> siblings;
    // 索的仰角, 正数往上滑。按两端世界坐标算好, 运行时不再碰单位
    double planned_elevation_deg = 0.0;
    // 上索认不出提示时的备用站位。架子旁边没有供电结构就不写
    std::optional<ZiplineRestand> restand;
    // 落点就是下一跳的上索架: 落地不下索, 直接接着瞄
    bool chain_continues = false;
};

enum class LandingClass
{
    Pending,
    AtTarget,     // 本次发射的目标节点圈内(首发/重试 = 规划落点, 回程 = 上索架)
    AtOrigin,     // 还在本次发射的起点节点圈内, 没发出去
    AtOther,      // 其它已知节点圈内, 滑错索了
    AtStrayTower, // 滑行发生过且停稳, 却不在任何已知节点圈内: 脚下是一根未登记的架子
    Unknown,      // 滑行没发生过、定位又不在起点圈内: 定位还没对上
};

struct ZiplineLaunch
{
    std::chrono::steady_clock::time_point fired_at {};
    double heading_deg = 0.0;
    int pitch_tier = 0;
    double aim_bias_deg = 0.0;
    LandingClass result = LandingClass::Pending;
    std::optional<ZiplineNodeRef> reached;
    std::optional<NaviPosition> stopped_at;
};

enum class HopOutcome
{
    Completed,
    WrongRope,  // 滑错过, 滑回后重试仍没到
    NoLaunch,   // 每档俯仰都没发出去: 索被挡、没上去, 观测上同形, 不再分
    Lost,       // 落地定位一直对不上
    Dismounted, // 主动下索交给恢复
};

struct ZiplineHopRecord
{
    ZiplineHopPlan plan;
    std::vector<ZiplineLaunch> launches;
    HopOutcome outcome = HopOutcome::NoLaunch;
    // 从上索架指向到过的错误落点, 重试时瞄准从这些方向让开
    std::vector<double> wrong_rope_bearings_deg;
    std::chrono::steady_clock::time_point began_at {};
    std::chrono::steady_clock::time_point ended_at {};
};

// 一趟导航里每一跳的记录。规划器据此排掉滑不动/滑错的索, 主状态机据此决定要不要整段退回走路
using HopLedger = std::vector<ZiplineHopRecord>;

// 上索键、下索键按出去就当成了, 没有二次确认的阶段
enum class ZiplineStage
{
    Idle,
    OnTower,      // 站在架子上, 准备这一次发射
    Aiming,       // 闭环转 yaw, 开环俯仰, 然后左键
    Fired,        // 已左键, 等定位断掉(滑出去了)或位置离开起点
    Riding,       // 滑行中小地图整个隐藏, 定位不到, 只等
    Landed,       // 定位回来了, 等停稳
    Classified,   // 停稳的位置查决策表
    ReturnAiming, // 滑错了: 从当前架子瞄回上索架
    Dismounting,  // 已发下索键, 等定位稳定
    Handoff,      // 把出口事件交回导航
    Failed,
};

// 一帧观测。fix 只在定位到且不是 held 时有值, 朝向就是 fix 的角度
struct ZiplineObservation
{
    std::optional<NaviPosition> fix;
    std::chrono::steady_clock::time_point at {};
};

// 阶段机交回导航的出口事件
struct HopCompleted
{
    NaviPosition at;
    bool still_on_tower = false;
};

struct ChainAbandoned
{
    const char* reason = "";
};

struct ReplanRequested
{
    bool still_on_tower = false;
    std::optional<ZiplineNodeRef> on_tower;
};

// 上索点站得不对, 人已经下来了。有备用站位就改瞄它, 没有就只收紧判定圈, 让人挪一下再上一次
struct NeedsReposition
{
    std::optional<ZiplineRestand> restand;
};

using StageResult = std::variant<std::monostate, HopCompleted, ChainAbandoned, ReplanRequested, NeedsReposition>;

} // namespace mapnavigator
