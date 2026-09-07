#pragma once

#include <chrono>
#include <cstdint>
#include <optional>
#include <vector>

#include "zipline_types.h"

namespace mapnavigator
{

// 阶段机看世界的唯一窗口。hint_nodes 是这一帧定位的搜索先验
class IZiplineObserver
{
public:
    virtual ~IZiplineObserver() = default;
    virtual ZiplineObservation Observe(const std::vector<ZiplineNodeRef>& hint_nodes) = 0;
    virtual void ResetTracking() = 0;
};

// 阶段机的手。每个动作同步发出、发完即回, 转没转到位由后面的观测说了算
class IZiplineActuator
{
public:
    virtual ~IZiplineActuator() = default;
    virtual bool ResetPitchToMaximum() = 0;
    // 只发一个后端批次, 返回实际发出的度数; 发不出去返回空
    virtual std::optional<double> TurnYaw(double delta_deg) = 0;
    virtual bool TurnPitch(double delta_deg) = 0;
    virtual void FireLaunch() = 0;
    virtual void Dismount() = 0;
    virtual void Wait(int32_t ms) = 0;
};

// 落地分类: 停稳的定位落在哪个已知节点圈里。圈外时滑行发生过就是脚下有根未登记的架子,
// 没发生过就是定位还没对上。reached 给出对应的节点, 未登记架子按定位坐标现造一个
LandingClass ClassifyLanding(
    const std::optional<NaviPosition>& fix,
    const ZiplineNodeRef& origin,
    const ZiplineNodeRef& target,
    const std::vector<ZiplineNodeRef>& known,
    bool riding_entered,
    ZiplineNodeRef* reached);

// 一跳滑索从站上架子到交还导航的阶段机。上索键、下索键按出去就当成了, 之后只看定位:
// 起滑后连着定位不到就是滑出去了, 定位回来并停稳就是落地了, 落在哪由分类决定。
// 每跳的发射与结果记进账本, 规划器据此排掉滑不动/滑错的索
class ZiplineRideMachine
{
public:
    // 开一跳。人已经站在架子上(链中续跳、滑回原架)时直接瞄, 否则由调用方先按上索键再来
    void Begin(const ZiplineHopPlan& plan);
    StageResult Tick(IZiplineObserver& observer, IZiplineActuator& actuator);
    // 外部要人立刻下来(丢链、换路)。跳还开着就按 Dismounted 记账
    void Dismount(IZiplineActuator& actuator);

    bool OnTower() const;
    std::optional<ZiplineNodeRef> TowerUnderfoot() const;

    ZiplineStage stage() const { return stage_; }

    const HopLedger& ledger() const { return ledger_; }

    // 清掉这一跳, 账本留着
    void Reset();
    void ResetNavigation();

private:
    using Clock = std::chrono::steady_clock;

    void EnterStage(ZiplineStage stage, Clock::time_point now);
    int64_t StageElapsedMs(Clock::time_point now) const;
    void CommitRecord(HopOutcome outcome, Clock::time_point now);
    std::vector<ZiplineNodeRef> KnownNodes() const;
    double AimBiasDeg() const;

    StageResult TickOnTower(IZiplineActuator& actuator, Clock::time_point now);
    StageResult TickAiming(const ZiplineObservation& obs, IZiplineActuator& actuator);
    StageResult TickFired(const ZiplineObservation& obs, IZiplineObserver& observer, IZiplineActuator& actuator);
    StageResult TickRiding(const ZiplineObservation& obs, IZiplineObserver& observer, IZiplineActuator& actuator);
    StageResult TickLanded(const ZiplineObservation& obs, IZiplineObserver& observer, IZiplineActuator& actuator);
    StageResult Classify(IZiplineObserver& observer, IZiplineActuator& actuator, Clock::time_point now);
    StageResult TickDismounting(const ZiplineObservation& obs, IZiplineActuator& actuator);
    StageResult FailAim(IZiplineActuator& actuator, const char* reason, Clock::time_point now);
    StageResult StartDismount(IZiplineActuator& actuator, StageResult exit, Clock::time_point now);
    StageResult Handoff(Clock::time_point now);

    ZiplineStage stage_ = ZiplineStage::Idle;
    Clock::time_point stage_entered_at_ {};
    ZiplineHopPlan plan_;
    ZiplineHopRecord record_;
    bool hop_open_ = false;
    HopLedger ledger_;

    // 本次发射从 origin_ 瞄 target_。回程时 origin_ 是滑到的那根架子, target_ 是上索架
    ZiplineNodeRef origin_;
    ZiplineNodeRef target_;
    double seed_elevation_deg_ = 0.0;
    double aim_bias_deg_ = 0.0;
    int pitch_tier_ = 0;
    bool returning_ = false;
    int hop_retry_count_ = 0;
    bool mount_retry_used_ = false;
    std::vector<ZiplineNodeRef> discovered_towers_;
    // 交回导航后人还站着的架子
    std::optional<ZiplineNodeRef> parked_on_;

    std::optional<NaviPosition> launch_fix_;
    std::optional<NaviPosition> last_fix_;
    int miss_streak_ = 0;
    int settle_hits_ = 0;
    bool riding_entered_ = false;
    std::optional<Clock::time_point> unknown_deadline_;

    // 瞄准闭环: 一次只发一个批次, 等朝向读数跟上并稳定后再算剩余, 顺手估一次转向增益
    std::optional<double> yaw_gain_;
    std::optional<double> prev_heading_;
    int stable_heading_hits_ = 0;
    bool turn_pending_ = false;
    double turn_ref_heading_ = 0.0;
    double turn_cmd_deg_ = 0.0;
    Clock::time_point turn_sent_at_ {};

    int dismount_presses_ = 0;
    std::optional<NaviPosition> dismount_stable_pos_;
    int dismount_stable_hits_ = 0;
    StageResult pending_exit_;
};

} // namespace mapnavigator
