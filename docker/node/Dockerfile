#syntax=docker/dockerfile:1

FROM node:24-trixie-slim AS node_upstream

FROM node_upstream AS app_base

WORKDIR /app

RUN <<-EOF
	apt-get update
	apt-get install -y --no-install-recommends \
		git
	npx puppeteer browsers install chrome --install-deps
	rm -rf /var/lib/apt/lists/*
EOF

COPY --link --chmod=755 docker/node/docker-resumai-entrypoint.sh /usr/local/bin/docker-resumai-entrypoint.sh
ENTRYPOINT ["docker-resumai-entrypoint.sh"]

CMD ["node"]

FROM app_base AS app_dev

ENV NODE_ENV=development

FROM app_base AS app_prod

ENV NODE_ENV=production
