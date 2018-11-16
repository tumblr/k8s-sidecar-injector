#!/bin/bash
# $1 is override for the image to build, otherwise assumes a sane default
set -ex
tag="$(git describe --always --tags HEAD)"
team="tumblr"
repo="k8s-sidecar-injector"
image_base="${team}/${repo}"
image="${1:-${image_base}:${tag}}"
latest="${image_base}:latest"
[[ -z $image ]] && echo "missing image, I dont know what to push unless you tell me with \$1" && exit 1
docker build -t "$image" -t "$latest" .
