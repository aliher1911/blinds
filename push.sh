#!/bin/bash

set -eu

base="$(dirname "$0")"

if [ ! -f "${base}/env.sh" ] ; then
  echo "Fill in details in env.sh"
  cat >"${base}/env.sh" <<EOF
# Uncomment and change target to point to remote scp destination for binary
#target="pi@raspberry.local:"
EOF
  exit 1
fi

. "${base}/env.sh"

if [ -z "${target:-}" ] ; then
  echo "env.sh is not configured correctly"
  exit 1  
fi

GOOS=linux GOARCH=arm go build -o blinds main.go

# Don't forget to set up public key logins

scp blinds pi@blinds:

