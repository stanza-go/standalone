.PHONY: dev dev-api dev-ui dev-admin build build-ui build-admin build-api docker clean

# Development — run all three services with hot reload
dev:
	@echo "Starting Stanza development servers..."
	@$(MAKE) -j3 dev-api dev-ui dev-admin

dev-api:
	@echo "API server on :23710"
	cd api && go run .

dev-ui:
	@echo "UI on :23700"
	cd ui && bun run dev

dev-admin:
	@echo "Admin on :23706"
	cd admin && bun run dev

# Build — produce a single binary with embedded frontends
build: build-ui build-admin build-api
	@echo "Build complete: api/bin/standalone"

build-ui:
	cd ui && bun run build

build-admin:
	cd admin && bun run build

build-api:
	mkdir -p api/ui api/admin
	cp -r ui/dist api/ui/dist
	cp -r admin/dist api/admin/dist
	cd api && CGO_ENABLED=1 go build -tags prod \
		-ldflags="-s -w \
			-X main.version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
			-X main.commit=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown) \
			-X main.buildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		-o bin/standalone .

# Docker — build production image from workspace root
docker:
	docker build -t stanza -f Dockerfile \
		--build-arg BUILD_VERSION=$$(cd .. && git -C standalone describe --tags --always --dirty 2>/dev/null || echo dev) \
		--build-arg BUILD_COMMIT=$$(cd .. && git -C standalone rev-parse --short HEAD 2>/dev/null || echo unknown) \
		--build-arg BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
		..

clean:
	rm -rf api/bin api/ui api/admin ui/dist admin/dist
