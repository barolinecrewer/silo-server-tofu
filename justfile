# Load SILO_ACC_* from a gitignored .env for local acceptance runs.
set dotenv-load

module := "github.com/barolinecrewer/silo-server-tofu"
binary := "bin/terraform-provider-silo"
# The CLI used for acceptance tests and example validation. OpenTofu is
# the primary target; terraform falls back for compatibility testing.
# Empty string when neither is installed; recipes guard on it.
tofu := `command -v tofu || command -v terraform || true`

default: build

# build: compile the provider binary
build:
    go build -o {{ binary }} .

# test: offline unit tests; no server or network required
test:
    go test -v -count=1 ./...

# testacc: destructive acceptance tests against a throwaway server.
# Runs the OpenTofu CLI via TF_ACC_TERRAFORM_PATH. Requires TF_ACC=1 plus
# SILO_ACC_BASE_URL and SILO_ACC_API_KEY; never point at production.
testacc:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "{{ tofu }}" ]; then
        echo "neither tofu nor terraform found in PATH" >&2
        exit 1
    fi
    # The harness reattaches the provider under registry.terraform.io with
    # namespaces "-" and "hashicorp"; tofu rejects "-" and resolves a bare
    # "silo" to registry.opentofu.org/hashicorp/silo, so pin both.
    if [ "$(basename "{{ tofu }}")" = "tofu" ]; then
        export TF_ACC_PROVIDER_HOST=registry.opentofu.org
    fi
    TF_ACC=1 TF_ACC_TERRAFORM_PATH="{{ tofu }}" TF_ACC_PROVIDER_NAMESPACE=hashicorp \
        go test -v -count=1 ./internal/provider/ -run 'TestAcc'

# tofu-validate: build the provider and validate every example with the
# real tofu CLI via dev_overrides. No registry access needed.
tofu-validate: build
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "{{ tofu }}" ]; then
        echo "tofu not found in PATH" >&2
        exit 1
    fi
    tfrc="$(mktemp -t silo-dev.tfrc)"
    trap 'rm -f "$tfrc"' EXIT
    printf 'provider_installation {\n  dev_overrides {\n    "silo-server/silo" = "%s/bin"\n  }\n  direct {}\n}\n' "$PWD" > "$tfrc"
    for d in examples/provider examples/resources/* examples/data-sources/*; do
        [ -d "$d" ] || continue
        echo "tofu validate: $d"
        (cd "$d" && TF_CLI_CONFIG_FILE="$tfrc" "{{ tofu }}" validate >/dev/null)
    done
    echo "all examples valid"

# lint: golangci-lint plus `tofu fmt -check` on examples
lint:
    #!/usr/bin/env bash
    set -euo pipefail
    golangci-lint run
    if [ -n "{{ tofu }}" ]; then
        "{{ tofu }}" fmt -check -recursive examples/
    else
        echo "tofu not installed; skipping example fmt check"
    fi

# fmt: gofmt + goimports + `tofu fmt` on examples
fmt:
    go fmt ./...
    @command -v goimports >/dev/null && goimports -w . || true
    @if [ -n "{{ tofu }}" ]; then "{{ tofu }}" fmt -recursive examples/; \
        else echo "tofu not installed; skipping example fmt"; fi

# vet: go vet
vet:
    go vet ./...

# generate: regenerate provider docs from examples (tfplugindocs)
generate:
    cd tools && go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs
    tfplugindocs generate

# clean: remove build output
clean:
    rm -rf bin/
