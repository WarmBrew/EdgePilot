.PHONY: dev build test docker-up docker-down clean

dev:
	cd server && go run ./cmd/server

test:
	cd server && go test -race ./...
	cd agent && go test -race ./...

build:
	cd server && go build -o dist/server ./cmd/server
	cd agent && make cross-compile

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

clean:
	rm -rf server/dist agent/dist web/dist
