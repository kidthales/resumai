#syntax=docker/dockerfile:1

FROM ollama/ollama:0.34.3 AS ollama_upstream

FROM ollama_upstream AS app_base

FROM app_base AS app_dev

FROM app_base AS app_prod
