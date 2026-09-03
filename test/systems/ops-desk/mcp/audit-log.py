#!/usr/bin/env python3
from __future__ import annotations
import json, sys

def respond(i, r=None, e=None):
    o = {"jsonrpc": "2.0", "id": i}; o.update({"error": e} if e else {"result": r})
    sys.stdout.write(json.dumps(o) + "\n"); sys.stdout.flush()

TOOLS = [
    {"name": "append_event", "description": "Append audit event.", "inputSchema": {"type": "object", "properties": {"kind": {"type": "string"}, "detail": {"type": "string"}}, "required": ["kind", "detail"], "additionalProperties": False}},
    {"name": "query_events", "description": "Query recent events.", "inputSchema": {"type": "object", "properties": {"limit": {"type": "integer"}}, "additionalProperties": False}},
    {"name": "export_incident_bundle", "description": "Bundle for postmortem.", "inputSchema": {"type": "object", "properties": {"incident_id": {"type": "string"}}, "required": ["incident_id"], "additionalProperties": False}},
]

def call_tool(n, a):
    if n == "append_event": return json.dumps({"stored": True, "kind": (a or {}).get("kind")})
    if n == "query_events": return json.dumps({"events": [{"kind": "deploy", "detail": "fixture"}]})
    if n == "export_incident_bundle": return json.dumps({"incident_id": (a or {}).get("incident_id"), "bundle": "fixture.zip"})
    return "unknown"

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line: continue
        try: req = json.loads(line)
        except json.JSONDecodeError: continue
        m, i = req.get("method"), req.get("id")
        if m == "initialize":
            respond(i, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "serverInfo": {"name": "audit-log", "version": "0.1.0"}})
        elif m == "notifications/initialized": continue
        elif m == "tools/list": respond(i, {"tools": TOOLS})
        elif m == "tools/call":
            p = req.get("params") or {}
            respond(i, {"content": [{"type": "text", "text": call_tool(p.get("name"), p.get("arguments") or {})}]})
        elif i is not None: respond(i, e={"code": -32601, "message": "nf"})

if __name__ == "__main__": main()
