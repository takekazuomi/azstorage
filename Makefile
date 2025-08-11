# Azure Storage開発用Makefile

# 環境変数のデフォルト値設定
AZURITE_BLOB_PORT ?= 10000
AZURITE_QUEUE_PORT ?= 10001
AZURITE_TABLE_PORT ?= 10002
AZURITE_DATA_PATH ?= ./data/azurite
AZURITE_CONTAINER_NAME ?= azurite-dev
AZURITE_CERT_PATH ?= ./certs/server.pem
AZURITE_KEY_PATH ?= ./certs/server-key.pem

# Go tools設定
GOBIN := $(PWD)/tmp/bin
MKCERT := $(GOBIN)/mkcert

.PHONY: help deps azurite-start azurite-stop azurite-logs azurite-clean azurite-certs

help: ## ヘルプ表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

$(MKCERT): ## mkcertインストール
	@mkdir -p $(GOBIN)
	@GOBIN=$(GOBIN) go install filippo.io/mkcert@v1.4.4 || echo "Error: mkcertインストール失敗"

deps: $(MKCERT) ## 依存ツールのインストール

azurite-start: $(AZURITE_CERT_PATH) $(AZURITE_KEY_PATH) ## Azuriteコンテナ起動
	@mkdir -p $(AZURITE_DATA_PATH)
	@mkdir -p $(dir $(AZURITE_CERT_PATH))
	@docker rm -f $(AZURITE_CONTAINER_NAME) 2>/dev/null || true
	@docker run -d \
		--name $(AZURITE_CONTAINER_NAME) \
		-p $(AZURITE_BLOB_PORT):10000 \
		-p $(AZURITE_QUEUE_PORT):10001 \
		-p $(AZURITE_TABLE_PORT):10002 \
		-v $(PWD)/$(AZURITE_DATA_PATH):/data \
		-v $(PWD)/$(AZURITE_CERT_PATH):/certs/server.pem \
		-v $(PWD)/$(AZURITE_KEY_PATH):/certs/server-key.pem \
		mcr.microsoft.com/azure-storage/azurite:3.35.0 \
		azurite --blobHost 0.0.0.0 --queueHost 0.0.0.0 --tableHost 0.0.0.0 --location /data --cert /certs/server.pem --key /certs/server-key.pem --debug /data/debug.log || echo "Error: Azurite起動失敗"

azurite-stop: ## Azuriteコンテナ停止・削除
	@docker stop $(AZURITE_CONTAINER_NAME) 2>/dev/null || echo "Warning: コンテナ停止失敗または既に停止済み"
	@docker rm $(AZURITE_CONTAINER_NAME) 2>/dev/null || echo "Warning: コンテナ削除失敗または存在しない"

azurite-logs: ## Azuriteコンテナのログ表示
	@docker logs -f $(AZURITE_CONTAINER_NAME) || echo "Error: ログ取得失敗 - コンテナが存在しない可能性"

azurite-clean: ## Azuriteデータ削除
	@rm -rf $(AZURITE_DATA_PATH) || echo "Error: データ削除失敗"
	@rm certs/* || echo "Error: データ削除失敗"

# 証明書ファイルが存在しない場合のみ生成
$(AZURITE_CERT_PATH) $(AZURITE_KEY_PATH): $(MKCERT)
	@mkdir -p $(dir $(AZURITE_CERT_PATH))
	@$(MKCERT) -key-file $(AZURITE_KEY_PATH) -cert-file $(AZURITE_CERT_PATH) localhost 127.0.0.1 || echo "Error: 証明書生成失敗"
	@cp $$(mkcert -CAROOT)/rootCA.pem ./certs/ || echo "Error: CA証明書コピー失敗"

azurite-certs: $(AZURITE_CERT_PATH) $(AZURITE_KEY_PATH) ## 自己署名証明書生成（mkcert使用）

azurite-restart: azurite-stop azurite-start ## Azuriteコンテナ再起動