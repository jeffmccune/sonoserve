#!/bin/bash

set -e
set -u

make cross-compile
scp dist/sonoserve-linux-amd64 root@tools:/usr/local/bin/sonoserve.new
ssh root@tools mv /usr/local/bin/sonoserve.new /usr/local/bin/sonoserve
ssh root@tools systemctl restart sonoserve.service
