#!/bin/bash

kubectl delete -f ./cr.yaml
kubectl delete -f ./crd-validation.yaml
kubectl delete -f ./controller.yaml
