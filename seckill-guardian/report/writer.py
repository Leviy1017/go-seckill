"""报告输出：把 LLM 诊断结果格式化打印"""

from datetime import datetime


def print_report(diagnosis: str, alert_count: int, run_count: int = 0):
    """终端输出一份完整的诊断报告"""
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    # 分隔线
    print()
    print("=" * 60)
    print(f"  Seckill Guardian — 诊断报告 #{run_count}")
    print(f"  时间: {now}")
    print(f"  LLM 分析结果如下：")
    print("=" * 60)
    print()

    print(diagnosis)

    print()
    print("-" * 60)
    if alert_count == 0:
        print("  ✅ 本次检查未发现异常，系统运行正常。")
    else:
        print(f"  📋 本次共发现 {alert_count} 个异常，详情见上方 LLM 分析。")
    print("-" * 60)
    print()
