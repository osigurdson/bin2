SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

export KUBECONFIG := /home/owen/dev/bin2/infra/terraform/config

DOTENV_FILE := .env
API_COUNT_FILE := cmd/api/.pushcount
API_IMAGE_REPO := lax.vultrcr.com/bin2/api
API_REGISTRY := https://lax.vultrcr.com/bin2
API_NAMESPACE := prod
API_DEPLOYMENT := api
API_CONTAINER := api
INIT_COUNT_FILE := cmd/init/.pushcount
INIT_IMAGE_REPO := lax.vultrcr.com/bin2/init
INIT_REGISTRY := https://lax.vultrcr.com/bin2
INIT_NAMESPACE := prod
INIT_DEPLOYMENT := api
INIT_CONTAINER := init
SEED_E2E_CONTAINER := seed-e2e
UI_DIR := ui
UI_COUNT_FILE := ui/.pushcount
UI_IMAGE_REPO := lax.vultrcr.com/bin2/ui
UI_REGISTRY := https://lax.vultrcr.com/bin2
UI_NAMESPACE := prod
UI_DEPLOYMENT := ui
UI_CONTAINER := ui
PULL_DIR := pull

LOAD_DOTENV = if [[ -f "$(DOTENV_FILE)" ]]; then set -a; source "$(DOTENV_FILE)"; set +a; fi;

.PHONY: deploy-all deploy-api deploy-init push-api deploy-ui deploy-pull api-logs ui-logs init-logs

# Deploys all production services in order.
deploy-all:
	$(MAKE) deploy-init
	$(MAKE) deploy-api
	$(MAKE) deploy-ui
	$(MAKE) deploy-pull

# Builds, containerizes, pushes and updates the image tag in Kubernetes
deploy-api:
	$(LOAD_DOTENV) \
	build_dir="$$(mktemp -d)"; \
	trap 'rm -rf "$$build_dir"' EXIT; \
	: "$${VULTR_CR_USERNAME:?VULTR_CR_USERNAME is required}"; \
	: "$${VULTR_CR_PASSWORD:?VULTR_CR_PASSWORD is required}"; \
	if [[ ! -f "$(API_COUNT_FILE)" ]]; then \
		echo "missing $(API_COUNT_FILE)"; \
		exit 1; \
	fi; \
	tag="$$(tr -d '[:space:]' < "$(API_COUNT_FILE)")"; \
	if [[ ! "$$tag" =~ ^[0-9]+$$ ]]; then \
		echo "invalid $(API_COUNT_FILE) value: $$tag"; \
		exit 1; \
	fi; \
	podman login "$(API_REGISTRY)" -u "$$VULTR_CR_USERNAME" -p "$$VULTR_CR_PASSWORD"; \
	go build -o "$$build_dir/api" ./cmd/api; \
	cp cmd/api/Dockerfile "$$build_dir/Dockerfile"; \
	echo "building $(API_IMAGE_REPO):$$tag"; \
	podman build --quiet -t api:"$$tag" "$$build_dir"; \
	podman tag api:"$$tag" "$(API_IMAGE_REPO):$$tag"; \
	echo "pushing $(API_IMAGE_REPO):$$tag"; \
	podman push "$(API_IMAGE_REPO):$$tag"; \
	echo "updating deployment/$(API_DEPLOYMENT) to $(API_IMAGE_REPO):$$tag"; \
	kubectl -n "$(API_NAMESPACE)" set image deployment/"$(API_DEPLOYMENT)" "$(API_CONTAINER)"="$(API_IMAGE_REPO):$$tag"; \
	kubectl -n "$(API_NAMESPACE)" rollout status deployment/"$(API_DEPLOYMENT)"; \
	echo $$((tag + 1)) > "$(API_COUNT_FILE)"

# Builds, containerizes, pushes and updates the init image tag in Kubernetes
deploy-init:
	$(LOAD_DOTENV) \
	build_dir="$$(mktemp -d)"; \
	trap 'rm -rf "$$build_dir"' EXIT; \
	: "$${VULTR_CR_USERNAME:?VULTR_CR_USERNAME is required}"; \
	: "$${VULTR_CR_PASSWORD:?VULTR_CR_PASSWORD is required}"; \
	if [[ ! -f "$(INIT_COUNT_FILE)" ]]; then \
		echo "missing $(INIT_COUNT_FILE)"; \
		exit 1; \
	fi; \
	tag="$$(tr -d '[:space:]' < "$(INIT_COUNT_FILE)")"; \
	if [[ ! "$$tag" =~ ^[0-9]+$$ ]]; then \
		echo "invalid $(INIT_COUNT_FILE) value: $$tag"; \
		exit 1; \
	fi; \
	podman login "$(INIT_REGISTRY)" -u "$$VULTR_CR_USERNAME" -p "$$VULTR_CR_PASSWORD"; \
	go build -o "$$build_dir/init" ./cmd/init; \
	cp cmd/init/Dockerfile "$$build_dir/Dockerfile"; \
	echo "building $(INIT_IMAGE_REPO):$$tag"; \
	podman build --quiet -t init:"$$tag" "$$build_dir"; \
	podman tag init:"$$tag" "$(INIT_IMAGE_REPO):$$tag"; \
	echo "pushing $(INIT_IMAGE_REPO):$$tag"; \
	podman push "$(INIT_IMAGE_REPO):$$tag"; \
	echo "updating deployment/$(INIT_DEPLOYMENT) init containers to $(INIT_IMAGE_REPO):$$tag"; \
	kubectl -n "$(INIT_NAMESPACE)" set image deployment/"$(INIT_DEPLOYMENT)" \
		"$(INIT_CONTAINER)"="$(INIT_IMAGE_REPO):$$tag" \
		"$(SEED_E2E_CONTAINER)"="$(INIT_IMAGE_REPO):$$tag"; \
	kubectl -n "$(INIT_NAMESPACE)" rollout status deployment/"$(INIT_DEPLOYMENT)"; \
	echo $$((tag + 1)) > "$(INIT_COUNT_FILE)"

# Builds, containerizes, pushes and updates the image tag in Kubernetes
deploy-ui:
	$(LOAD_DOTENV) \
	legacy_env_local_tmp="$(UI_DIR)/__env_local"; \
	env_local_tmp="$$(mktemp)"; \
	moved_env_local=0; \
	restore_env_local() { \
		if [[ "$$moved_env_local" == "1" && -f "$$env_local_tmp" ]]; then \
			mv "$$env_local_tmp" "$(UI_DIR)/.env.local"; \
		else \
			rm -f "$$env_local_tmp"; \
		fi; \
	}; \
	trap restore_env_local EXIT; \
	: "$${VULTR_CR_USERNAME:?VULTR_CR_USERNAME is required}"; \
	: "$${VULTR_CR_PASSWORD:?VULTR_CR_PASSWORD is required}"; \
	if [[ ! -f "$(UI_COUNT_FILE)" ]]; then \
		echo "missing $(UI_COUNT_FILE)"; \
		exit 1; \
	fi; \
	tag="$$(tr -d '[:space:]' < "$(UI_COUNT_FILE)")"; \
	if [[ ! "$$tag" =~ ^[0-9]+$$ ]]; then \
		echo "invalid $(UI_COUNT_FILE) value: $$tag"; \
		exit 1; \
	fi; \
	if [[ ! -f "$(UI_DIR)/.env.local" && -f "$$legacy_env_local_tmp" ]]; then \
		echo "restoring legacy env backup from $$legacy_env_local_tmp"; \
		mv "$$legacy_env_local_tmp" "$(UI_DIR)/.env.local"; \
	fi; \
	if [[ -f "$(UI_DIR)/.env.local" ]]; then \
		mv "$(UI_DIR)/.env.local" "$$env_local_tmp"; \
		moved_env_local=1; \
	fi; \
	podman login "$(UI_REGISTRY)" -u "$$VULTR_CR_USERNAME" -p "$$VULTR_CR_PASSWORD"; \
	echo "building $(UI_IMAGE_REPO):$$tag"; \
	NODE_ENV=production npm --prefix "$(UI_DIR)" run build; \
	podman build --quiet -t ui:"$$tag" "$(UI_DIR)"; \
	podman tag ui:"$$tag" "$(UI_IMAGE_REPO):$$tag"; \
	echo "pushing $(UI_IMAGE_REPO):$$tag"; \
	podman push "$(UI_IMAGE_REPO):$$tag"; \
	echo "updating deployment/$(UI_DEPLOYMENT) to $(UI_IMAGE_REPO):$$tag"; \
	kubectl -n "$(UI_NAMESPACE)" set image deployment/"$(UI_DEPLOYMENT)" "$(UI_CONTAINER)"="$(UI_IMAGE_REPO):$$tag"; \
	kubectl -n "$(UI_NAMESPACE)" rollout status deployment/"$(UI_DEPLOYMENT)"; \
	echo $$((tag + 1)) > "$(UI_COUNT_FILE)"

# Deploys the wrangler based OCI pull service
deploy-pull:
	npm --prefix "$(PULL_DIR)" run deploy

# Gets the logs from the api
api-logs:
	kubectl logs deployment/$(API_DEPLOYMENT) -n $(API_NAMESPACE)

# Gets the logs from the ui
ui-logs:
	kubectl logs deployment/$(UI_DEPLOYMENT) -n $(UI_NAMESPACE)

# Gets the logs from the init process
init-logs:
		kubectl logs deployment/$(API_DEPLOYMENT) -n $(API_NAMESPACE) -c init
