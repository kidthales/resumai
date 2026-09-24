#syntax=docker/dockerfile:1

FROM timbru31/node-alpine-git:24 AS node_upstream

FROM node_upstream AS app_base

WORKDIR /app

FROM app_base AS app_dev

ENV NODE_ENV=development

FROM app_base AS app_prod

ENV NODE_ENV=production
