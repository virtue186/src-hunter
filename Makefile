build-web:
	go build -o ./bin/web ./cmd/web

run-web:build-web
	./bin/web

build-worker:
	go build -o ./bin/worker ./cmd/worker

run-worker:build-worker
	./bin/worker

test:
	go test ./...

clean-redis:
	@echo "清理 Redis DB 0 @ localhost:6379"
	redis-cli -h localhost -p 6379 -n 0 FLUSHDB