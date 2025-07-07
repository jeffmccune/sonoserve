#!/bin/bash

set -e
set -u

make cross-compile
scp dist/sonoserve-linux-amd64 sound:/usr/local/bin/sonoserve.new
ssh sound mv /usr/local/bin/sonoserve.new /usr/local/bin/sonoserve
ssh sound systemctl restart sonoserve.service
