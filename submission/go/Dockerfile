# Build with a pinned Go patch release; the final image contains only the
# statically linked server binary.
FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o /out/obsidio ./main.go

FROM scratch
COPY --from=build --chown=65532:65532 /out/obsidio /obsidio
USER 65532:65532
EXPOSE 8080
ENV GOMAXPROCS=2 \
    GOMEMLIMIT=1536MiB \
    GOGC=200 \
    RISK_WORKERS=2
ENTRYPOINT ["/obsidio"]
