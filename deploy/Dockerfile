FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /seismic-server ./cmd/server
FROM alpine:3.20
RUN adduser -D -u 10001 seismic
USER seismic
WORKDIR /app
COPY --from=build /seismic-server /app/seismic-server
COPY configs/local.yaml /app/configs/local.yaml
EXPOSE 18336
ENTRYPOINT ["/app/seismic-server", "-config", "/app/configs/local.yaml"]

