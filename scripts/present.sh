#!/usr/bin/env bash
# Open the slides in the terminal. Keys: → / space / l / j next, ← / h / k back, ctrl+e run code block, q quit.
cd "$(dirname "$0")/../slides" && exec slides talk.md
