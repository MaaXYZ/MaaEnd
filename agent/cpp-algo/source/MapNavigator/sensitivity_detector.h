#pragma once

#include <array>
#include <cstdint>
#include <optional>

namespace mapnavigator
{

namespace sensitivity
{

// 一条指令的转量摊在后面十来拍上，逐拍相除量到的是滞后。拿这么多拍一起拟合才是灵敏度。
inline constexpr int kLagCount = 12;

using LagVector = std::array<double, kLagCount>;
using LagMatrix = std::array<LagVector, kLagCount>;

struct Config
{
    // 只认转过头。转不到位有太多正常原因（指令被吞、贴墙转不动、掉帧），低倍率分不出是哪种。
    double overshoot_ratio = 1.20;
    // 估计值减去这么多倍标准误差还过线才判。正常机的估计值落在 1.00 附近 ±0.05。
    double sigma_margin = 3.0;
    // 估计值再高就当观测本身出了问题，不改。
    double max_ratio = 2.5;
    // 攒够这么多拍样本才开始判，之后样本每翻一倍再判一次，线路结束也判一次。
    int min_samples = 500;
    // 拍号跳一格且那拍没发过转向，两拍相隔在此之内才把两拍并成一行记账。
    int64_t bridge_max_gap_ms = 600;
    // 出口记的账和这拍报的指令差出这么多度，就是别的路径发过转向，滞后链作废。
    double issued_match_tol_deg = 0.5;
};

struct Verdict
{
    double ratio = 1.0;
    int ratio_percent = 0;
    int sample_count = 0;
};

// 当前的估计值和它的标准误差，留给日志。
struct Estimate
{
    double ratio = 0.0;
    double se = 0.0;
    int sample_count = 0;
    double cmd_deg = 0.0;
};

// 比对发出的转向指令和实测的朝向变化，算出实际转到了指令的百分之多少。
// 整个进程一直攒着：灵敏度是个设置，不会这条线路对下条线路错。
class Detector
{
public:
    explicit Detector(Config config = {});

    // 每次导航开始调一次：断开滞后链，攒下的方程全部保留。
    void BeginRun();
    // 每一份从输入出口发出去的偏航度数，不论哪条路径发的。
    void NoteIssued(double delta_deg);
    std::optional<Verdict> RecordTick(uint64_t tick_seq, int64_t now_ms, double heading_deg, double issued_delta_deg, bool degraded_fix);
    // 线路结束时调一次。
    std::optional<Verdict> EndRun();

    bool fired() const { return fired_; }

    // 上一次算出的估计值，取走就清空。
    std::optional<Estimate> TakeLastEstimate();

private:
    void PushLag(double cmd_deg);
    void Accumulate(const LagVector& row, double heading_delta, double issued_delta_deg);
    void ResetAccumulators();
    std::optional<Estimate> Solve() const;
    std::optional<Verdict> Evaluate();

    Config config_;

    uint64_t prev_tick_seq_ = 0;
    bool has_prev_tick_ = false;
    int64_t prev_tick_ms_ = 0;
    double prev_heading_deg_ = 0.0;
    bool has_prev_heading_ = false;
    double issued_since_record_ = 0.0;

    // 前 kLagCount 拍的指令，[0] 是上一拍。攒不满说明账刚断过，这拍不进方程。
    LagVector cmd_lags_ {};
    int chain_len_ = 0;

    // 正规方程的累加量。断拍只清滞后链，不动这些：只有「哪拍对哪拍」要连续，统计量不要。
    LagMatrix xtx_ {};
    LagVector xty_ {};
    double yty_ = 0.0;
    int sample_count_ = 0;
    double cmd_deg_ = 0.0;
    int next_eval_at_ = 0;

    std::optional<Estimate> last_estimate_;
    bool fired_ = false;
};

// 归一到 (-180, 180]，两个朝向读数相减时用。
double NormalizeDeg(double deg);

} // namespace sensitivity

} // namespace mapnavigator
