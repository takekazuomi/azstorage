package blob

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// UnifiedCredential はTokenCredentialとSharedKeyCredentialを統合する構造体
type UnifiedCredential struct {
	credential any // azcore.TokenCredential or *azblob.SharedKeyCredential
}

// GetCredential は内部の認証情報を返す
func (c *UnifiedCredential) GetCredential() interface{} {
	return c.credential
}

// IsTokenCredential はTokenCredentialかどうかを判定
func (c *UnifiedCredential) IsTokenCredential() bool {
	_, ok := c.credential.(azcore.TokenCredential)
	return ok
}

// IsSharedKeyCredential はSharedKeyCredentialかどうかを判定
func (c *UnifiedCredential) IsSharedKeyCredential() bool {
	_, ok := c.credential.(*azblob.SharedKeyCredential)
	return ok
}

// NewOAuthCredential はDefaultAzureCredentialを使用したUnifiedCredentialを作成
func NewOAuthCredential() (*UnifiedCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("OAuth認証情報作成失敗: %w", err)
	}
	return &UnifiedCredential{credential: cred}, nil
}

// NewAccountKeyCredential はSharedKeyCredentialを使用したUnifiedCredentialを作成
func NewAccountKeyCredential(accountName, accountKey string) (*UnifiedCredential, error) {
	cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("account key認証情報作成失敗: %w", err)
	}
	return &UnifiedCredential{credential: cred}, nil
}

// NewCredential は環境に応じて適切な認証方式を選択してUnifiedCredentialを作成
//
// 認証方式の選択ルール
//
// 以下の順序で認証を試行：
//  1. Environment Credential (環境変数)
//  2. Workload Identity Credential
//  3. Managed Identity Credential (Azure環境)
//  4. Azure CLI Credential (az login)
//  5. Azure Developer CLI Credential (azd auth login)
//
// Environment Credential用環境変数：
//   - AZURE_TENANT_ID: テナントID (必須)
//   - AZURE_CLIENT_ID: クライアント(アプリケーション)ID (必須)
//   - AZURE_CLIENT_SECRET: クライアントシークレット (ClientSecret認証用)
//   - AZURE_CLIENT_CERTIFICATE_PATH: 証明書パス (Certificate認証用)
//   - AZURE_USERNAME, AZURE_PASSWORD: ユーザー名/パスワード (UsernamePassword認証用)
//
// 認証チェーン制御用環境変数 (AZURE_TOKEN_CREDENTIALS):
//   - azidentity v1.10.0+: カテゴリ除外
//   - "prod": 開発者ツール認証を除外
//   - "dev": デプロイサービス認証を除外
//   - azidentity v1.11.0+: 特定認証選択
//   - "AzureCLICredential", "EnvironmentCredential", "ManagedIdentityCredential" 等
//
// 参考: https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/credential-chains
func NewCredential(_ context.Context, blobURL string) (*UnifiedCredential, error) {
	if IsAzuriteEnvironment(blobURL) {
		if strings.HasPrefix(blobURL, "http://") {
			// HTTP Azurite: Account Key認証
			return NewAccountKeyCredential(DevAccountName, DevAccountKey)
		}
		// HTTPS Azurite: OAuth認証を利用
	}
	// Azure Storage: OAuth認証
	return NewOAuthCredential()
}

// IsAzuriteEnvironment はURLからAzurite環境かどうかを判定
func IsAzuriteEnvironment(url string) bool {
	// HTTPスキームのAzurite環境
	if strings.HasPrefix(url, "http://") &&
		(strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1")) {
		return true
	}

	// HTTPSスキームのAzurite環境
	if strings.HasPrefix(url, "https://") &&
		(strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1")) {
		return true
	}

	// devstoreaccount1を含む場合（スキーム問わず）
	if strings.Contains(url, DevAccountName) {
		return true
	}

	return false
}
