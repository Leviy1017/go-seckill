"""
Seckill Guardian — 秒杀系统智能运维 Agent

核心设计：
  1. 探针（Probe）      — 眼睛：定时检查 Redis/MySQL，发现异常
  2. 记忆（Memory）      — 经验：滑动窗口记录历史，识别趋势
  3. 诊断（Diagnoser）   — 大脑：链式推理（紧急度排序 → 原因分析 → 验证计划 → 综合报告）
  4. 报告（Report）      — 嘴巴：格式化输出

Agent 只读不写——查 Redis 用 GET，查 MySQL 用 SELECT，不做任何修改操作。
"""

import os
import time
from typing import List

#环境变量注入
from dotenv import load_dotenv

from probe.base import Alert
from probe.stock import StockProbe
from probe.order_timeout import OrderTimeoutProbe
from probe.connection_pool import ConnectionPoolProbe
from probe.rate_limit import RateLimitProbe

from collector import ContextCollector
from memory import HistoryManager
from diagnoser import LLMDiagnoser
from report import print_report


def main():
    # 1. 加载配置
    load_dotenv("config.env")

    # 2. 注册探针
    probes = [
        #StockProbe() 就是"造一个库存探针对象出来"，OrderTimeoutProbe()就是"造一个超时订单探针对象出来"。
        StockProbe(),
        OrderTimeoutProbe(),
        ConnectionPoolProbe(),
        RateLimitProbe(),
    ]
    print(f"[Guardian] 已注册 {len(probes)} 个探针：")
    for p in probes:
        print(f"  - {p.name()} (每 {p.interval()}s)")

    # 3. 核心组件
    collector = ContextCollector()
    history = HistoryManager(window_size=10)
    diagnoser = LLMDiagnoser()

    print(f"[Guardian] LLM 模型: {diagnoser.model}")
    print(f"[Guardian] 记忆窗口: 最近 {history.window_size} 轮")
    print(f"[Guardian] 推理模式: 链式 4 步（排序 → 归因 → 验证 → 报告）")
    print(f"[Guardian] 开始监控，按 Ctrl+C 停止\n")

    # 4. 初始检查
    run_check(probes, collector, history, diagnoser, run_count=1)

    # 5. 定时调度
    run_count = 1
    try:
        while True:
            time.sleep(int(os.getenv("CHECK_INTERVAL", 30)))
            run_count += 1
            run_check(probes, collector, history, diagnoser, run_count)
    except KeyboardInterrupt:
        print("\n[Guardian] 已停止。")


def run_check(probes, collector, history, diagnoser, run_count):
    """一次完整检查：跑探针 → 收集上下文 → 查历史 → LLM 链式推理 → 记录历史 → 输出报告"""
    from datetime import datetime
    now = datetime.now().strftime("%H:%M:%S")

    # 1. 跑所有探针
    alerts: List[Alert] = []
    for probe in probes:
        try:
            result = probe.check()
            if result:
                alerts.append(result)
        except Exception as e:
            print(f"[{now}] ⚠️ 探针 {probe.name()} 执行失败: {e}")

    # 2. 没异常就安静
    if not alerts:
        print(f"[{now}] ✅ 一切正常")
        history.record(run_count, [])
        return

    # 3. 收集全局上下文
    alerts = collector.enrich(alerts)

    # 4. 获取历史记忆
    history_context = history.to_context()

    # 5. LLM 链式推理诊断
    print(f"[{now}] 🔍 发现 {len(alerts)} 个异常，LLM 正在链式推理...")
    diagnosis = diagnoser.diagnose(alerts, history_context)

    # 6. 记录这次检查到历史
    history.record(run_count, alerts)

    # 7. 输出报告
    print_report(diagnosis, len(alerts), run_count)


if __name__ == "__main__":
    main()
