FROM golang:1.21-alpine AS builder

WORKDIR /go/src/github.com/krobertson/chia-garden
ADD . /go/src/github.com/krobertson/chia-garden/
RUN go get ./...

RUN CGO_ENABLED=0 GOOS=linux go build -o chia-garden main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/krobertson/chia-garden/chia-garden /chia-garden

EXPOSE 3434

ENTRYPOINT ["/chia-garden"]
