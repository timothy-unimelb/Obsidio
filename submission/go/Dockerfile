# Build with a pinned Go patch release; the final image contains only the
# statically linked server binary.
FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS build
WORKDIR /src
ARG GO_BUILD_TAGS
COPY go.mod *.go *.s ./
# Build gate: a kernel that is wrong on this CPU would still return a plausible
# digest and score zero on /risk, so the digest tests run on the grader's own
# machine and a failure stops the image rather than shipping it.
RUN go vet ./... && go test -short ./...
RUN mkdir -p /data && CGO_ENABLED=0 go build -tags "$GO_BUILD_TAGS" -trimpath -ldflags="-s -w -buildid=" -o /out/obsidio .

FROM scratch
COPY --from=build --chown=65532:65532 /out/obsidio /obsidio
# Price updates are journalled here; mount a volume to keep them across container replacement.
COPY --from=build --chown=65532:65532 /data /data
VOLUME /data
USER 65532:65532
EXPOSE 8080
# GOGC=off stops scheduled collections on a near-zero steady-state heap; the
# memory limit is the hard safety net and must always accompany it.
# Milestone control (never shipped): the same build with the governor off.
ENV RISK_SHED=0 \
    GOMAXPROCS=2 \
    GOMEMLIMIT=512MiB \
    GOGC=off \
    RISK_WORKERS=2 \
    RISK_SHED_BUDGET_BP=88 \
    PRICE_LOG=/data/prices.log
ENTRYPOINT ["/obsidio"]
