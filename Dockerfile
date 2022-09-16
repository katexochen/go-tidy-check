FROM golang:1.19 as builder
WORKDIR /workspace

ARG goproxy=https://proxy.golang.org
ENV GOPROXY=$goproxy

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download all

COPY ./ ./

ARG RELEASE_VERSION
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-X main.Version=${RELEASE_VERSION}" \
    -o go-tidy-check .

FROM golang:1.19
WORKDIR /
COPY --from=builder /workspace/go-tidy-check .
ENTRYPOINT ["/go-tidy-check"]
