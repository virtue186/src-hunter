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