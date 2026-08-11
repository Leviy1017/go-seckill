"""限流攻击探针：检查是否有买家被频繁限流"""

import os
from typing import Optional

import redis

from .base import BaseProbe, Alert


class RateLimitProbe(BaseProbe):
    """检测异常高频请求"""

    def __init__(self):
        self.rdb = redis.Redis(
            host=os.getenv("REDIS_HOST", "localhost"),
            port=int(os.getenv("REDIS_PORT", 6379)),
            password=os.getenv("REDIS_PASSWORD") or None,
            db=int(os.getenv("REDIS_DB", 0)),
            decode_responses=True,
        )
        self.abuse_threshold = int(os.getenv("RATE_LIMIT_THRESHOLD", 10))

    def name(self) -> str:
        return "rate_limit_abuse"

    def interval(self) -> int:
        return int(os.getenv("CHECK_INTERVAL", 30))

    def check(self) -> Optional[Alert]:
        # 扫描所有 seckill:rate:* key
        rate_keys = self.rdb.keys("seckill:rate:*")

        abusers = []
        for key in rate_keys:
            count = int(self.rdb.get(key) or 0)
            if count >= self.abuse_threshold:
                buyer_id = key.split(":")[-1]
                abusers.append({
                    "buyer_id": buyer_id,
                    "request_count": count,
                    "key": key,
                })

        if not abusers:
            return None

        top3 = sorted(abusers, key=lambda x: x["request_count"], reverse=True)[:3]

        return Alert(
            level="warning",
            title=f"异常请求检测：{len(abusers)} 个买家被频繁限流",
            probe_name=self.name(),
            context={
                "abuser_count": len(abusers),
                "threshold": f"每秒 > {self.abuse_threshold} 次",
                "top_abusers": [
                    f"买家 {a['buyer_id']}: 每秒 {a['request_count']} 次"
                    for a in top3
                ],
                "may_be_attack": len(abusers) >= 5,
            },
        )
