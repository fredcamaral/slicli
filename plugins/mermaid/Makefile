PLUGIN_NAME := mermaid
OUTPUT := $(PLUGIN_NAME).so

.PHONY: build
build:
	go build -buildmode=plugin -o $(OUTPUT) .

.PHONY: test
test:
	go test -v ./...

.PHONY: install
install: build
	mkdir -p ~/.config/slicli/plugins
	cp $(OUTPUT) ~/.config/slicli/plugins/

.PHONY: clean
clean:
	rm -f $(OUTPUT)