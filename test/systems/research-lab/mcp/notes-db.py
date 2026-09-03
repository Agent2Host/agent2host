#!/usr/bin/env python3
"""Fixture notes-db MCP for research-lab."""
from __future__ import annotations
import json, sys

def respond(i, r=None, e=None):
    o = {"jsonrpc": "2.0", "id": i}; o.update({"error": e} if e else {"result": r})
    sys.stdout.write(json.dumps(o) + "\n"); sys.stdout.flush()

TOOLS = [
    {"name": "save_note", "description": "Save a research note.", "inputSchema": {"type": "object", "properties": {"title": {"type": "string"}, "body": {"type": "string"}}, "required": ["title", "body"], "additionalProperties": False}},
    {"name": "search_notes", "description": "Search notes by keyword.", "inputSchema": {"type": "object", "properties": {"q": {"type": "string"}}, "required": ["q"], "additionalProperties": False}},
    {"name": "link_notes", "description": "Link two note ids.", "inputSchema": {"type": "object", "properties": {"from_id": {"type": "string"}, "to_id": {"type": "string"}}, "required": ["from_id", "to_id"], "additionalProperties": False}},
    {"name": "list_tags", "description": "List note tags.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
]

def call_tool(n, a):
    if n == "save_note": return json.dumps({"id": "n42", "title": (a or {}).get("title"), "status": "saved"})
    if n == "search_notes": return json.dumps({"q": (a or {}).get("q"), "results": [{"id": "n1", "title": "MCP notes"}]})
    if n == "link_notes": return json.dumps({"linked": True, "from": (a or {}).get("from_id"), "to": (a or {}).get("to_id")})
    if n == "list_tags": return json.dumps({"tags": ["a2h", "research", "english"]})
    return "unknown"

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line: continue
        try: req = json.loads(line)
        except json.JSONDecodeError: continue
        m, i = req.get("method"), req.get("id")
        if m == "initialize":
            respond(i, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "serverInfo": {"name": "notes-db", "version": "0.1.0"}})
        elif m == "notifications/initialized": continue
        elif m == "tools/list": respond(i, {"tools": TOOLS})
        elif m == "tools/call":
            p = req.get("params") or {}
            respond(i, {"content": [{"type": "text", "text": call_tool(p.get("name"), p.get("arguments") or {})}]})
        elif i is not None: respond(i, e={"code": -32601, "message": "nf"})

if __name__ == "__main__": main()
