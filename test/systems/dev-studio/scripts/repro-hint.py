#!/usr/bin/env python3
"""Fixture repro hint for debugging skill."""
import json
import sys

print(json.dumps({"hint": "Reproduce with: go test ./internal/adapter/ -run Acceptance -count=1", "canary": "REPRO_HINT_OK"}))
