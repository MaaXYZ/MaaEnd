#include "sensitivity_observer.h"

#include <atomic>
#include <mutex>
#include <optional>

#include <MaaUtils/Logger.h>

#include "../Common/notice.h"
#include "sensitivity_detector.h"

namespace mapnavigator
{

namespace sensitivity
{

namespace
{

std::mutex g_mutex;
Detector g_detector;
std::atomic<double> g_turn_units_scale { 1.0 };

void PublishVerdict(MaaContext* context, const Verdict& verdict)
{
    const int ratio_percent = verdict.ratio_percent;
    const int window_count = verdict.window_count;
    // 判决只认过线的窗，倍率必大于 1，系数只会把转向量往小了调。
    const double scale = 1.0 / verdict.ratio;
    g_turn_units_scale.store(scale);
    LogInfo << "Turn sensitivity corrected." << VAR(ratio_percent) << VAR(window_count) << VAR(scale);

    common::notice::Publish(context, common::notice::Text("navigation.sensitivity_corrected", { ratio_percent, window_count }));
}

} // namespace

void BeginRun()
{
    const double scale = g_turn_units_scale.load();
    if (scale != 1.0) {
        LogInfo << "Turn sensitivity scale in effect." << VAR(scale);
    }
    const std::lock_guard<std::mutex> guard(g_mutex);
    g_detector.BeginRun();
}

double TurnUnitsScale()
{
    return g_turn_units_scale.load();
}

void RecordTick(MaaContext* context, uint64_t tick_seq, double heading_deg, double issued_delta_deg, bool degraded_fix)
{
    std::optional<Verdict> verdict;
    std::optional<WindowSample> sample;
    {
        const std::lock_guard<std::mutex> guard(g_mutex);
        verdict = g_detector.RecordTick(tick_seq, heading_deg, issued_delta_deg, degraded_fix);
        sample = g_detector.TakeLastWindow();
    }

    if (sample) {
        const double ratio = sample->ratio;
        const double cmd_deg = sample->cmd_deg;
        const int tick_count = sample->tick_count;
        LogInfo << "Turn sensitivity window." << VAR(ratio) << VAR(cmd_deg) << VAR(tick_count);
    }
    if (verdict) {
        PublishVerdict(context, *verdict);
    }
}

void EndRun(MaaContext* context, bool route_failed)
{
    std::optional<Verdict> verdict;
    std::optional<WindowSample> sample;
    {
        const std::lock_guard<std::mutex> guard(g_mutex);
        verdict = g_detector.EndRun(route_failed);
        sample = g_detector.TakeLastWindow();
    }

    if (sample) {
        const double ratio = sample->ratio;
        const double cmd_deg = sample->cmd_deg;
        const int tick_count = sample->tick_count;
        LogInfo << "Turn sensitivity window." << VAR(ratio) << VAR(cmd_deg) << VAR(tick_count);
    }
    if (verdict) {
        PublishVerdict(context, *verdict);
    }
}

} // namespace sensitivity

} // namespace mapnavigator
