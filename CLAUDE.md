# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

Azure Storage のシンプルなラッパーライブラリ。Go言語で実装され、以下の機能を提供：

1. Azurite 開発ストレージとAzure Storageアカウントの統合
2. Azure開発でのMSI（Managed Service Identity）サポート
3. User Delegation SAS / Service SAS生成機能
4. 環境別認証方式自動選択（OAuth/Account Key）
5. HTTP/HTTPS両環境対応

## ローカル開発環境構成

- 言語: Go 1.24.2
- モジュール: github.com/takekazu/azstorage
- Azurite: 3.35.0 (HTTPS証明書付き)
- 証明書ツール: mkcert v1.4.4 (自動インストール)
- 環境変数管理: direnv (.envrc)

## 開発コマンド

### Azurite環境

- 依存ツール準備: `make deps`
- 証明書生成: `make azurite-certs`
- Azurite起動: `make azurite-start` (HTTPS固定)
- Azurite停止: `make azurite-stop`
- ログ確認: `make azurite-logs`
- データ削除: `make azurite-clean`

### Go開発

- ビルド: `go build ./...`
- テスト: `go test ./...`
- リンター: `go vet ./...`
- モジュール管理: `go mod tidy`

### POC動作確認

- HTTPS環境: `make poc1-azurite`
- HTTP環境: `AZURITE_HTTP=true make poc1-azurite`
- Azure環境: `make poc1-azure`
- 全環境テスト: `make poc1-all`

## 証明書とHTTPS設定

### 証明書管理

- mkcertによる自己署名証明書生成
- rootCA証明書: ~/.local/share/mkcert/rootCA.pem
- サーバー証明書: ./certs/server.pem, ./certs/server-key.pem

### Windows接続時の注意

- Azure Storage Explorerは`--ignore-certificate-errors`オプション推奨
- または rootCA.pem をWindows証明書ストアにインポート

## プロジェクト構造

```text
.
├── Makefile              # Azurite開発環境管理
├── .envrc               # direnv環境変数設定
├── go.mod               # Goモジュール定義
├── tmp/bin/             # ローカルツール（mkcert等）
├── certs/               # SSL証明書
├── data/azurite/        # Azuriteデータ保存
├── blob/                # Azure Blob Storage機能
│   ├── client.go        # 拡張クライアント（埋め込みパターン）
│   └── generateSAS.go   # SAS生成機能
└── example/poc1/        # POCサンプル
```

## アーキテクチャ

### Client構造体（埋め込みパターン）

```go
type Client struct {
    *azblob.Client        // 埋め込みによるメソッドプロモーション
    isAzurite       bool  // 環境判定
    azuriteDefaults *SASOptions  // デフォルト値
}
```

### 環境別認証方式

- **Azure**: OAuth認証（DefaultAzureCredential）
- **Azurite HTTPS**: OAuth認証 + InsecureSkipVerify自動適用
- **Azurite HTTP**: Account Key認証のみ

### 今後の開発予定

Azure Storage関連の機能実装に向け、以下のディレクトリ構成を想定：

- `blob/`: Azure Blob Storage関連機能（実装済み）
- その他Azure Storage サービス（Queue, Table, File）対応予定

## 開発ルール

### Makefileコーディング規約

- **静粛実行**: 標準出力は最小限に抑制、エラー時のみ出力
- **標準出力を`/dev/null`へ**: 成功時の出力は`2>/dev/null`でリダイレクト
- **エラーメッセージ**: 失敗時は`|| echo "Error: ..."`で明確なエラー表示
- **冗長な進捗表示禁止**: `echo "=== 〜実行中 ==="`などの実行状況表示は削除

例：

```makefile
# 良い例
target: ## ターゲット説明
 @command 2>/dev/null || echo "Error: command失敗"

# 悪い例
target: ## ターゲット説明
 @echo "=== command実行中 ==="
 @command
 @echo "command完了"
```

## 注意事項

- Azure認証情報をコードにハードコーディングしない
- 開発環境ではAzurite使用を推奨
- 本番環境ではMSI認証を活用
