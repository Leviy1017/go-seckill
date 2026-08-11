"""上下文收集器：把探针产出的 Alert 和系统实时状态合并"""

import os
from typing import List

import redis
import pymysql

from probe.base import Alert


class ContextCollector:
    """收集额外的上下文数据，补充到 Alert 的 context 里"""

    def __init__(self):
        self.rdb = redis.Redis(
            host=os.getenv("REDIS_HOST", "localhost"),
            port=int(os.getenv("REDIS_PORT", 6379)),
            password=os.getenv("REDIS_PASSWORD") or None,
            db=int(os.getenv("REDIS_DB", 0)),
            decode_responses=True,
        )
        self.mysql = pymysql.connect(
            host=os.getenv("MYSQL_HOST", "localhost"),
            port=int(os.getenv("MYSQL_PORT", 3306)),
            user=os.getenv("MYSQL_USER", "root"),
            password=os.getenv("MYSQL_PASSWORD", "root"),
            database=os.getenv("MYSQL_DB", "seckill"),
            charset="utf8mb4",
        )

    def enrich(self, alerts: List[Alert]) -> List[Alert]:
        """给所有 Alert 补充系统全局上下文"""
        if not alerts:
            return alerts

        # 全局指标（每个探针的 Alert 都带一份）
        global_ctx = {}
        try:
            info = self.rdb.info("stats")
            global_ctx["redis_ops_per_sec"] = info.get("instantaneous_ops_per_sec", 0)
            global_ctx["redis_connected_clients"] = info.get("connected_clients", 0)
        except Exception:
            pass

        try:
            with self.mysql.cursor() as cur:
                # 秒杀商品总数
                cur.execute("SELECT COUNT(*) FROM seckill_products")
                global_ctx["total_products"] = cur.fetchone()[0]

                # active 状态的商品
                cur.execute("SELECT COUNT(*) FROM seckill_products WHERE status='active'")
                global_ctx["active_products"] = cur.fetchone()[0]

                # 最近 5 分钟订单数
                cur.execute(
                    "SELECT COUNT(*) FROM seckill_orders WHERE order_time > DATE_SUB(NOW(), INTERVAL 5 MINUTE)"
                )
                global_ctx["orders_last_5min"] = cur.fetchone()[0]

                # 各状态订单分布
                cur.execute(
                    "SELECT order_status, COUNT(*) FROM seckill_orders GROUP BY order_status"
                )
                global_ctx["order_status_distribution"] = dict(cur.fetchall())
        except Exception:
            pass

        # 注入到每个 Alert
        for alert in alerts:
            alert.context["_system_snapshot"] = global_ctx

        return alerts
