"""
L1 statistical anomaly detection, ported from
``sdk/typescript/src/arp/intelligence/anomaly.ts``.

Tracks events per minute per monitor source and scores the current minute's
count as a z-score against the learned per-minute baseline.
"""
from __future__ import annotations

import json
import math
import os
from typing import Callable, Dict, List, Optional

from ._reentry import arp_internal
from .types import ARPEvent, now_ms

MAX_HISTORY_MINUTES = 10080  # 7 days at 1-minute resolution
MIN_DATA_POINTS = 30


class AnomalyDetector:
    def __init__(self, clock: Optional[Callable[[], float]] = None):
        self._clock = clock or now_ms
        self._series: Dict[str, List[int]] = {}
        self._baselines: Dict[str, Dict[str, float]] = {}

    def _minute(self) -> int:
        return int(math.floor(self._clock() / 60000))

    def score(self, event: ARPEvent) -> float:
        """Z-score of this minute's event count for ``event.source`` (0 = normal, >2 = anomalous)."""
        current_minute = self._minute()
        series = self._series.get(event.source, [])
        current_count = sum(1 for m in series if m == current_minute) + 1

        baseline = self._baselines.get(event.source)
        if not baseline or baseline["count"] < MIN_DATA_POINTS:
            return 0.0
        if baseline["stddev"] == 0:
            return 2.5 if current_count > baseline["mean"] else 0.0
        return abs(current_count - baseline["mean"]) / baseline["stddev"]

    def record(self, event: ARPEvent) -> None:
        current_minute = self._minute()
        series = self._series.setdefault(event.source, [])
        series.append(current_minute)

        cutoff = current_minute - MAX_HISTORY_MINUTES
        trim = next((i for i, m in enumerate(series) if m >= cutoff), -1)
        if trim > 0:
            del series[:trim]
        self._update_baseline(event.source, series)

    def get_baseline(self, source: str) -> Optional[Dict[str, float]]:
        baseline = self._baselines.get(source)
        return dict(baseline) if baseline else None

    def reset(self) -> None:
        self._series.clear()
        self._baselines.clear()

    def save(self, data_dir: str) -> None:
        """Persist in the TS ``anomaly-baselines.json`` shape."""
        payload = {
            "timeSeries": {
                k: [{"timestamp": m, "value": 1} for m in v] for k, v in self._series.items()
            },
            "baselines": self._baselines,
        }
        with arp_internal():
            os.makedirs(data_dir, exist_ok=True)
            with open(os.path.join(data_dir, "anomaly-baselines.json"), "w", encoding="utf-8") as fh:
                json.dump(payload, fh, indent=2)

    @classmethod
    def load(cls, data_dir: str, clock: Optional[Callable[[], float]] = None) -> "AnomalyDetector":
        detector = cls(clock=clock)
        path = os.path.join(data_dir, "anomaly-baselines.json")
        try:
            with arp_internal():
                with open(path, "r", encoding="utf-8") as fh:
                    parsed = json.load(fh)
            for key, points in (parsed.get("timeSeries") or {}).items():
                detector._series[key] = [int(p["timestamp"]) for p in points]
            for key, value in (parsed.get("baselines") or {}).items():
                detector._baselines[key] = {
                    "mean": float(value["mean"]),
                    "stddev": float(value["stddev"]),
                    "count": int(value["count"]),
                }
        except (OSError, ValueError, TypeError, KeyError, AttributeError):
            return cls(clock=clock)  # missing or corrupted: start fresh
        return detector

    def _update_baseline(self, source: str, series: List[int]) -> None:
        minute_counts: Dict[int, int] = {}
        for m in series:
            minute_counts[m] = minute_counts.get(m, 0) + 1
        values = list(minute_counts.values())
        if not values:
            return
        count = len(values)
        mean = sum(values) / count
        variance = sum((v - mean) ** 2 for v in values) / count
        self._baselines[source] = {"mean": mean, "stddev": math.sqrt(variance), "count": count}
