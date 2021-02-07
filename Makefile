VERSION = $(shell git rev-parse HEAD)
DOCKER_BUILD_ARGS = --network host --build-arg https_proxy=${https_proxy} --build-arg BUILT_VERSION=${VERSION}
CWD		= `pwd`

build::
	docker build ${DOCKER_BUILD_ARGS} -t quay.io/vxlabs/wasp:${VERSION} .
release:: build release-nodep
deploy:
	fly deploy -i vxlabs/wasp:${VERSION}
test::
	go test -v ./...
watch::
	while true; do inotifywait -qq -r -e create,close_write,modify,move,delete ./ && clear; date; echo; go test ./...; done
cistatus::
	@curl -s https://api.github.com/repos/vx-labs/wasp/actions/runs | jq -r '.workflow_runs[] | ("[" + .created_at + "] " + .head_commit.message +": "+.status+" ("+.conclusion+")")'  | head -n 5

lintMD:
	docker run --rm \
	  -v ${CWD}/README.md:/srv/src/README.md \
	  --workdir /srv \
	    mivok/markdownlint:0.4.0 -r ~MD024,~MD025,~MD013 .