#!/usr/bin/env python3
"""NDJSON MCP ci-tools for dev-studio."""
from __future__ import annotations

import json
import sys


def respond(msg_id, result=None, error=None):
    out = {"jsonrpc": "2.0", "id": msg_id}
    out["error" if error else "result"] = error if error else result
    sys.stdout.write(json.dumps(out) + "\n")
    sys.stdout.flush()


TOOLS = [
    {"name": "pipeline_status", "description": "CI pipeline state fixture.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
    {"name": "get_lint_summary", "description": "Linter output summary.", "inputSchema": {"type": "object", "properties": {"path": {"type": "string"}}, "additionalProperties": False}},
    {"name": "plan_test_matrix", "description": "Packages to test for a change set.", "inputSchema": {"type": "object", "properties": {"packages": {"type": "array", "items": {"type": "string"}}}, "additionalProperties": False}},
]


def call_tool(name: str, arguments: dict) -> str:
    if name == "pipeline_status":
        return json.dumps({"status": "green", "last_run": "2026-08-31T12:00:00Z", "canary": "CI_CANARY_OK"})
    if name == "get_lint_summary":
        path = (arguments or {}).get("path", ".")
        return json.dumps({"path": path, "errors": 0, "warnings": 1, "notes": ["go vet clean on adapter"]})
    if name == "plan_test_matrix":
        pkgs = (arguments or {}).get("packages") or ["./internal/adapter/...", "./internal/cli/..."]
        return json.dumps({"packages": pkgs, "command": "go test ./..."})
    return f"unknown tool {name}"


def main() -> None:
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
        except json.JSONDecodeError:
            continue
        method, msg_id = req.get("method"), req.get("id")
        if method == "initialize":
            respond(msg_id, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "serverInfo": {"name": "ci-tools", "version": "0.1.0"}})
        elif method == "notifications/initialized":
            continue
        elif method == "tools/list":
            respond(msg_id, {"tools": TOOLS})
        elif method == "tools/call":
            params = req.get("params") or {}
            text = call_tool(params.get("name"), params.get("arguments") or {})
            respond(msg_id, {"content": [{"type": "text", "text": text}]})
        elif msg_id is not None:
            respond(msg_id, error={"code": -32601, "message": "not found"})


if __name__ == "__main__":
    main()
