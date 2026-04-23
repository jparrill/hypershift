RUNTIME ?= $(shell sh ../hack/utils.sh get_container_engine)
version := $(shell awk -f image-version.awk requirements.txt)
VERSION ?= $(if $(MKDOCS_TAG),$(MKDOCS_TAG),$(version))
IMG ?= quay.io/hypershift/mkdocs-material:$(VERSION)

VENV := .venv
PIP := $(VENV)/bin/pip
MKDOCS := $(VENV)/bin/mkdocs

$(MKDOCS): requirements.txt
	python3 -m venv $(VENV)
	$(PIP) install -r requirements.txt

.PHONY: venv
venv: $(MKDOCS)

.PHONY: serve
serve: venv
	$(MKDOCS) serve -a 127.0.0.1:8000

.PHONY: build
build: venv
	$(MKDOCS) build

.PHONY: build-containerized
build-containerized:
	$(RUNTIME) volume create --ignore hypershift-docs-site
	$(RUNTIME) run --rm -it \
		-v ${PWD}:/home/docs/hypershift:Z \
		-v hypershift-docs-site:/home/docs/hypershift/site \
		$(IMG) build --strict

.PHONY: serve-containerized
serve-containerized:
	$(RUNTIME) run --rm -it -p 8000:8000 \
		-v ${PWD}:/home/docs/hypershift:Z \
		-v hypershift-docs-site:/home/docs/hypershift/site \
		$(IMG) serve --dev-addr 0.0.0.0:8000

.PHONY: image
image:
	$(RUNTIME) build -t $(IMG) .

.PHONY: push
push:
        $(RUNTIME) push $(IMG)
