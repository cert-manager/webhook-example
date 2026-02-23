FROM golang:1.26.0-alpine AS build

WORKDIR /workspace

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -o webhook .

FROM gcr.io/distroless/static-debian13

COPY --from=build /workspace/webhook /usr/local/bin/webhook

ENTRYPOINT ["webhook"]
