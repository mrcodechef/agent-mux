#!/bin/bash
# Mock Codex harness that runs slowly (for timeout testing).
echo '{"type":"thread.started","thread_id":"mock-thread-slow","model":"gpt-5.4"}'
echo '{"type":"turn.started","turn_index":0}'
# Sleep long enough to trigger timeout
sleep 30
echo '{"type":"item.completed","item_id":"item_001","item_type":"agent_message","content":"Finally done."}'
echo '{"type":"turn.completed","turn_index":0,"usage":{"input_tokens":100,"output_tokens":50},"duration_ms":30000}'
