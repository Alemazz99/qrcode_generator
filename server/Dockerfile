# Build dalla root della repo (serve docs/ per il renderer):
#   docker build -f server/Dockerfile -t qrserver .
FROM golang:1.26-alpine AS build
WORKDIR /src/server

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
# Come `make assets`: la web app di docs/ diventa il generatore pubblico su /crea/
COPY docs/ ./web/public/
RUN rm -f web/public/.nojekyll
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/qrserver .

FROM alpine:3.22
RUN adduser -D -H -u 10001 app && mkdir /data && chown app /data
COPY --from=build /out/qrserver /usr/local/bin/qrserver
USER app
ENV PORT=8080 DB_PATH=/data/qr.db
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:${PORT}/healthz || exit 1
CMD ["qrserver"]
