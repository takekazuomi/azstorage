# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

Azure Storage のシンプルなラッパーライブラリ。Go言語で実装され、以下の機能を提供：
1. Azurite 開発ストレージとAzure Storageアカウントの統合
2. Azure開発でのMSI（Managed Service Identity）サポート

## 開発環境構成

- 言語: Go 1.24.2
- モジュール: github.com/takekazu/azstorage
- プロジェクト構造: 現在は初期段階でGoコードファイルなし、開発準備中

## 開発コマンド

現在Makefileは空のため、標準的なGoコマンドを使用：

- ビルド: `go build ./...`
- テスト: `go test ./...`
- リンター: `go vet ./...` または `golangci-lint run`（設定済みの場合）
- モジュール管理: `go mod tidy`

## 今後の開発予定

Azure Storage関連の機能実装に向け、以下のディレクトリ構成を想定：
- `blob/`: Azure Blob Storage関連機能
- その他Azure Storage サービス（Queue, Table, File）対応予定

## 注意事項

- Azure認証情報をコードにハードコーディングしない
- 開発環境ではAzurite使用を推奨
- 本番環境ではMSI認証を活用