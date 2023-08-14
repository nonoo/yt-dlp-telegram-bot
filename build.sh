#!/bin/bash

. config.inc.sh

docker build \
--build-arg API_ID=$API_ID \
--build-arg API_HASH=$API_HASH \
--build-arg BOT_TOKEN=$BOT_TOKEN \
--build-arg ALLOWED_USERIDS=$ALLOWED_USERIDS \
--build-arg ADMIN_USERIDS=$ADMIN_USERIDS \
--build-arg ALLOWED_GROUPIDS=$ALLOWED_GROUPIDS \
.