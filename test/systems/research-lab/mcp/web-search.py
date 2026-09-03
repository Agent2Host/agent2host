#!/usr/bin/env python3
"""Fixture web-search MCP for research-lab."""
from __future__ import annotations
import json, sys

def respond(i, r=None, e=None):
    o = {"jsonrpc": "2.0", "id": i}
    o.update({"error": e} if e else {"result": r})
    sys.stdout.write(json.dumps(o) + "\n"); sys.stdout.flush()

TOOLS = [
    {"name": "search_web", "description": "Search fixture web index.", "inputSchema": {"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"], "additionalProperties": False}},
    {"name": "fetch_snippet", "description": "Fetch page snippet by id.", "inputSchema": {"type": "object", "properties": {"page_id": {"type": "string"}}, "required": ["page_id"], "additionalProperties": False}},
    {"name": "list_sources", "description": "List cached sources.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
]

def call_tool(n, a):
    if n == "search_web":
        q = (a or {}).get("query", "")
        return json.dumps({"query": q, "hits": [{"page_id": "p1", "title": "Agent2Host overview", "score": 0.9}], "canary": "WEB_SEARCH_OK"})
    if n == "fetch_snippet":
        pid = (a or {}).get("page_id", "")
        return json.dumps({"page_id": pid, "snippet": "Agent2Host registers Agent Systems and runs them on Hosts.", "url": "https://example.invalid/a2h"})
    if n == "list_sources":
        return json.dumps({"sources": ["p1", "p2"]})
    return "unknown"

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line: continue
        try: req = json.loads(line)
        except json.JSONDecodeError: continue
        m, i = req.get("method"), req.get("id")
        if m == "initialize":
            respond(i, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "serverInfo": {"name": "web-search", "version": "0.1.0"}})
        elif m == "notifications/initialized": continue
        elif m == "tools/list": respond(i, {"tools": TOOLS})
        elif m == "tools/call":
            p = req.get("params") or {}
            respond(i, {"content": [{"type": "text", "text": call_tool(p.get("name"), p.get("arguments") or {})}]})
        elif i is not None: respond(i, e={"code": -32601, "message": "nf"})

if __name__ == "__main__": main()
