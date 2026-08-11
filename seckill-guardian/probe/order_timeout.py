"""超时订单探针：检查是否有 paid 状态超过阈值的订单"""

import os
from typing import Optional

import pymysql

from .base import BaseProbe, Alert


class OrderTimeoutProbe(BaseProbe):
    """检查未处理的超时订单"""

    def __init__(self):
        self.conn = pymysql.connect(
            host=os.getenv("MYSQL_HOST", "localhost"),
            port=int(os.getenv("MYSQL_PORT", 3306)),
            user=os.getenv("MYSQL_USER", "root"),
            password=os.getenv("MYSQL_PASSWORD", "root"),
            database=os.getenv("MYSQL_DB", "seckill"),
            charset="utf8mb4",
        )
        self.timeout_minutes = int(os.getenv("ORDER_TIMEOUT_MINUTES", 30))

    def name(self) -> str:
        return "order_timeout"

    def interval(self) -> int:
        return int(os.getenv("CHECK_INTERVAL", 30))

    def check(self) -> Optional[Alert]:
        with self.conn.cursor() as cursor:
            cursor.execute(
                """SELECT order_id, buyer_name, seller_name, product_name,
                          seckill_price, order_time,
                          TIMESTAMPDIFF(MINUTE, order_time, NOW()) AS waiting_minutes
                   FROM seckill_orders
                   WHERE order_status = 'paid'
                     AND order_time < DATE_SUB(NOW(), INTERVAL %s MINUTE)
                   ORDER BY order_time ASC""",
                (self.timeout_minutes,),
            )
            orders = cursor.fetchall()

        if not orders:
            return None

        overdue_list = [
            {
                "order_id": o[0],
                "buyer": o[1],
                "seller": o[2],
                "product": o[3],
                "price": float(o[4]),
                "order_time": str(o[5]),
                "waiting_minutes": o[6],
            }
            for o in orders
        ]

        # 找到等最久的那个
        worst = overdue_list[0]

        return Alert(
            level="warning",
            title=f"超时订单：{len(orders)} 笔订单超过 {self.timeout_minutes} 分钟未被接单",
            probe_name=self.name(),
            context={
                "overdue_count": len(orders),
                "threshold_minutes": self.timeout_minutes,
                "longest_wait": f"{worst['waiting_minutes']} 分钟",
                "longest_order": f"买家 {worst['buyer']} 抢了 {worst['product']}，卖家 {worst['seller']} 还未处理",
                "overdue_orders": overdue_list,
            },
        )
