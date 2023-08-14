FROM golang:1.20 as builder
WORKDIR /app/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v

FROM alpine as prep
RUN wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp

FROM python:alpine
RUN apk update
RUN apk upgrade
RUN apk add --no-cache ffmpeg
COPY --from=prep yt-dlp /usr/local/bin
RUN chmod 755 /usr/local/bin/yt-dlp
COPY --from=builder /app/yt-dlp-telegram-bot /app/yt-dlp-telegram-bot

CMD ["/app/yt-dlp-telegram-bot"]
ENV API_ID=
ENV API_HASH=
ENV BOT_TOKEN=
ENV ALLOWED_USERIDS=
ENV ADMIN_USERIDS=
ENV ALLOWED_GROUPIDS=