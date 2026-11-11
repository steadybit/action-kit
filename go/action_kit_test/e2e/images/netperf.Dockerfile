# netperf and netserver for the e2e network tests.
# Based on Debian like the iperf3 image: Ubuntu's mirrors were not resolvable from the GitHub runners (2026-11-11).
FROM debian:trixie-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends netperf \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*
