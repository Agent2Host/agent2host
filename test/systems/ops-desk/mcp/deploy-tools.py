#!/usr/bin/env python3
from __future__ import annotations
import json, sys

def respond(i, r=None, e=None):
    o = {"jsonrpc": "2.0", "id": i}; o.update({"error": e} if e else {"result": r})
    sys.stdout.write(json.dumps(o) + "\n"); sys.stdout.flush()

TOOLS = [
    {"name": "plan_deploy", "description": "Fixture deploy plan.", "inputSchema": {"type": "object", "properties": {"env": {"type": "string"}}, "required": ["env"], "additionalProperties": False}},
    {"name": "rollback_status", "description": "Rollback readiness.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
    {"name": "health_check", "description": "Service health fixture.", "inputSchema": {"type": "object", "properties": {"service": {"type": "string"}}, "additionalProperties": False}},
    {"name": "forbidden_deploy", "description": "Off allowlist.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
]

def call_tool(n, a):
    if n == "plan_deploy": return json.dumps({"env": (a or {}).get("env"), "steps": ["preflight", "apply", "verify"], "canary": "DEPLOY_PLAN_OK"})
    if n == "rollback_status": return json.dumps({"ready": True, "last_good": "v0.1.0-alpha.2"})
    if n == "health_check": return json.dumps({"service": (a or {}).get("service", "api"), "status": "healthy"})
    if n == "forbidden_deploy": return "FORBIDDEN"
    return "unknown"

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line: continue
        try: req = json.loads(line)
        except json.JSONDecodeError: continue
        m, i = req.get("method"), req.get("id")
        if m == "initialize":
            respond(i, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "serverInfo": {"name": "deploy-tools", "version": "0.1.0"}})
        elif m == "notifications/initialized": continue
        elif m == "tools/list": respond(i, {"tools": TOOLS})
        elif m == "tools/call":
            p = req.get("params") or {}
            respond(i, {"content": [{"type": "text", "text": call_tool(p.get("name"), p.get("arguments") or {})}]})
        elif i is not None: respond(i, e={"code": -32601, "message": "nf"})

if __name__ == "__main__": main()
