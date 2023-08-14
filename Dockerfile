FROM golang:1.20 as builder
WORKDIR /app/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v

FROM python:alpine
RUN wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -O /usr/local/bin/yt-dlp && \
	chmod 755 /usr/local/bin/yt-dlp
RUN apk update && apk upgrade && apk add --no-cache ffmpeg
COPY --from=builder /app/yt-dlp-telegram-bot /app/yt-dlp-telegram-bot

ENTRYPOINT ["/app/yt-dlp-telegram-bot"]
ENV API_ID= API_HASH= BOT_TOKEN= ALLOWED_USERIDS= ADMIN_USERIDS= ALLOWED_GROUPIDS=
