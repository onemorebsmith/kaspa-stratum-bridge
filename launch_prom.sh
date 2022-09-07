#!/bin/bash
docker run \
    -p 9090:9090 \
    -v $(pwd)/prom/prometheus.yaml:/etc/prometheus/prometheus.yml \
    -v prom-persist:/prometheus \
    --add-host host.docker.internal:host-gateway \
    prom/prometheus  &
