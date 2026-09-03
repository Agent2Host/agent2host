#!/usr/bin/env python3
"""NDJSON MCP git-tools for dev-studio."""
from __future__ import annotations

import json
import sys


def respond(msg_id, result=None, error=None):
    out = {"jsonrpc": "2.0", "id": msg_id}
    out["error" if error else "result"] = error if error else result
    sys.stdout.write(json.dumps(out) + "\n")
    sys.stdout.flush()


TOOLS = [
    {"name": "branch_status", "description": "Current branch and ahead/behind.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
    {"name": "suggest_commit_message", "description": "Conventional commit suggestion from diff summary.", "inputSchema": {"type": "object", "properties": {"scope": {"type": "string"}}, "additionalProperties": False}},
    {"name": "list_staged_files", "description": "Files in the index.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
    {"name": "tag_release_candidate", "description": "Fixture tag plan (does not run git).", "inputSchema": {"type": "object", "properties": {"version": {"type": "string"}}, "required": ["version"], "additionalProperties": False}},
]


def call_tool(name: str, arguments: dict) -> str:
    if name == "branch_status":
        return json.dumps({"branch": "feat/dev-studio-corpus", "ahead": 2, "behind": 0, "dirty": True})
    if name == "suggest_commit_message":
        scope = (arguments or {}).get("scope", "test")
        return json.dumps({"message": f"feat({scope}): add dev-studio integration corpus"})
    if name == "list_staged_files":
        return json.dumps({"staged": ["test/systems/dev-studio/system.json"]})
    if name == "tag_release_candidate":
        ver = (arguments or {}).get("version", "0.0.0")
        return json.dumps({"tag": f"v{ver}-rc1", "status": "planned"})
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
            respond(msg_id, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "serverInfo": {"name": "git-tools", "version": "0.1.0"}})
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
