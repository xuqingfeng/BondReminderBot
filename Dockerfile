FROM golang:alpine as builder

WORKDIR /go/src/app

COPY . .

RUN go mod download && \
    GOOS=linux GOARCH=amd64 go build -o brb .

FROM alpine

WORKDIR /app

RUN apk add --no-cache curl

COPY --from=builder /go/src/app/brb /app/brb

CMD ["/app/brb"]