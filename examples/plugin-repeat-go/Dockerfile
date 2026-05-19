FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
COPY pkg ./pkg
COPY examples/plugin-repeat-go ./examples/plugin-repeat-go
WORKDIR /src/examples/plugin-repeat-go
RUN go mod download
RUN CGO_ENABLED=0 go build -o /plugin .

FROM alpine:3.20
RUN adduser -D -u 1000 plugin
USER plugin
COPY --from=builder /plugin /plugin
ENTRYPOINT ["/plugin"]
