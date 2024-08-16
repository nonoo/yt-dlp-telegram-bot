FROM golang:1.22 AS builder
WORKDIR /app/
COPY go.mod go.sum /app/
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v

FROM python:alpine
RUN apk update && apk upgrade && apk add --no-cache ffmpeg
COPY --from=builder /app/yt-dlp-telegram-bot /app/yt-dlp-telegram-bot
COPY --from=builder /app/yt-dlp.conf /root/yt-dlp.conf

ENTRYPOINT ["/app/yt-dlp-telegram-bot"]
ENV API_ID= API_HASH= BOT_TOKEN= ALLOWED_USERIDS= ADMIN_USERIDS= ALLOWED_GROUPIDS= YTDLP_COOKIES=
