#!/bin/bash
# Mock Codex harness that emits an error.
echo '{"type":"error","code":"model_not_found","message":"Model '\''gpt-99'\'' is not available."}'
exit 1
