FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod init github.com/anirudhragam/GrayWatcher && go build -o verdict-server .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/verdict-server .
EXPOSE 80
CMD ["./verdict-server"]
