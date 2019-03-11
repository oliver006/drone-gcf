#!/usr/bin/env sh

TOKEN=`curl -s -H "Content-Type: application/json" -X POST -d '{"username": "'$DOCKER_USERNAME'", "password": "'$DOCKER_PASSWORD'"}' https://hub.docker.com/v2/users/login/ | jq -r .token`

echo "Deleting tag: ${IMAGE_TAG}"
curl -s "https://hub.docker.com/v2/repositories/oliver006/drone-gcf/tags/${IMAGE_TAG}/" -X DELETE -H "Authorization: JWT ${TOKEN}"
