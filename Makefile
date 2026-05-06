.PHONY: all build clean clean-package compress dev generate generate-k8s generate-provider grep-no-monitoring-metrics-provisioned lint lint-new lint-tf lint-security lint-all mocks package release setup sync-dashboards test tools validate vendor verify-dashboards-synced

commands = api atom build convox docs resolver

binaries = $(addprefix $(GOPATH)/bin/, $(commands))
sources  = $(shell find . -name '*.go')

all: build

build: $(binaries)

clean: clean-package

clean-package:
	echo "not supported"

compress: $(binaries)
	echo "not supported"

dev:
	test -n "$(IMAGE)" # IMAGE
	test -n "$(RACK)" # RACK
	docker build --target development -t $(IMAGE) .
	docker push $(IMAGE)
	$(call restart,$(RACK)-system,deployment/api)
	$(call restart,$(RACK)-system,deployment/atom)
	$(call restart,$(RACK)-system,deployment/resolver)
	$(call restart,$(RACK)-system,deployment/router)
	$(call wait,$(RACK)-system,deployment/api)
	$(call wait,$(RACK)-system,deployment/atom)
	$(call wait,$(RACK)-system,deployment/resolver)
	$(call wait,$(RACK)-system,deployment/router)

generate: generate-provider generate-k8s

generate-k8s:
	make -C pkg/atom generate
	make -C provider/k8s generate

generate-provider:
	go run cmd/generate/main.go controllers > pkg/api/controllers.go
	go run cmd/generate/main.go routes > pkg/api/routes.go
	go run cmd/generate/main.go sdk > sdk/methods.go

mocks: generate-provider
	make -C pkg/atom mocks
	make -C pkg/structs mocks
	go run vendor/github.com/vektra/mockery/cmd/mockery/mockery.go -case underscore -dir pkg/start -outpkg sdk -output pkg/mock/start -name Interface
	go run vendor/github.com/vektra/mockery/cmd/mockery/mockery.go -case underscore -dir sdk -outpkg sdk -output pkg/mock/sdk -name Interface
	go run vendor/github.com/vektra/mockery/cmd/mockery/mockery.go -case underscore -dir vendor/github.com/convox/stdcli -outpkg stdcli -output pkg/mock/stdcli -name Executor
	go run vendor/github.com/vektra/mockery/cmd/mockery/mockery.go -case underscore -dir pkg/build -outpkg build -output pkg/mock/build -name Engine

package:
	echo "not needed"

release:
	test -n "$(VERSION)" # VERSION
	git tag $(VERSION) -s -m $(VERSION)
	git push origin refs/tags/$(VERSION)

test:
	env TEST=true go test -covermode atomic -coverprofile coverage.txt -mod=vendor ./...

lint:
	golangci-lint run --timeout 5m ./...

lint-new: grep-no-monitoring-metrics-provisioned
	golangci-lint run --timeout 5m --new-from-rev=$$(git merge-base HEAD master) ./...

grep-no-monitoring-metrics-provisioned:
	@matches=$$(find . -type f \( -name '*.go' -o -name '*.tf' -o -name '*.yaml' -o -name '*.yml' -o -name '*.md' -o -name '*.vue' -o -name '*.graphql' \) \
	  -not -path './.git/*' -not -path './vendor/*' -not -path './node_modules/*' \
	  -not -path './docs/reference/releases/3-24.md' \
	  -not -path './docs/configuration/monitoring.md' \
	  -not -path './Makefile' \
	  -not -path './pkg/cli/rack.go' \
	  -not -path './pkg/cli/rack_test.go' \
	  -not -path './pkg/rack/terraform_test.go' \
	  -not -path './pkg/rack/terraform_bounded_restart_test.go' \
	  -print0 | xargs -0 grep -n 'monitoring_metrics_provisioned' 2>/dev/null); \
	if [ -n "$$matches" ]; then printf '%s\n' "$$matches"; echo 'FAIL: monitoring_metrics_provisioned references outside the explicit allowlist'; exit 1; fi

lint-tf:
	@for dir in $$(find terraform -name '*.tf' -not -path './vendor/*' -not -path '*/.terraform/*' -exec dirname {} \; | sort -u); do \
		echo "==> tflint $$dir"; \
		(cd $$dir && tflint --config $$(git rev-parse --show-toplevel)/.tflint.hcl) || exit 1; \
		echo "==> terraform validate $$dir"; \
		(cd $$dir && terraform init -backend=false -input=false > /dev/null 2>&1 && terraform validate) || exit 1; \
	done

validate:
	@for dir in $$(find terraform -name '*.tf' -not -path './vendor/*' -not -path '*/.terraform/*' -exec dirname {} \; | sort -u); do \
		echo "==> terraform validate $$dir"; \
		(cd $$dir && terraform init -backend=false -input=false > /dev/null 2>&1 && terraform validate) || exit 1; \
	done

lint-security:
	checkov -d terraform --framework terraform --compact

lint-all: lint lint-tf lint-security

# GPU observability dashboards — manual sync from source-of-truth dir into
# the rack-side TF location. Source: examples/gpu-llm/grafana/*.json. The
# rack TF (terraform/cluster/aws/dashboards/) consumes these JSONs at apply
# time; the standalone Grafana convox app at convox-examples/grafana-gpu-dashboards
# ships dashboards inline. Run after editing any JSON dashboard.
sync-dashboards:
	@mkdir -p terraform/cluster/aws/dashboards
	@find terraform/cluster/aws/dashboards -maxdepth 1 -name '*.json' -delete
	@if ls examples/gpu-llm/grafana/*.json >/dev/null 2>&1; then \
		cp examples/gpu-llm/grafana/*.json terraform/cluster/aws/dashboards/; \
	fi
	@touch terraform/cluster/aws/dashboards/.gitkeep
	@echo "synced JSON dashboards to terraform/cluster/aws/dashboards"

# CI prereq — fails when downstream copy drifts from source. Run after edits
# without sync-dashboards. Manual-only sync target prevents auto-mutation.
verify-dashboards-synced:
	@srcs=$$(cd examples/gpu-llm/grafana 2>/dev/null && ls *.json 2>/dev/null | sort); \
	tfs=$$(cd terraform/cluster/aws/dashboards 2>/dev/null && ls *.json 2>/dev/null | sort); \
	if [ "$$srcs" != "$$tfs" ]; then \
		echo "FAIL: terraform/cluster/aws/dashboards out of sync with examples/gpu-llm/grafana"; \
		echo "  source files: $$srcs"; \
		echo "  tf files:     $$tfs"; \
		echo "  run: make sync-dashboards"; \
		exit 1; \
	fi; \
	for f in examples/gpu-llm/grafana/*.json; do \
		[ -e "$$f" ] || continue; \
		base=$$(basename "$$f"); \
		if ! diff -q "$$f" "terraform/cluster/aws/dashboards/$$base" >/dev/null; then \
			echo "FAIL: $$base differs between source and terraform/cluster/aws/dashboards"; \
			echo "  run: make sync-dashboards"; \
			exit 1; \
		fi; \
	done
	@echo "OK: dashboard JSON in sync (source + terraform/cluster/aws/dashboards)"

setup:
	pip install pre-commit
	pre-commit install

tools:
	go install -mod=vendor ./vendor/github.com/crazy-max/xgo

vendor:
	go mod vendor
	go mod tidy
	go run vendor/github.com/goware/modvendor/main.go -copy="**/*.c **/*.h"

$(binaries): $(GOPATH)/bin/%: $(sources)
	go install -mod=vendor --ldflags="-s -w" ./cmd/$*

define restart
	kubectl get $(2) -n $(1) >/dev/null 2>&1 && kubectl rollout restart $(2) -n $(1) || true
endef

define wait
	kubectl get $(2) -n $(1) >/dev/null 2>&1 && kubectl rollout status $(2) -n $(1) || true
endef
