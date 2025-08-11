# Azure Storage開発用Makefile

# 環境変数のデフォルト値設定
AZURITE_BLOB_PORT ?= 10000
AZURITE_QUEUE_PORT ?= 10001
AZURITE_TABLE_PORT ?= 10002
AZURITE_DATA_PATH ?= ./data/azurite
AZURITE_CONTAINER_NAME ?= azurite-dev
AZURITE_CERT_PATH ?= ./certs/server.pem
AZURITE_KEY_PATH ?= ./certs/server-key.pem

# Azure設定
AZURE_RESOURCE_GROUP ?= azstorage-dev
AZURE_LOCATION ?= japaneast
AZURE_STORAGE_ACCOUNT_NAME ?= azstoragedev$(shell whoami)
AZURE_SUBSCRIPTION_ID ?= $(shell az account show --query id -o tsv 2>/dev/null || echo "")
AZURE_TENANT_ID ?= $(shell az account show --query tenantId -o tsv 2>/dev/null || echo "")
AZURE_USER_OBJECT_ID ?= $(shell az ad signed-in-user show --query id -o tsv 2>/dev/null || echo "")

# Go tools設定
GOBIN := $(PWD)/tmp/bin
MKCERT := $(GOBIN)/mkcert

.PHONY: help deps azurite-start azurite-stop azurite-logs azurite-clean azurite-certs azure-check azure-set-context azure-create-rg azure-generate-name azure-create-storage-cli azure-create-storage azure-setup-msi azure-setup-user azure-setup azure-info azure-delete

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
		azurite --blobHost 0.0.0.0 --queueHost 0.0.0.0 --tableHost 0.0.0.0 --location /data --cert /certs/server.pem --key /certs/server-key.pem --oauth basic --debug /data/debug.log || echo "Error: Azurite起動失敗"

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

# Azure Storage Account管理
azure-check: ## Azure CLI設定確認
	@az account show --query '{subscriptionId:id,tenantId:tenantId,user:user.name}' --output table 2>/dev/null || echo "Error: Azure CLI未ログイン。'az login'を実行してください"
	@echo "Environment variables:"
	@echo "  AZURE_SUBSCRIPTION_ID=$(AZURE_SUBSCRIPTION_ID)"
	@echo "  AZURE_TENANT_ID=$(AZURE_TENANT_ID)"
	@echo "  AZURE_USER_OBJECT_ID=$(AZURE_USER_OBJECT_ID)"

azure-set-context: ## Azure Subscription/Tenant設定
	@if [ -n "$(AZURE_SUBSCRIPTION_ID)" ]; then \
		az account set --subscription $(AZURE_SUBSCRIPTION_ID) 2>/dev/null || echo "Error: Subscription設定失敗"; \
	fi
	@if [ -n "$(AZURE_TENANT_ID)" ]; then \
		echo "Current Tenant: $(AZURE_TENANT_ID)"; \
	fi

azure-create-rg: ## Azure Resource Group作成
	@az group create \
		--name $(AZURE_RESOURCE_GROUP) \
		--location $(AZURE_LOCATION) 2>/dev/null || echo "Error: Resource Group作成失敗"

azure-generate-name: ## Storage Account名生成（Bicep + uniqueString）
	@mkdir -p templates
	@RG_ID="/subscriptions/$(AZURE_SUBSCRIPTION_ID)/resourceGroups/$(AZURE_RESOURCE_GROUP)" && \
	az deployment group create \
		--resource-group $(AZURE_RESOURCE_GROUP) \
		--template-file templates/generate-name.bicep \
		--parameters resourceGroupId="$$RG_ID" \
		--query 'properties.outputs.storageAccountName.value' \
		--output tsv > .azure-storage-name 2>/dev/null || echo "Error: 名前生成失敗"

azure-create-storage-cli: ## 生成済み名前でStorage Account作成
	@GENERATED_NAME=$$(cat .azure-storage-name 2>/dev/null) && \
	az storage account create \
		--name $$GENERATED_NAME \
		--resource-group $(AZURE_RESOURCE_GROUP) \
		--location $(AZURE_LOCATION) \
		--sku Standard_LRS \
		--assign-identity \
		--allow-blob-public-access false \
		--https-only true \
		--min-tls-version TLS1_2 2>/dev/null || echo "Error: Storage Account作成失敗"

azure-create-storage: azure-generate-name azure-create-storage-cli ## Azure Storage Account作成（uniqueString使用）

azure-setup-msi: ## MSI権限設定
	@GENERATED_NAME=$$(cat .azure-storage-name 2>/dev/null) && \
	IDENTITY_PRINCIPAL_ID=$$(az storage account show \
		--name $$GENERATED_NAME \
		--resource-group $(AZURE_RESOURCE_GROUP) \
		--query identity.principalId \
		--output tsv 2>/dev/null) && \
	az role assignment create \
		--assignee $$IDENTITY_PRINCIPAL_ID \
		--role "Storage Blob Data Contributor" \
		--scope "/subscriptions/$(AZURE_SUBSCRIPTION_ID)/resourceGroups/$(AZURE_RESOURCE_GROUP)" 2>/dev/null || echo "Error: MSI設定失敗"

azure-setup-user: ## Azure CLI実行ユーザーにStorage Account権限付与
	@az role assignment create \
		--assignee $(AZURE_USER_OBJECT_ID) \
		--role "Storage Blob Data Contributor" \
		--scope "/subscriptions/$(AZURE_SUBSCRIPTION_ID)/resourceGroups/$(AZURE_RESOURCE_GROUP)" 2>/dev/null || echo "Error: ユーザー権限設定失敗"

azure-setup: azure-set-context azure-create-rg azure-create-storage azure-setup-msi azure-setup-user ## Azure環境完全セットアップ（MSI+ユーザー権限）

azure-info: ## Azure Storage Account情報表示
	@GENERATED_NAME=$$(cat .azure-storage-name 2>/dev/null) && \
	az storage account show \
		--name $$GENERATED_NAME \
		--resource-group $(AZURE_RESOURCE_GROUP) \
		--query '{name:name,location:location,identity:identity.principalId}' \
		--output table 2>/dev/null || echo "Error: Storage Account情報取得失敗"

azure-delete: ## Azure Storage Account削除
	@GENERATED_NAME=$$(cat .azure-storage-name 2>/dev/null) && \
	read -p "Storage Account '$$GENERATED_NAME' を削除しますか? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	@GENERATED_NAME=$$(cat .azure-storage-name 2>/dev/null) && \
	az storage account delete \
		--name $$GENERATED_NAME \
		--resource-group $(AZURE_RESOURCE_GROUP) \
		--yes 2>/dev/null || echo "Error: Storage Account削除失敗"