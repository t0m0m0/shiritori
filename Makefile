.PHONY: build clean stop start restart test build-frontend

build-frontend:
	cd frontend && npm run build

build: build-frontend
	go build -o shiritori-server ./cmd/srv

clean:
	rm -f shiritori-server
	rm -rf srv/static/dist

test:
	go test ./...
