FROM golang:1.22-alpine AS build_deps

RUN apk add --no-cache git

WORKDIR /workspace

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

COPY main.go .

RUN CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .

FROM alpine:3 as final

RUN addgroup -g 1000 appgroup && adduser -u 1000 -G appgroup -D webhook

RUN apk add --no-cache ca-certificates

USER 1000

COPY --from=build /workspace/webhook /usr/local/bin/webhook

ENTRYPOINT ["webhook"]
