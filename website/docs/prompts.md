# Session 2 - Prompts and Responses

## Turn 1 - Create dev-loop.sh script

> Create a dev-loop.sh script in the root of the repo that uses the make build task to build the project, then execute the binary in the background capturing stderr to a log file you can read. Wait for the ready endpoint to return true read a line from a fifo. If the line says "rebuild" repeat the loop, clearing out the log files.

Created `dev-loop.sh` that:
- Builds using `make build`
- Runs the binary with stderr redirected to `sonoserve.stderr.log`
- Waits for the health endpoint to return successfully
- Reads from FIFO `/tmp/sonoserve-dev-loop.fifo`
- Rebuilds when "rebuild" is sent to the FIFO

Usage: `./dev-loop.sh` then `echo rebuild > /tmp/sonoserve-dev-loop.fifo` to trigger rebuilds.

## Turn 2 - Reminder to update prompts.md

> Remember to update prompts.md and commit each iteration.

Updating prompts.md with the latest conversation and committing changes.
