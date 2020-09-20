#!/bin/bash

kubectl delete configmap cm-prometheus
sleep 5
kubectl create configmap cm-prometheus --from-file=prometheus.yml
kubectl delete pods -l=app=prometheus-server