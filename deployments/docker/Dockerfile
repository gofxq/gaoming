ARG GO_VERSION=1.25.8
ARG NODE_VERSION=24

FROM node:${NODE_VERSION}-alpine AS web-build
WORKDIR /web
COPY web/package.json web/yarn.lock ./
RUN yarn install --frozen-lockfile
COPY web/ ./
# API and SSE use the same origin as the served page.
RUN VITE_API_ORIGIN= VITE_API_BASE_URL= VITE_STREAM_PATH= yarn build

FROM golang:${GO_VERSION} AS build
ARG SERVICE_PATH=./server
WORKDIR /src
COPY go.mod go.sum ./
COPY api ./api
COPY pkg ./pkg
COPY server ./server
COPY agent ./agent
# Match this stage's architecture to the requested Docker target platform.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/app ${SERVICE_PATH}

FROM alpine:3.20 AS runtime
RUN adduser -D -u 10001 appuser && mkdir -p /app/logs && chown -R appuser:appuser /app
WORKDIR /app
COPY --from=build /out/app /app/app
USER appuser
ENTRYPOINT ["/app/app"]

FROM runtime AS agent

FROM runtime AS server
COPY --from=web-build /web/dist /app/web
ENV GAOMING_WEB_DIR=/app/web
EXPOSE 9090 9091
