# Build context: backend/. Runtime contains only static binaries and CA roots.
FROM docker.io/library/golang:1.27.3-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/isaber-api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -o /out/isaber-migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -trimpath -o /out/isaber-owner ./cmd/owner \
 && CGO_ENABLED=0 go build -trimpath -o /out/isaber-probe ./cmd/probe \
 && mkdir -p /out/data/uploads && chmod 0700 /out/data/uploads

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/isaber-* /app/
# Copy the parent so uploads remains a child with its source mode preserved.
COPY --from=build --chown=65532:65532 /out/data/ /data/
USER 65532:65532
WORKDIR /app
ENV HTTP_ADDR=0.0.0.0:8080 APP_ENV=production UPLOAD_DIR=/data/uploads
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=4s --start-period=10s --retries=12 CMD ["/app/isaber-probe"]
ENTRYPOINT ["/app/isaber-api"]
