FROM --platform=linux/amd64 ghcr.io/kamailio/kamailio:6.1.2-bookworm

USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends sipp ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /scenarios
ENTRYPOINT ["sipp"]
