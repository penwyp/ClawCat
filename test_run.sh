#!/bin/bash

# Test claudecat run command with debug output
echo "Testing claudecat run command..."

# Use test data path
export CLAWCAT_DEBUG=true
export CLAWCAT_LOG_LEVEL=debug

# Run claudecat with the test data path
./claudecat run --paths /Users/penwyp/Dat/worktree/claude_data_snapshot/projects --debug