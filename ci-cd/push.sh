#!/bin/bash
# $1 is the image to push to the registry
set -ex
tag="$(git describe --always --tags HEAD)"
team="tumblr"
repo="k8s-sidecar-injector"
image_base="${team}/${repo}"
image="${1:-${image_base}:${tag}}"
latest="${image_base}:latest"
[[ -z $image ]] && echo "missing image, I dont know what to push unless you tell me with \$1" && exit 1
docker push "$image"
docker push "$latest"
