# go.mod требует Go >= 1.25, а автоскачивание тулчейна в этом окружении
# недоступно, поэтому по умолчанию берём установленный Go 1.25, если он есть
# (имя каталога go-1.23 в Debian не соответствует реальной версии).
GO      ?= $(or $(shell command -v /usr/lib/go-1.23/bin/go 2>/dev/null),go)
BINARY  ?= s2t_gguf_server
OUT     ?= .
SCRIPTS := scripts

.PHONY: all build run test vet model clean help

all: build

## build — собрать бинарь сервера в $(OUT)/$(BINARY)
build:
	$(GO) build -o $(OUT)/$(BINARY) ./cmd/app

## run — собрать и запустить сервер (предварительно скачивает модель)
run: model build
	./$(OUT)/$(BINARY)

## test — прогнать тесты
test:
	$(GO) test ./...

## vet — статическая проверка кода
vet:
	$(GO) vet ./...

## model — скачать GGUF-модель (scripts/download-model.sh)
model:
	./$(SCRIPTS)/download-model.sh

## clean — удалить собранный бинарь
clean:
	rm -f $(OUT)/$(BINARY)

## help — показать доступные цели
help:
	@grep -hE '^## ' $(MAKEFILE_LIST) | sed 's/^## //'
