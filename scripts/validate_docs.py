#!/usr/bin/env python3
"""Check required relay documentation exists and preserves critical boundaries."""

from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REQUIRED_TEXT = {
    "docs/contracts/relay-tunnel-v1.md": (
        "desktop_hello",
        "mobile_open",
        "duplicate_channel",
        "unknown_locator",
        "not** a proxy to desktop",
        "Agentflow's dedicated",
        "agentflow#422",
        "pheron-app#12",
    ),
    "docs/adr/0001-relay-runtime-and-deployment.md": (
        "Vercel Functions",
        "Long-running service",
        "provider-neutral",
    ),
}
def fail(message: str) -> None:
    raise SystemExit(f"documentation validation failed: {message}")


for relative, needles in REQUIRED_TEXT.items():
    path = ROOT / relative
    if not path.is_file():
        fail(f"missing {relative}")
    text = path.read_text(encoding="utf-8")
    for needle in needles:
        if needle not in text:
            fail(f"{relative} is missing required text: {needle}")
print(f"validated {len(REQUIRED_TEXT)} relay documents")
