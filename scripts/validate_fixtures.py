#!/usr/bin/env python3
"""Validate the public relay-tunnel v1 fixture shape without logging contents."""

from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
FIXTURES = ROOT / "fixtures" / "relay-tunnel-v1"
REQUIRED = {
    "valid-handshake.json": ("valid_handshake", {"desktop_reply", "mobile_reply"}),
    "invalid-handshake.json": ("invalid_handshake", {"error", "retryable"}),
    "unknown-locator.json": ("unknown_locator", {"error", "stream_created"}),
    "duplicate-channel.json": ("duplicate_channel", {"error", "existing_channel_preserved"}),
    "expiry.json": ("expiry", {"routing_entry_removed", "active_stream_action"}),
    "disconnect.json": ("disconnect", {"mobile_action", "payload_logged"}),
}


def fail(message: str) -> None:
    raise SystemExit(f"fixture validation failed: {message}")


for filename, (scenario, expected_keys) in REQUIRED.items():
    path = FIXTURES / filename
    if not path.is_file():
        fail(f"missing {path.relative_to(ROOT)}")
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as error:
        fail(f"{path.name}: invalid JSON ({error.msg})")
    if value.get("version") != 1:
        fail(f"{path.name}: version must be 1")
    if value.get("scenario") != scenario:
        fail(f"{path.name}: scenario must be {scenario}")
    expected = value.get("expected")
    if not isinstance(expected, dict) or not expected_keys <= expected.keys():
        fail(f"{path.name}: expected result is incomplete")

actual = {path.name for path in FIXTURES.glob("*.json")}
if actual != set(REQUIRED):
    fail("fixture set does not exactly match the v1 contract")

print(f"validated {len(REQUIRED)} relay tunnel fixtures")
