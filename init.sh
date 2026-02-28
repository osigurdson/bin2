set -e
podman kube down compose.yaml
podman kube play compose.yaml
source .env
sleep 2
# This build doesn't seem to build everything
go build ./...
go run ./cmd/init clean
go run ./cmd/init migrate
# go run ./cmd/init add-user user@example.com --sub user
# go run ./cmd/init add-user other@example.com --sub other
