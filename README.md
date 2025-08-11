# github.com/takekazu/azstorage

Azure Storage のシンプルなラッパー

1. Azurite 開発ストレージとAzure Storageアカウントの統合
2. Azure開発でのMSIサポート

## 開発環境セットアップ

### Azurite起動

```bash
# azurite 起動（証明書自動生成）
make azurite-start
```

### 証明書生成

```bash
# mkcertを使用した自己署名証明書生成
make azurite-certs
```

#### mkcert採用理由

- **ゼロ設定**: 複雑な設定ファイル不要
- **自動信頼**: ブラウザが自動的に証明書を信頼
- **SAN対応**: Subject Alternative Names対応（非推奨のCommon Name使用なし）
- **開発特化**: ローカル開発環境に最適化

## OAuth認証とUser Delegation SAS対応

Azuriteは`--oauth basic`により**基本的なOAuth認証シミュレーション**とUser Delegation SAS生成機能を提供。

### User Delegation SAS対応

**Azurite v3.21.0+**: User Delegation SAS生成に完全対応

- ✅ **User Delegation Key取得**: GetUserDelegationCredential API
- ✅ **User Delegation SAS生成**: SignWithUserDelegation
- ✅ **Service SAS自動フォールバック**: User Delegation失敗時の代替手段
- 🔄 **統一インターフェース**: Azure Storage/Azurite環境切り替え対応

### 認証方式

#### SharedKey認証（従来）

```bash
# 接続文字列による認証
DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=...
```

#### OAuth認証（推奨）

```bash
# DefaultAzureCredentialによるBearer token認証
USE_AZURE=false go run example/poc1/main.go
```

### OAuth機能詳細

#### 検証される項目

- ✅ **Bearer Token存在確認**
- ✅ **Issuer（発行者）検証**
- ✅ **Audience（対象者）検証**
- ✅ **Expiry（有効期限）検証**

#### 制限事項

- ❌ **Token署名検証なし**
- ❌ **権限・Permission検証なし**
- ⚠️ **開発環境用途のみ**

### DefaultAzureCredentialとの統合

```go
// Azurite/Azure統一認証コード
cred, err := azidentity.NewDefaultAzureCredential(nil)
client, err := azblob.NewClient(blobURL, cred, nil)
```

**認証チェーン**:

1. **Azure CLI** (`az login`済みの場合)
2. **MSI** (Azure環境の場合)
3. **その他認証方式** (順次試行)

### 環境切り替え

```bash
# Azurite User Delegation SAS生成（v3.21.0+対応）
USE_AZURE=false go run example/poc1/main.go

# Azure Storage User Delegation SAS生成
USE_AZURE=true go run example/poc1/main.go
```

同じ認証・SAS生成コードでローカル開発・本番環境の両方対応。

### HTTPS要件

**重要**: AzuriteでOAuth認証を使用する場合、**HTTPS必須**。

- OAuth認証には証明書が必要
- HTTPでは`Cannot use TokenCredential without HTTPS`エラー発生
- 現在の実装は`mkcert`で自己署名証明書使用により対応済み

**参考**:

- [Azurite OAuth Authentication - Microsoft Learn](https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite#oauth-authentication)
- [Azure/Azurite - GitHub](https://github.com/Azure/Azurite#oauth)

## WindowsからのAzure Storage Explorer接続

### HTTPS接続設定

WindowsのAzure Storage ExplorerからWSL2上のAzuriteにHTTPS接続する場合の設定：

#### 方法1: 起動時オプション（推奨）

```bash
# Azure Storage Explorerを--ignore-certificate-errorsオプション付きで起動
"C:\Program Files\Microsoft Azure Storage Explorer\StorageExplorer.exe" --ignore-certificate-errors
```

#### 方法2: rootCA証明書インポート（推奨）

1. **WSL2からrootCA証明書をコピー**:

   ```bash
   # rootCA証明書の場所確認
   $(PWD)/tmp/bin/mkcert -CAROOT
   
   # Windowsデスクトップにコピー
   cp ~/.local/share/mkcert/rootCA.pem /mnt/c/Users/[username]/Desktop/
   ```

2. **Windows証明書ストアにインポート**:

   - `Win+R` → `certmgr.msc` で証明書管理画面を開く
   - 「信頼されたルート証明機関」→「証明書」を右クリック
   - 「すべてのタスク」→「インポート」を選択
   - デスクトップの `rootCA.pem` を選択してインポート

3. **Azure Storage Explorerで接続**:
   - 通常の接続手順で自動的に証明書が信頼される
   - 追加設定不要

**注意**: server.pemではなく、rootCA.pemをインポートすることが重要

#### 接続文字列例

```text
DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=https://localhost:20000/devstoreaccount1;QueueEndpoint=https://localhost:20001/devstoreaccount1;TableEndpoint=https://localhost:20002/devstoreaccount1;
```

### よくある問題

**Error: "UNABLE_TO_VERIFY_LEAF_SIGNATURE":**

- 原因: server.pem（リーフ証明書）のみをインポートした場合
- 解決: rootCA.pem（ルート証明書）をインポートする
- mkcertはrootCA → server.pemの証明書チェーンを作成するため、rootCAの信頼が必要

### 注意事項

- `--ignore-certificate-errors`はローカル開発環境でのみ使用
- 本番環境では決して使用しない

## 参考リンク

### 公式ドキュメント

- [Use Azurite emulator - Microsoft Learn](https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite)
- [Azure/Azurite - GitHub](https://github.com/Azure/Azurite)

### OAuth認証・User Delegation SAS関連

- [Azurite OAuth Support](https://learn.microsoft.com/en-us/azure/storage/common/storage-use-azurite#oauth-authentication)
- [User Delegation SAS](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blob-user-delegation-sas-create-cli)
- [Azurite v3.21.0+ User Delegation SAS対応](https://github.com/Azure/Azurite/issues/656)
- [DefaultAzureCredential Documentation](https://learn.microsoft.com/en-us/dotnet/api/azure.identity.defaultazurecredential)

### 証明書設定

- [Azure Storage Explorer証明書問題対応](https://github.com/Microsoft/AzureStorageExplorer/issues/8593)
