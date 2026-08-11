"""Redis 库存探针：检查 Redis 里的库存是否和 MySQL 一致"""

import os
from typing import Optional

import redis
import pymysql

from .base import BaseProbe, Alert


class StockProbe(BaseProbe):
    """检查每个 active 商品的 Redis 库存是否异常"""

    def __init__(self):
        self.rdb = redis.Redis(
            host=os.getenv("REDIS_HOST", "localhost"),
            port=int(os.getenv("REDIS_PORT", 6379)),
            password=os.getenv("REDIS_PASSWORD") or None,
            db=int(os.getenv("REDIS_DB", 0)),
            decode_responses=True,
        )
        self.mysql_conn = pymysql.connect(
            host=os.getenv("MYSQL_HOST", "localhost"),
            port=int(os.getenv("MYSQL_PORT", 3306)),
            user=os.getenv("MYSQL_USER", "root"),
            password=os.getenv("MYSQL_PASSWORD", "root"),
            database=os.getenv("MYSQL_DB", "seckill"),
            charset="utf8mb4",
        )

    def name(self) -> str:
        return "redis_stock"

    def interval(self) -> int:
        return int(os.getenv("CHECK_INTERVAL", 30))

    def check(self) -> Optional[Alert]:
        # 1. 查 MySQL 所有 active 商品
        with self.mysql_conn.cursor() as cursor:
            cursor.execute(
                "SELECT product_id, name, stock FROM seckill_products WHERE status = 'active'"
            )
            products = cursor.fetchall()

        anomalies = []
        for product_id, name, mysql_stock in products:
            # 2. 查 Redis 库存
            redis_key = f"seckill:stock:{product_id}"
            redis_val = self.rdb.get(redis_key)

            # Redis key 不存在
            if redis_val is None:
                anomalies.append({
                    "product_id": product_id,
                    "product_name": name,
                    "mysql_stock": mysql_stock,
                    "redis_stock": "key 不存在",
                    "issue": "key 不存在，可能是预热失败或 Redis 重启",
                })
                continue

            redis_stock = int(redis_val)

            # MySQL 和 Redis 库存不一致
            if mysql_stock != redis_stock:
                anomalies.append({
                    "product_id": product_id,
                    "product_name": name,
                    "mysql_stock": mysql_stock,
                    "redis_stock": redis_stock,
                    "issue": f"库存不一致，差异 {abs(mysql_stock - redis_stock)} 件",
                })

            # Redis 库存为 0 但 MySQL 还有
            if redis_stock == 0 and mysql_stock > 0:
                anomalies.append({
                    "product_id": product_id,
                    "product_name": name,
                    "mysql_stock": mysql_stock,
                    "redis_stock": 0,
                    "issue": "Redis 库存零但 MySQL 还有货，买家无法下单",
                })

        if not anomalies:
            return None

        # 3. 收集额外上下文
        redis_info = self.rdb.info("memory")
        return Alert(
            level="critical" if any(a["issue"] and "无法下单" in a["issue"] for a in anomalies) else "warning",
            title=f"Redis 库存异常：{len(anomalies)} 个商品状态不一致",
            probe_name=self.name(),
            context={
                "anomaly_count": len(anomalies),
                "anomalies": anomalies,
                "redis_memory_used": redis_info.get("used_memory_human", "unknown"),
            },
        )
