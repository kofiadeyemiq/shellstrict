#!/bin/bash
#
# A minimal script demonstrating the gap between ShellCheck and shellstrict:
# no strict-mode flags, and an mktemp result with no trap-based cleanup.
# ShellCheck (even with every optional check enabled) has nothing to say
# about either problem; shellstrict flags both. See the main README's
# "Why" section.

tmp=$(mktemp)
echo "working in $tmp"
cp "$config_file" "$tmp"
process "$tmp"
