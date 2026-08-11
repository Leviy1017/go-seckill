"""连接池探针：检查 MySQL 连接池是否接近上限"""

import os
from typing import Optional

import pymysql

from .base import BaseProbe, Alert


class ConnectionPoolProbe(BaseProbe):
    """监控 MySQL 连接数"""

    def __init__(self):
        self.conn = pymysql.connect(
            host=os.getenv("MYSQL_HOST", "localhost"),
            port=int(os.getenv("MYSQL_PORT", 3306)),
            user=os.getenv("MYSQL_USER", "root"),
            password=os.getenv("MYSQL_PASSWORD", "root"),
            charset="utf8mb4",
        )
        self.threshold = float(os.getenv("CONN_POOL_THRESHOLD", 0.8))

    def name(self) -> str:
        return "connection_pool"

    def interval(self) -> int:
        return int(os.getenv("CHECK_INTERVAL", 30))

    def check(self) -> Optional[Alert]:
        with self.conn.cursor() as cursor:
            # 当前连接数
            cursor.execute("SELECT COUNT(*) FROM information_schema.PROCESSLIST")
            current = cursor.fetchone()[0]

            # 最大连接数
            cursor.execute("SHOW VARIABLES LIKE 'max_connections'")
            max_conn = int(cursor.fetchone()[1])

            # 活跃连接
            cursor.execute(
                "SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE COMMAND != 'Sleep'"
            )
            active = cursor.fetchone()[0]

        usage_ratio = current / max_conn if max_conn > 0 else 0

        if usage_ratio < self.threshold:
            return None

        return Alert(
            level="warning" if usage_ratio < 0.95 else "critical",
            title=f"数据库连接池告警：{current}/{max_conn}（{usage_ratio:.0%}）",
            probe_name=self.name(),
            context={
                "current_connections": current,
                "max_connections": max_conn,
                "usage_ratio": f"{usage_ratio:.1%}",
                "active_connections": active,
                "idle_connections": current - active,
                "threshold": f"{self.threshold:.0%}",
            },
        )
