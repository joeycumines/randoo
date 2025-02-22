#!/bin/sh

trap 'echo "Debug: SIGINT received, exiting in 5 seconds..."; sleep 5; exit 1' INT

echo "Running... (Press Ctrl+C to trigger SIGINT)"
while :; do
    sleep 1
done
