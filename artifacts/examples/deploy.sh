#!/bin/bash

kubectl apply -f ./crd-validation.yaml
kubectl apply -f ./controller.yaml
kubectl apply -f ./cr.yaml
