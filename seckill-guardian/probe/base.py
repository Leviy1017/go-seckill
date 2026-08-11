"""探针基类：所有探针都实现这个接口"""

from abc import ABC, abstractmethod
from typing import Optional
from dataclasses import dataclass, field

#@dataclass 是 Python 3.7 引入的装饰器，自动帮你生成 __init__、__repr__、__eq__ 等方法。
@dataclass
class Alert:
    """一条告警：探针发现异常时产出"""
    level: str                     # "warning" 或 "critical"
    title: str                     # 告警标题，如 "Redis 库存丢失"
    probe_name: str                # 来源探针
    context: dict = field(default_factory=dict)  # 收集到的上下文数据

#@abstractmethod：标记一个方法是"抽象方法"。意思是：我只规定了方法签名，不写实现，子类必须
#自己实现这个方法，否则实例化时 Python 会报错
class BaseProbe(ABC):
    """探针基类"""

    @abstractmethod 
    def name(self) -> str:
        """探针名字，如 'redis_stock'"""
        ...

    @abstractmethod
    def interval(self) -> int:
        """检查间隔（秒）"""
        ...

    @abstractmethod
    def check(self) -> Optional[Alert]:
        """执行检查。正常返回 None，异常返回 Alert"""
        ...
