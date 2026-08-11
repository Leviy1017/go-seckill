"""滑动窗口记忆：记录最近 N 次检查结果，让 Agent 能识别趋势"""

from collections import deque
from datetime import datetime
from typing import List, Dict, Optional


class CheckRecord:
    """单次检查记录"""
    def __init__(self, round_num: int, alerts_count: int, alerts_summary: List[str]):
        self.round = round_num
        self.time = datetime.now().strftime("%H:%M:%S")
        self.alerts_count = alerts_count
        self.alerts_summary = alerts_summary  # 每个 Alert 的简短标题

    def brief(self) -> str:
        if self.alerts_count == 0:
            return f"第 {self.round} 轮 [{self.time}] ✅ 正常"
        return f"第 {self.round} 轮 [{self.time}] ⚠️ {self.alerts_count} 个异常: {'; '.join(self.alerts_summary)}"


class HistoryManager:
    """滑动窗口记忆：只保留最近 window_size 次检查"""

    def __init__(self, window_size: int = 10):
        self.window_size = window_size
        self.records: deque[CheckRecord] = deque(maxlen=window_size)

    def record(self, round_num: int, alerts: list) -> None:
        """记录一次检查结果"""
        summary = []
        for alert in alerts:
            summary.append(alert.title[:40])  # 截短标题
        self.records.append(CheckRecord(round_num, len(alerts), summary))

    def to_context(self) -> str:
        """生成可以喂给 LLM 的历史上下文"""
        if not self.records:
            return "（暂无历史记录）"

        lines = ["## 📜 最近检查历史"]
        for r in self.records:
            lines.append(f"  - {r.brief()}")

        # 趋势分析
        lines.append(self._trend_hint())
        return "\n".join(lines)

    def _trend_hint(self) -> str:
        """自动检测趋势"""
        if len(self.records) < 3:
            return ""

        recent = [r for r in self.records][-5:]  # 最近 5 次
        counts = [r.alerts_count for r in recent]

        if all(c == 0 for c in counts):
            return "  趋势: 最近几轮全部正常，系统处于稳定状态。"
        if counts[-1] > counts[0] > 0:
            return "  趋势: 告警数量呈上升趋势，需要关注。"
        if counts[-1] < counts[0]:
            return "  趋势: 告警数量在下降，问题正在缓解。"
        if max(counts) == counts[-1] and counts[-1] > 0:
            return "  趋势: 本轮告警数为近期最高。"
        return "  趋势: 告警数量相对平稳。"
