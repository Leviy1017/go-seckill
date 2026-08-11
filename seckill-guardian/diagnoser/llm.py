"""LLM 诊断模块：链式推理 + 记忆上下文，让 Agent 像人一样思考"""

import os
import json
from typing import List

from openai import OpenAI

from probe.base import Alert

# ── 链式推理各步骤的提示词 ──────────────────────────────

STEP1_SYSTEM = """你是秒杀系统运维专家。你的任务是：根据给定的告警列表和历史趋势，做紧急度排序。
不要分析原因，不要给建议。只做排序。

输出格式（严格按此格式）：
1. [严重程度] 告警标题 — 为什么紧急（一句话）
2. [严重程度] 告警标题 — 为什么紧急（一句话）
...

注意：critical > warning；同级别的，影响面大的排前面。"""

STEP2_SYSTEM = """你是秒杀系统运维专家。上一步已经做了紧急度排序，现在请分析每个告警的可能原因。

对每个告警，给出：
- 最可能的原因（1-2个）
- 可能性判断（高/中/低）
- 原因的解释（为什么）

输出格式：
## 告警1：标题
- 可能原因A（高）: ...
- 可能原因B（中）: ...

## 告警2：标题
..."""

STEP3_SYSTEM = """你是秒杀系统运维专家。上一步已经分析了可能原因，现在请为每个原因设计验证步骤。

对每个原因，给出：
- 怎么验证（具体的命令/查询/日志关键词）
- 什么结果算确认、什么结果算排除

输出格式：
## 验证告警1
### 原因A
- 验证方法: ...
- 确认条件: ...
- 排除条件: ..."""

STEP4_SYSTEM = """你是秒杀系统运维专家。现在请综合前面的紧急度排序、原因分析和验证计划，产出一份最终诊断报告。

输出格式：
## 🔴 紧急问题
## 🟡 需关注
## 📊 系统全景（结合历史趋势做判断）
## 🔗 问题关联（如果多个告警之间有因果关系，请指出）
## ✅ 验证计划（优先验证什么）
## 💡 综合建议"""


class LLMDiagnoser:
    """链式推理诊断器：分 4 步思考，每一步产出都喂给下一步"""

    def __init__(self):
        self.client = OpenAI(
            api_key=os.getenv("OPENAI_API_KEY", ""),
            base_url=os.getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
        )
        self.model = os.getenv("OPENAI_MODEL", "gpt-4o-mini")

    # ── 公开方法 ──────────────────────────────────────

    def diagnose(self, alerts: List[Alert], history_context: str = "") -> str:
        """链式推理诊断入口"""
        if not alerts:
            return "✅ 一切正常，未发现异常。"

        alerts_data = self._format_alerts(alerts)
        prompt_prefix = f"{history_context}\n\n{alerts_data}"

        try:
            # 第 1 步：紧急度排序
            step1 = self._call_llm(STEP1_SYSTEM, f"{prompt_prefix}\n\n请对以上告警做紧急度排序。")

            # 第 2 步：原因分析
            step2 = self._call_llm(
                STEP2_SYSTEM,
                f"{alerts_data}\n\n以下是我初步的紧急度判断：\n{step1}\n\n请分析每个告警的可能原因。",
            )

            # 第 3 步：验证计划
            step3 = self._call_llm(
                STEP3_SYSTEM,
                f"{alerts_data}\n\n原因分析：\n{step2}\n\n请为每个原因设计验证步骤。",
            )

            # 第 4 步：综合报告
            step4 = self._call_llm(
                STEP4_SYSTEM,
                f"{history_context}\n\n{alerts_data}\n\n"
                f"紧急度排序：\n{step1}\n\n原因分析：\n{step2}\n\n验证计划：\n{step3}\n\n"
                f"请产出一份最终诊断报告。",
            )

            # 返回完整思考过程 + 最终报告
            return self._format_output(step1, step2, step3, step4)

        except Exception as e:
            return f"⚠️ LLM 调用失败（{e}）\n\n原始告警数据：\n{alerts_data}"

    # ── 内部方法 ──────────────────────────────────────

    def _call_llm(self, system_prompt: str, user_prompt: str) -> str:
        """调一次 LLM，返回文本"""
        response = self.client.chat.completions.create(
            model=self.model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            temperature=0.3,
            max_tokens=1500,
        )
        return response.choices[0].message.content.strip()

    def _format_alerts(self, alerts: List[Alert]) -> str:
        """把 Alert 列表转成结构化文本"""
        parts = ["以下是秒杀系统监控发现的异常：\n"]

        snapshot = None
        for i, alert in enumerate(alerts, 1):
            ctx = dict(alert.context)  # 拷贝，不修改原数据
            if snapshot is None:
                snapshot = ctx.pop("_system_snapshot", None)

            parts.append(f"--- 告警 {i} ---")
            parts.append(f"探针: {alert.probe_name} | 级别: {alert.level}")
            parts.append(f"标题: {alert.title}")
            parts.append("详情:")
            for k, v in ctx.items():
                if isinstance(v, (list, dict)):
                    parts.append(f"  {k}: {json.dumps(v, ensure_ascii=False, default=str)}")
                else:
                    parts.append(f"  {k}: {v}")
            parts.append("")

        if snapshot:
            parts.insert(1, f"系统快照: {json.dumps(snapshot, ensure_ascii=False, default=str)}\n")

        return "\n".join(parts)

    def _format_output(self, step1: str, step2: str, step3: str, step4: str) -> str:
        """把四个步骤拼成完整的思考链 + 最终报告"""
        return (
            f"{'─' * 50}\n"
            f"🧠 思考过程\n"
            f"{'─' * 50}\n\n"
            f"【步骤 1/4 — 紧急度排序】\n{step1}\n\n"
            f"【步骤 2/4 — 原因分析】\n{step2}\n\n"
            f"【步骤 3/4 — 验证计划】\n{step3}\n\n"
            f"{'─' * 50}\n"
            f"📋 最终诊断\n"
            f"{'─' * 50}\n\n"
            f"{step4}"
        )
