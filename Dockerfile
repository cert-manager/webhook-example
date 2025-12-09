FROM golang:1.22-alpine3.19@sha256:6f73a1b8b608dad4866b9f746ac6888ffdb112f75ef59ed97c43b5f734368718 AS build_deps

RUN apk add --no-cache git

WORKDIR /workspace

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

COPY . .

RUN CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .

FROM alpine:3.23@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375

RUN apk add --no-cache ca-certificates

COPY --from=build /workspace/webhook /usr/local/bin/webhook

ENTRYPOINT ["webhook"]
