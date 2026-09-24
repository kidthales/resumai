#syntax=docker/dockerfile:1

FROM timbru31/node-alpine-git:24 AS node_upstream

FROM node_upstream AS app_base

WORKDIR /app

COPY --link --chmod=755 docker/node/docker-resumai-entrypoint.sh /usr/local/bin/docker-resumai-entrypoint.sh
ENTRYPOINT ["docker-resumai-entrypoint.sh"]

CMD ["node"]

FROM app_base AS app_dev

ENV NODE_ENV=development

FROM app_base AS app_prod

ENV NODE_ENV=production
