#!/bin/bash

if [ "$#" -ne 1 ]; then
  echo "node name is not specified."
  exit 1
fi

if kubectl get node "$1" &>/dev/null; then
  kubectl annotate --overwrite nodes "$1" 'cmoperator.io/scrape'='true' 'cmoperator.io/port'='10255'
else
  echo "node $1 not found."
fi