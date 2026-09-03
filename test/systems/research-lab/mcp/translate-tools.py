#!/usr/bin/env python3
"""Fixture translate-tools MCP for research-lab english coach."""
from __future__ import annotations
import json, sys

def respond(i, r=None, e=None):
    o = {"jsonrpc": "2.0", "id": i}; o.update({"error": e} if e else {"result": r})
    sys.stdout.write(json.dumps(o) + "\n"); sys.stdout.flush()

TOOLS = [
    {"name": "lookup_word", "description": "Dictionary lookup fixture.", "inputSchema": {"type": "object", "properties": {"word": {"type": "string"}}, "required": ["word"], "additionalProperties": False}},
    {"name": "suggest_example", "description": "Example sentence for a word.", "inputSchema": {"type": "object", "properties": {"word": {"type": "string"}}, "required": ["word"], "additionalProperties": False}},
    {"name": "score_grammar", "description": "Fixture grammar score.", "inputSchema": {"type": "object", "properties": {"sentence": {"type": "string"}}, "required": ["sentence"], "additionalProperties": False}},
]

def call_tool(n, a):
    w = (a or {}).get("word", (a or {}).get("sentence", ""))
    if n == "lookup_word": return json.dumps({"word": w, "definition": f"fixture definition of {w}"})
    if n == "suggest_example": return json.dumps({"word": w, "example": f"I use {w} in my daily research."})
    if n == "score_grammar": return json.dumps({"sentence": w, "score": 0.85, "canary": "GRAMMAR_OK"})
    return "unknown"

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line: continue
        try: req = json.loads(line)
        except json.JSONDecodeError: continue
        m, i = req.get("method"), req.get("id")
        if m == "initialize":
            respond(i, {"protocolVersion": "2024-11-05", "capabilities": {"tools": {}}, "serverInfo": {"name": "translate-tools", "version": "0.1.0"}})
        elif m == "notifications/initialized": continue
        elif m == "tools/list": respond(i, {"tools": TOOLS})
        elif m == "tools/call":
            p = req.get("params") or {}
            respond(i, {"content": [{"type": "text", "text": call_tool(p.get("name"), p.get("arguments") or {})}]})
        elif i is not None: respond(i, e={"code": -32601, "message": "nf"})

if __name__ == "__main__": main()
