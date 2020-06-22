VERSION = $(shell git rev-parse HEAD)
DOCKER_BUILD_ARGS = --network host --build-arg https_proxy=${https_proxy} --build-arg BUILT_VERSION=${VERSION}

build::
	docker build ${DOCKER_BUILD_ARGS} -t quay.io/vxlabs/wasp:${VERSION} .
release:: build release-nodep
deploy:
	terraform apply -auto-approve -var image_tag=${VERSION}
nuke:
	terraform destroy -auto-approve -var image_tag=${VERSION}
test::
	go test -v ./...
watch::
	while true; do inotifywait -qq -r -e create,close_write,modify,move,delete ./ && clear; date; echo; go test ./...; done
cistatus::
	@curl -s https://api.github.com/repos/vx-labs/wasp/actions/runs | jq -r '.workflow_runs[] | ("[" + .created_at + "] " + .head_commit.message +": "+.status+" ("+.conclusion+")")'  | head -n 5

