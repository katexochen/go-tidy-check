FROM golang:1.19 as builder
WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download all

COPY ./ ./

ARG VERSION
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-X main.Version=${VERSION}" \
    -o go-tidy-check .

FROM gcr.io/distroless/base:latest
WORKDIR /
COPY --from=builder /workspace/go-tidy-check .
COPY --from=builder /usr/local/go/bin/go /usr/local/go/bin/go
ENV PATH="${PATH}:/usr/local/go/bin"
ENTRYPOINT ["/go-tidy-check"]
