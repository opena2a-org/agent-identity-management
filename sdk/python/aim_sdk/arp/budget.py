"""
L2 budget control, ported from ``sdk/typescript/src/arp/intelligence/budget.ts``.

A monthly USD budget and an hourly call cap. A call reserves its estimated cost
before it is made, so calls in flight count against both limits; it then
records its actual cost or releases the reservation. State is saved to
``budget.json`` in the data directory, in the TS shape.
"""
from __future__ import annotations

import json
import math
import os
from datetime import datetime, timezone
from typing import Any, Callable, Dict, Optional

from ._reentry import arp_internal
from .types import iso_from_ms, ms_from_iso, now_ms

BUDGET_FILE = "budget.json"
DEFAULT_BUDGET_USD = 5.0
DEFAULT_MAX_CALLS_PER_HOUR = 20


class BudgetReservation:
    __slots__ = ("estimated_cost_usd",)

    def __init__(self, estimated_cost_usd: float):
        self.estimated_cost_usd = estimated_cost_usd


def _fresh_state(now_iso: str) -> Dict[str, Any]:
    return {
        "totalSpentUsd": 0,
        "periodStart": now_iso,
        "totalCalls": 0,
        "failedCalls": 0,
        "lastFailure": None,
        "callsThisHour": 0,
        "hourStart": now_iso,
        "recentCosts": [],
    }


class BudgetController:
    def __init__(
        self,
        data_dir: str,
        budget_usd: Optional[float] = None,
        max_calls_per_hour: Optional[int] = None,
        clock: Optional[Callable[[], float]] = None,
    ):
        self._data_dir = data_dir
        self._clock = clock or now_ms
        self.budget_usd = DEFAULT_BUDGET_USD if budget_usd is None else budget_usd
        self.max_calls_per_hour = (
            DEFAULT_MAX_CALLS_PER_HOUR if max_calls_per_hour is None else max_calls_per_hour
        )
        self._holds: Dict[BudgetReservation, int] = {}
        self._hour_window = 0
        self._state = self._load_state()

    # ------------------------------------------------------------------ #
    def can_reserve(self, estimated_cost_usd: Any) -> bool:
        if isinstance(estimated_cost_usd, bool) or not isinstance(estimated_cost_usd, (int, float)):
            return False
        if not estimated_cost_usd >= 0:  # NaN and negatives
            return False
        self._rollover_if_needed()
        self._rollover_hour_if_needed()
        if self._state["totalSpentUsd"] + self._held_usd() + estimated_cost_usd > self.budget_usd:
            return False
        return self._state["callsThisHour"] < self.max_calls_per_hour

    def reserve(self, estimated_cost_usd: Any) -> Optional[BudgetReservation]:
        if not self.can_reserve(estimated_cost_usd):
            return None
        reservation = BudgetReservation(estimated_cost_usd)
        self._holds[reservation] = self._hour_window
        self._state["callsThisHour"] += 1
        try:
            self._save_state()
        except Exception:
            del self._holds[reservation]
            self._state["callsThisHour"] -= 1
            raise
        return reservation

    def record(self, reservation: BudgetReservation, cost_usd: Any, tokens: int) -> None:
        self._close(reservation, "record")
        self._rollover_if_needed()
        self._rollover_hour_if_needed()
        ok = (
            isinstance(cost_usd, (int, float))
            and not isinstance(cost_usd, bool)
            and math.isfinite(cost_usd)
            and cost_usd >= 0
        )
        charged = cost_usd if ok else reservation.estimated_cost_usd
        self._state["totalSpentUsd"] += charged
        self._state["totalCalls"] += 1
        self._state["recentCosts"].append(
            {"timestamp": iso_from_ms(self._clock()), "cost": charged, "tokens": tokens}
        )
        self._state["recentCosts"] = self._state["recentCosts"][-100:]
        self._save_state()

    def release(self, reservation: BudgetReservation) -> None:
        window = self._close(reservation, "release")
        self._rollover_if_needed()
        self._rollover_hour_if_needed()
        if window == self._hour_window:
            self._state["callsThisHour"] -= 1
        self._save_state()

    def record_failure(self, adapter: str, message: str) -> None:
        self._rollover_if_needed()
        self._rollover_hour_if_needed()
        self._state["failedCalls"] += 1
        self._state["lastFailure"] = {
            "at": iso_from_ms(self._clock()), "adapter": adapter, "message": message,
        }
        self._save_state()

    def get_status(self) -> Dict[str, Any]:
        self._rollover_if_needed()
        self._rollover_hour_if_needed()
        spent = self._state["totalSpentUsd"] + self._held_usd()
        last = self._state["lastFailure"]
        return {
            "spent": round(spent * 10000) / 10000,
            "budget": self.budget_usd,
            "remaining": round((self.budget_usd - spent) * 10000) / 10000,
            "percentUsed": round((spent / self.budget_usd) * 100) if self.budget_usd else 0,
            "callsThisHour": self._state["callsThisHour"],
            "maxCallsPerHour": self.max_calls_per_hour,
            "totalCalls": self._state["totalCalls"],
            "failedCalls": self._state["failedCalls"],
            "lastFailure": dict(last) if last else None,
        }

    def reset(self) -> None:
        self._state = _fresh_state(iso_from_ms(self._clock()))
        self._hour_window += 1
        self._save_state()

    # ------------------------------------------------------------------ #
    def _held_usd(self) -> float:
        return sum(r.estimated_cost_usd for r in self._holds)

    def _close(self, reservation: BudgetReservation, method: str) -> int:
        if reservation not in self._holds:
            raise ValueError(
                "BudgetController.%s() needs an open reservation from reserve() on this "
                "controller; this one was already recorded or released, or did not come "
                "from reserve()" % method
            )
        return self._holds.pop(reservation)

    def _rollover_if_needed(self) -> None:
        start = ms_from_iso(self._state["periodStart"])
        now = self._clock()
        if math.isnan(start):
            start = now
        a = datetime.fromtimestamp(start / 1000.0, tz=timezone.utc)
        b = datetime.fromtimestamp(now / 1000.0, tz=timezone.utc)
        if (a.year, a.month) != (b.year, b.month):
            self._state = _fresh_state(iso_from_ms(now))
            self._hour_window += 1
            self._save_state()

    def _rollover_hour_if_needed(self) -> None:
        start = ms_from_iso(self._state["hourStart"])
        now = self._clock()
        if math.isnan(start) or now - start > 3600000:
            self._state["callsThisHour"] = 0
            self._state["hourStart"] = iso_from_ms(now)
            self._hour_window += 1

    def _load_state(self) -> Dict[str, Any]:
        path = os.path.join(self._data_dir, BUDGET_FILE)
        try:
            with arp_internal():
                if os.path.exists(path):
                    with open(path, "r", encoding="utf-8") as fh:
                        saved = json.load(fh)
                    state = _fresh_state(iso_from_ms(self._clock()))
                    state.update(saved)
                    state["failedCalls"] = saved.get("failedCalls") or 0
                    state["lastFailure"] = saved.get("lastFailure")
                    return state
        except (OSError, ValueError, TypeError, AttributeError):
            pass
        return _fresh_state(iso_from_ms(self._clock()))

    def _save_state(self) -> None:
        with arp_internal():
            os.makedirs(self._data_dir, exist_ok=True)
            persisted = dict(self._state)
            # Estimates in flight are saved as spent, so a crash never forgets them.
            persisted["totalSpentUsd"] = self._state["totalSpentUsd"] + self._held_usd()
            with open(os.path.join(self._data_dir, BUDGET_FILE), "w", encoding="utf-8") as fh:
                json.dump(persisted, fh, indent=2)
