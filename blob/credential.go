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
	credential interface{} // azcore.TokenCredential or *azblob.SharedKeyCredential
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
		return nil, fmt.Errorf("Account Key認証情報作成失敗: %w", err)
	}
	return &UnifiedCredential{credential: cred}, nil
}

// NewCredential は環境に応じて適切な認証方式を自動選択してUnifiedCredentialを作成
func NewCredential(ctx context.Context, blobURL string) (*UnifiedCredential, error) {
	if IsAzuriteEnvironment(blobURL) {
		if strings.HasPrefix(blobURL, "http://") {
			// HTTP Azurite: Account Key認証
			return NewAccountKeyCredential(DevAccountName, DevAccountKey)
		}
		// HTTPS Azurite: OAuth認証
		return NewOAuthCredential()
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