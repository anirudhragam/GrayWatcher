FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod init github.com/anirudhragam/GrayWatcher && \
    go get gonum.org/v1/gonum@v0.17.0 && \
    go build -o verdict-server .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/verdict-server .
EXPOSE 80
CMD ["./verdict-server"]
