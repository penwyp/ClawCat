#!/bin/bash

echo "Testing cache functionality..."

# Run analyze command twice to test cache
echo "First run (should create cache):"
time ./bin/claudecat analyze --limit 5

echo -e "\nSecond run (should use cache):"
time ./bin/claudecat analyze --limit 5

echo -e "\nDone!"