FROM golang:1.22-bookworm AS build
ENV GOPROXY=https://goproxy.cn,direct
ENV GOTOOLCHAIN=local
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build ./... && go build -o /workspace/pellet-line .

FROM debian:bookworm-slim
COPY --from=build /workspace/pellet-line /usr/local/bin/pellet-line
ENV PELLET_LINE_DB=/data/pellet-line.db
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/pellet-line"]
