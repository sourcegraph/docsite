#!/usr/bin/env bash

# Auto-build and reload for development

# Usage
# ./dev/launch.sh
# or
# DOCSITE_CONFIG=../my-docs/docsite.json ./dev/launch.sh

DOCSITE_CONFIG="${DOCSITE_CONFIG:-$(pwd)/../sourcegraph/doc/docsite.json}"
PROCESS_NAME="docsitedev"

while true; do
    echo -e "\n# Launching docsite"
    killall ${PROCESS_NAME} > /dev/null 2>&1
    cd cmd/docsite || exit 1
    go get
    cd -  > /dev/null 2>&1 || exit 1
    bash -c "exec -a ${PROCESS_NAME} docsite -config '${DOCSITE_CONFIG}' serve" &
    notifywait ./  > /dev/null 2>&1 # TODO: restrict to Go files only
done
