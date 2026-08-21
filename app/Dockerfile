# Obsidio engineered backend: Go. Multi-stage, pinned images, reproducible.
# Go >=1.25 required: stdlib crypto/sha256 gains a SHA-NI path on amd64 (~2x /risk
# on Zen/Ice Lake+ hardware); 1.22-1.24 top out at AVX2.
FROM golang:1.26-bookworm AS build
WORKDIR /app
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server .

FROM debian:bookworm-slim
COPY --from=build /app/server /server
# Belt-and-suspenders with runtime.GOMAXPROCS(2) in main(): never let the
# runtime size itself from the visible host cores.
ENV GOMAXPROCS=2
EXPOSE 8080
CMD ["/server"]
