ARG GO_VERSION=1.25.8

FROM golang:${GO_VERSION} AS build
ARG SERVICE_PATH
WORKDIR /src

COPY go.mod go.work Makefile buf.yaml buf.gen.yaml ./
COPY api ./api
COPY pkg ./pkg
COPY services ./services
COPY agent ./agent

RUN test -n "${SERVICE_PATH}"
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ${SERVICE_PATH}

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/app /app/app
USER appuser
ENTRYPOINT ["/app/app"]
