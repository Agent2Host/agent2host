#!/usr/bin/env python3
"""Minimal NDJSON stdio MCP server for dev-studio repo-tools."""
from __future__ import annotations

import json
import sys


def respond(msg_id, result=None, error=None):
    out = {"jsonrpc": "2.0", "id": msg_id}
    if error is not None:
        out["error"] = error
    else:
        out["result"] = result
    sys.stdout.write(json.dumps(out) + "\n")
    sys.stdout.flush()


TOOLS = [
    {"name": "list_changed_files", "description": "List changed paths.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
    {"name": "get_diff_summary", "description": "Summarize diff for a path.", "inputSchema": {"type": "object", "properties": {"path": {"type": "string"}}, "additionalProperties": False}},
    {"name": "read_file_hunk", "description": "Return a fixture hunk for a path.", "inputSchema": {"type": "object", "properties": {"path": {"type": "string"}}, "required": ["path"], "additionalProperties": False}},
    {"name": "forbidden_echo", "description": "Off allowlist isolation probe.", "inputSchema": {"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"], "additionalProperties": False}},
]


def call_tool(name: str, arguments: dict) -> str:
    if name == "list_changed_files":
        return json.dumps({"files": ["internal/adapter/pipeline.go", "test/systems/dev-studio/system.json", "docs/how-a2h-works.md"]})
    if name == "get_diff_summary":
        path = (arguments or {}).get("path", "(all)")
        return json.dumps({"path": path, "summary": "Fixture: dev-studio corpus added; pipeline unchanged.", "risk": "low"})
    if name == "read_file_hunk":
        path = (arguments or {}).get("path", "")
        return json.dumps({"path": path, "hunk": "+ // dev-studio integration corpus\n"})
    if name == "forbidden_echo":
        return "FORBIDDEN:" + str((arguments or {}).get("text", ""))
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
            respond(msg_id, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "serverInfo": {"name": "repo-tools", "version": "0.1.0"}})
        elif method == "notifications/initialized":
            continue
        elif method == "tools/list":
            respond(msg_id, {"tools": TOOLS})
        elif method == "tools/call":
            params = req.get("params") or {}
            text = call_tool(params.get("name"), params.get("arguments") or {})
            respond(msg_id, {"content": [{"type": "text", "text": text}]})
        elif method == "ping":
            respond(msg_id, {})
        elif msg_id is not None:
            respond(msg_id, error={"code": -32601, "message": f"unknown method {method}"})


if __name__ == "__main__":
    main()
