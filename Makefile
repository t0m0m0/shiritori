.PHONY: build clean stop start restart test

build:
	go build -o shiritori-server ./cmd/srv

clean:
	rm -f shiritori-server

test:
	go test ./...
