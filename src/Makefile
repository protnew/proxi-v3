.PHONY: all build build-frontend build-backend run test clean docker-build

APP_NAME = unkillable-messenger
PORT ?= 9999

all: build

build-frontend:
	cd frontend && npm install && npm run build

build-backend:
	cd src-vpn && go build -o ../webserver ./cmd/webserver

build: build-frontend build-backend

run: build
	DIST_DIR=./frontend/dist PORT=$(PORT) ./webserver

test:
	cd src-vpn && go test -v ./...

clean:
	rm -f webserver
	rm -rf frontend/dist
	rm -rf frontend/node_modules

docker-build:
	docker build -t $(APP_NAME) .

docker-run:
	docker run -p $(PORT):$(PORT) -p 51820:51820/udp --rm $(APP_NAME)
