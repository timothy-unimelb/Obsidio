# Obsidio engineered backend: Go. Multi-stage, pinned images, reproducible.
FROM golang:1.22-bookworm AS build
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
