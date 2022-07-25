#!/bin/sh

rm -rf dist/ && mkdir dist

export UID=$(id -u)
export GID=$(id -g)
docker run --rm \
	-v `pwd`/go:/go/src -v `pwd`/dist:/dist \
	--workdir /go/src \
	--user $UID:$GID \
	--entrypoint ./cross-compile.sh \
	goreleaser/goreleaser-cross:v1.18.3
