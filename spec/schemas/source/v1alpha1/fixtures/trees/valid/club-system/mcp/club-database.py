#!/usr/bin/env python3
"""Minimal stdio MCP server for the club-database fixture.

Supports both MCP Content-Length framing and newline-delimited JSON
(some Host clients use NDJSON). Always flush; unbuffered-friendly.
"""

from __future__ import annotations

import json
import sys

POLICY_TEXT = "Guest policy: members only after 18:00."
POLICY_CANARY = "POLICY_CANARY_OK"

TOOLS = [
    {
        "name": "search_policy",
        "description": "Search club policy documents.",
        "inputSchema": {
            "type": "object",
            "properties": {"query": {"type": "string"}},
            "additionalProperties": True,
        },
    },
    {
        "name": "get_member",
        "description": "Verify membership status.",
        "inputSchema": {
            "type": "object",
            "properties": {"member_id": {"type": "string"}},
            "additionalProperties": True,
        },
    },
]

# "content-length" | "ndjson" — match whatever the client used last.
_framing = "content-length"


def send(msg: dict) -> None:
    body = json.dumps(msg, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    if _framing == "ndjson":
        sys.stdout.buffer.write(body + b"\n")
    else:
        sys.stdout.buffer.write(f"Content-Length: {len(body)}\r\n\r\n".encode("ascii"))
        sys.stdout.buffer.write(body)
    sys.stdout.buffer.flush()


def read_msg() -> dict | None:
    global _framing
    first = sys.stdin.buffer.readline()
    if not first:
        return None
    # NDJSON: first line is the JSON object.
    stripped = first.lstrip()
    if stripped.startswith(b"{") or stripped.startswith(b"["):
        _framing = "ndjson"
        return json.loads(stripped.decode("utf-8"))
    # Content-Length framing (LSP / MCP stdio).
    _framing = "content-length"
    headers: dict[str, str] = {}
    line = first.decode("ascii", errors="replace").strip()
    while True:
        if not line:
            break
        key, _, value = line.partition(":")
        headers[key.lower()] = value.strip()
        next_line = sys.stdin.buffer.readline()
        if not next_line:
            break
        line = next_line.decode("ascii", errors="replace").strip()
    length = int(headers.get("content-length", "0") or "0")
    if length <= 0:
        return None
    body = sys.stdin.buffer.read(length)
    if len(body) < length:
        return None
    return json.loads(body.decode("utf-8"))


def tool_result(text: str) -> dict:
    return {"content": [{"type": "text", "text": text}], "isError": False}


def handle_call(name: str, arguments: dict) -> dict:
    if name == "search_policy":
        query = (arguments or {}).get("query", "")
        return tool_result(f"{POLICY_TEXT} {POLICY_CANARY} query={query!r}")
    if name == "get_member":
        member_id = (arguments or {}).get("member_id", "")
        return tool_result(f"member_id={member_id!r} status=active")
    raise ValueError(f"unknown tool: {name}")


def handle(req: dict) -> None:
    method = req.get("method")
    req_id = req.get("id")
    params = req.get("params") or {}

    if method == "initialize":
        client_version = params.get("protocolVersion") or "2024-11-05"
        send(
            {
                "jsonrpc": "2.0",
                "id": req_id,
                "result": {
                    "protocolVersion": client_version,
                    "capabilities": {"tools": {"listChanged": False}},
                    "serverInfo": {"name": "club-database", "version": "1.0.0"},
                },
            }
        )
        return

    if method in ("notifications/initialized", "initialized"):
        return

    if method == "ping":
        if req_id is not None:
            send({"jsonrpc": "2.0", "id": req_id, "result": {}})
        return

    if method == "tools/list":
        send({"jsonrpc": "2.0", "id": req_id, "result": {"tools": TOOLS}})
        return

    if method == "tools/call":
        name = params.get("name", "")
        try:
            result = handle_call(name, params.get("arguments") or {})
            send({"jsonrpc": "2.0", "id": req_id, "result": result})
        except ValueError as exc:
            send(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "error": {"code": -32602, "message": str(exc)},
                }
            )
        return

    if req_id is not None:
        send(
            {
                "jsonrpc": "2.0",
                "id": req_id,
                "error": {"code": -32601, "message": f"method not found: {method}"},
            }
        )


def main() -> None:
    # Avoid block-buffering if Host forgot -u.
    try:
        sys.stdout.reconfigure(write_through=True)  # type: ignore[attr-defined]
    except Exception:
        pass
    while True:
        req = read_msg()
        if req is None:
            break
        handle(req)


if __name__ == "__main__":
    main()
