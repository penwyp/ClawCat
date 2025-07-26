#!/bin/bash

# Test ClawCat run command with debug output
echo "Testing ClawCat run command..."

# Use test data path
export CLAWCAT_DEBUG=true
export CLAWCAT_LOG_LEVEL=debug

# Run clawcat with the test data path
./clawcat run --paths /Users/penwyp/Dat/worktree/claude_data_snapshot/projects --debug