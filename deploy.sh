#!/bin/bash

set -e
set -u

## Dogwood
# server="tools"
## Sound House
server="192.168.4.88"

make cross-compile
scp dist/sonoserve-linux-amd64 root@${server}:/usr/local/bin/sonoserve.new
ssh root@${server} mv /usr/local/bin/sonoserve.new /usr/local/bin/sonoserve
ssh root@${server} systemctl restart sonoserve.service
