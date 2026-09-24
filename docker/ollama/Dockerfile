#syntax=docker/dockerfile:1

FROM ollama/ollama:0.34.3 AS ollama_upstream

FROM ollama_upstream AS app_base

RUN apt-get update && \
	apt-get install -y --no-install-recommends \
		curl && \
	rm -rf /var/lib/apt-get/lists/*

COPY --link --chmod=755 docker/ollama/docker-resumai-entrypoint.sh /usr/local/bin/docker-resumai-entrypoint.sh
ENTRYPOINT ["docker-resumai-entrypoint.sh"]

FROM app_base AS app_dev

FROM app_base AS app_prod
