# Azure Storage開発用Makefile

# Azurite環境設定
AZURITE_CERT_PATH ?= ./certs/server.pem
AZURITE_KEY_PATH ?= ./certs/server-key.pem

# Docker Compose設定
COMPOSE_PROJECT_NAME ?= azstorage
COMPOSE_FILE ?= docker-compose.yml

# HTTP/HTTPS環境分離設定
AZURITE_HTTP_PORT ?= 20000
AZURITE_HTTPS_PORT ?= 21000
AZURITE_HTTP_SERVICE ?= azurite-http
AZURITE_HTTPS_SERVICE ?= azurite-https
AZURITE_HTTP_DATA_PATH ?= ./data/azurite-http
AZURITE_HTTPS_DATA_PATH ?= ./data/azurite-https

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
GOLANGCI_LINT := $(GOBIN)/golangci-lint

.PHONY: help deps azurite-start azurite-stop azurite-restart azurite-clean azurite-certs azurite-logs azure-check azure-set-context azure-create-rg azure-generate-name azure-create-storage-cli azure-create-storage azure-setup-msi azure-setup-user azure-setup azure-info azure-delete test-azurite test-azure test-all lint lint-fix poc1-azurite poc1-azure poc1-all

help: ## ヘルプ表示
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

$(MKCERT): ## mkcertインストール
	@mkdir -p $(GOBIN)
	@GOBIN=$(GOBIN) go install filippo.io/mkcert@v1.4.4 2>/dev/null || echo "Error: mkcertインストール失敗"

$(GOLANGCI_LINT): ## golangci-lint v2.4.0インストール（公式推奨バイナリ方式）
	@mkdir -p $(GOBIN)
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(GOBIN) v2.4.0 2>/dev/null || echo "Error: golangci-lint v2.4.0インストール失敗"

deps: $(MKCERT) $(GOLANGCI_LINT) ## 依存ツールのインストール


azurite-clean: ## 全Azuriteデータ・証明書削除
	@rm -rf $(AZURITE_HTTP_DATA_PATH) $(AZURITE_HTTPS_DATA_PATH) 2>/dev/null || true
	@rm -f certs/* 2>/dev/null || true

# 証明書ファイルが存在しない場合のみ生成
$(AZURITE_CERT_PATH) $(AZURITE_KEY_PATH): $(MKCERT)
	@mkdir -p $(dir $(AZURITE_CERT_PATH))
	@$(MKCERT) -key-file $(AZURITE_KEY_PATH) -cert-file $(AZURITE_CERT_PATH) localhost 127.0.0.1 2>/dev/null || echo "Error: 証明書生成失敗"
	@cp $$(mkcert -CAROOT)/rootCA.pem ./certs/ 2>/dev/null || echo "Error: CA証明書コピー失敗"

azurite-certs: $(AZURITE_CERT_PATH) $(AZURITE_KEY_PATH) ## 自己署名証明書生成（mkcert使用）


# Azurite統合操作（Docker Compose版）
azurite-start: azurite-certs ## Azuriteコンテナ起動（HTTP/HTTPS両環境）
	@mkdir -p $(AZURITE_HTTP_DATA_PATH) $(AZURITE_HTTPS_DATA_PATH)
	@docker compose up -d 2>/dev/null || echo "Error: Azurite起動失敗"

azurite-stop: ## Azuriteコンテナ停止（HTTP/HTTPS両環境）
	@docker compose down 2>/dev/null || true

azurite-restart: azurite-stop azurite-start ## Azuriteコンテナ再起動（HTTP/HTTPS両環境）

azurite-logs: ## Azuriteコンテナのログ表示（両環境）
	@docker compose logs -f 2>/dev/null || echo "Error: Azuriteコンテナが起動していません"

# Azure Storage Account管理
azure-check: ## Azure CLI設定確認
	@az account show --query '{subscriptionId:id,tenantId:tenantId,user:user.name}' --output table 2>/dev/null || echo "Error: Azure CLI未ログイン。'az login'を実行してください"

azure-set-context: ## Azure Subscription/Tenant設定
	@if [ -n "$(AZURE_SUBSCRIPTION_ID)" ]; then \
		az account set --subscription $(AZURE_SUBSCRIPTION_ID) 2>/dev/null || echo "Error: Subscription設定失敗"; \
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

# 動作確認ターゲット
test-azurite: ## Azurite環境でテスト実行
	@go test -v ./... -tags=azurite || echo "Error: Azuriteテスト失敗"

test-azure: ## Azure環境でテスト実行
	@go test -v ./... -tags=azure || echo "Error: Azureテスト失敗"

test-all: test-azurite test-azure ## 全環境でテスト実行

# コード品質チェック
lint: $(GOLANGCI_LINT) ## golangci-lintでコード品質チェック
	@$(GOLANGCI_LINT) run ./... || echo "Error: lint失敗"

lint-fix: $(GOLANGCI_LINT) ## golangci-lintで修正可能な問題を自動修正
	@$(GOLANGCI_LINT) run --fix ./... || echo "Error: lint修正失敗"

poc1-azurite: azurite-start ## POC1 Azurite環境で実行（AZURITE_HTTP環境変数で制御）
	@AZURITE_HTTP=${AZURITE_HTTP} USE_AZURE=false go run example/poc1/main.go || echo "Error: POC1 Azurite実行失敗"

poc1-azure: ## POC1 Azure環境で実行
	@USE_AZURE=true go run example/poc1/main.go || echo "Error: POC1 Azure実行失敗"

poc1-all: poc1-azurite poc1-azure ## POC1 全環境で実行