#!/bin/bash

. config.inc.sh

bin=./yt-dlp-telegram-bot
if [ ! -x "$bin" ]; then
	bin="go run *.go"
fi

API_ID=$API_ID \
API_HASH=$API_HASH \
BOT_TOKEN=$BOT_TOKEN \
ALLOWED_USERIDS=$ALLOWED_USERIDS \
ADMIN_USERIDS=$ADMIN_USERIDS \
ALLOWED_GROUPIDS=$ALLOWED_GROUPIDS \
MAX_SIZE=$MAX_SIZE \
YTDLP_PATH=$YTDLP_PATH \
$bin
