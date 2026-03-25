#!/bin/bash
# Mock Codex harness that emits valid NDJSON event stream.
# Simulates: session start -> tool use -> file write -> response -> turn complete

echo '{"type":"thread.started","thread_id":"mock-thread-001","model":"gpt-5.4","created_at":"2026-03-25T10:00:00Z"}'
echo '{"type":"turn.started","turn_index":0}'
echo '{"type":"item.started","item_id":"item_001","item_type":"command_execution","command":"echo hello"}'
sleep 0.1
echo '{"type":"item.completed","item_id":"item_001","item_type":"command_execution","command":"echo hello","exit_code":0,"stdout":"hello\n","stderr":"","duration_ms":50}'
echo '{"type":"item.started","item_id":"item_002","item_type":"file_change","file_path":"hello.go"}'
echo '{"type":"item.completed","item_id":"item_002","item_type":"file_change","file_path":"hello.go","change_type":"created","duration_ms":30}'
echo '{"type":"item.started","item_id":"item_003","item_type":"agent_message"}'
echo '{"type":"item.updated","item_id":"item_003","item_type":"agent_message","content_delta":"Done. Created hello.go"}'
echo '{"type":"item.completed","item_id":"item_003","item_type":"agent_message","content":"Done. Created hello.go with a hello world program.","duration_ms":500}'
echo '{"type":"turn.completed","turn_index":0,"usage":{"input_tokens":1000,"output_tokens":200,"reasoning_tokens":50},"duration_ms":1200}'
