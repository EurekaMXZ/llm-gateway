.PHONY: compose-up compose-down compose-logs compose-ps check-state test-go m2-smoke m3-smoke

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f --tail=200

compose-ps:
	docker compose ps

check-state:
	scripts/agent/check_state.sh

test-go:
	@set -e; \
	for d in backend/packages/platform backend/services/*; do \
		echo "==> $$d"; \
		(cd $$d && go test ./...); \
	done

m2-smoke:
	scripts/agent/m2_closeout_smoke.sh

m3-smoke:
	scripts/agent/m3_closeout_smoke.sh
