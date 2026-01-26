APP_NAME = sample-app

build:
	go mod vendor
	go generate ./...
	CGO_ENABLED=0 go build -o ./bin/app $(buildpath)

run: build
	./bin/app

image:
	docker build -t $(APP_NAME):latest .

clean:
	rm -f ./bin/app

test:
	go test -v ./...
