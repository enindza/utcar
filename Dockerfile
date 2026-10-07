# Build the binary first (pure Go, no cgo needed for SQLite):
#   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
#   cp /etc/ssl/certs/ca-certificates.crt .
FROM scratch

ADD utcar /
ADD ca-certificates.crt /etc/ssl/certs/

# Database directory, e.g. --db /data/utcar.db (or UTCAR_DB=/data/utcar.db)
VOLUME /data

EXPOSE 12300

ENTRYPOINT ["/utcar"]
