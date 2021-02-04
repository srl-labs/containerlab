#!/bin/sh
# this script gets the token for mysocket.io service
# and saves it under mysocketio_token name in the current working dir
# usage `mysocket-token.sh <email> <password>`

curl -sX POST -H "Content-Type: application/json" \
     https://api.mysocket.io/login \
     -d "{\"email\":\"$1\",\"password\":\"$2\"}" \
     | awk '/token/ {print $2}' | tr -d \" > mysocketio_token