#syntax=docker/dockerfile:1

FROM ollama/ollama:0.34.3 AS ollama_upstream

FROM ollama_upstream AS app_base

RUN <<-EOF
	apt-get update
	apt-get install -y --no-install-recommends
	rm -rf /var/lib/apt-get/lists/*
EOF

FROM app_base AS app_dev

FROM app_base AS app_prod
