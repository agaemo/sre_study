FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod .
COPY main.go .
RUN go build -o database .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/database .
EXPOSE 8082
CMD ["./database"]
