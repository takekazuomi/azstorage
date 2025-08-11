package blob

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

const (
	// DevAccountName はAzureローカル開発ストレージ(Azurite)のデフォルトアカウント名
	DevAccountName = "devstoreaccount1"
	// DevAccountKey はAzureローカル開発ストレージ(Azurite)のデフォルトアカウントキー
	DevAccountKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
)

// clientOptions は内部設定構造体
type clientOptions struct {
	insecureSkipVerify bool
}

// Option はNewClient関数のオプション関数型
type Option func(*clientOptions)

// WithInsecureSkipVerify 自己署名証明書でのHTTPS接続を許可（開発環境用）
func WithInsecureSkipVerify() Option {
	return func(opts *clientOptions) {
		opts.insecureSkipVerify = true
	}
}

// SASOptions はSAS生成のオプション
type SASOptions struct {
	// Permissions SAS権限設定
	Permissions sas.BlobPermissions
	// ExpiryDuration SAS有効期限（デフォルト: 1時間）
	ExpiryDuration time.Duration
	// UseServiceSAS Service SAS強制使用（デフォルト: false = User Delegation SAS優先）
	UseServiceSAS bool
	// AccountKey Service SAS用のアカウントキー（UseServiceSAS=true時必須）
	AccountKey string
	// AccountName Service SAS用のアカウント名（UseServiceSAS=true時必須）
	AccountName string
}

// Client はAzure Blob Storage拡張クライアント
type Client struct {
	*azblob.Client
	isAzurite       bool
	azuriteDefaults *SASOptions
}

// IsAzurite はAzurite環境かどうかを返す
func (c *Client) IsAzurite() bool {
	return c.isAzurite
}

// ApplyDefaults はAzurite環境の場合デフォルト値を設定
func (c *Client) ApplyDefaults(opts *SASOptions) {
	if c.isAzurite && c.azuriteDefaults != nil {
		if opts.AccountKey == "" && opts.AccountName == "" {
			opts.AccountKey = c.azuriteDefaults.AccountKey
			opts.AccountName = c.azuriteDefaults.AccountName
		}
	}
}

// isAzuriteEnvironment はURLからAzurite環境かどうかを判定
func isAzuriteEnvironment(url string) bool {
	return strings.Contains(url, "localhost") ||
		strings.Contains(url, "127.0.0.1") ||
		strings.Contains(url, DevAccountName)
}

// NewClient はDefaultAzureCredentialを使用してAzure Blob Storageクライアントを作成
//
// DefaultAzureCredentialは以下の順序で認証を試行：
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
func NewClient(ctx context.Context, blobURL string, options ...Option) (*Client, error) {
	// オプション適用
	opts := &clientOptions{}
	for _, option := range options {
		option(opts)
	}

	// DefaultAzureCredential取得
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("認証情報作成失敗: %w", err)
	}

	// ClientOptions設定
	var clientOpts *azblob.ClientOptions
	if opts.insecureSkipVerify {
		// 自己署名証明書を許可するHTTPクライアント設定
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		clientOpts = &azblob.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: httpClient,
			},
		}
	}

	// azblob.Client作成
	azClient, err := azblob.NewClient(blobURL, cred, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("クライアント作成失敗: %w", err)
	}

	// Enhanced機能付きClientとして返す
	client := &Client{
		Client:    azClient,
		isAzurite: isAzuriteEnvironment(azClient.URL()),
	}

	// Azurite環境の場合デフォルト値設定
	if client.isAzurite {
		client.azuriteDefaults = &SASOptions{
			AccountName: DevAccountName,
			AccountKey:  DevAccountKey,
		}
	}

	return client, nil
}
