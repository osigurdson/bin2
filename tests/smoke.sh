#!/usr/bin/env bash
set -e

REG=bin2.nthesis.ai/test

images=(
	alpine:3.19
	busybox:latest
	nginx:latest
	redis:latest
	postgres:latest
	node:20
)

for img in "${images[@]}"; do
	name=$(echo "$img" | cut -d: -f1)
	tag=$(echo "$img" | cut -d: -f2)

	echo "----"
	echo "Pulling $img"
	docker pull "$img"

	target="$REG/$name:$tag"

	echo "Tagging $target"
	docker tag "$img" "$target"

	echo "Pushing $target"
	docker push "$target"

	echo "Pulling from registry to verify"
	docker pull "$target"
done

echo "Smoke test complete."
