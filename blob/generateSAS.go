package blob

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)


// DefaultSASOptions はデフォルトのSASオプションを返す
func DefaultSASOptions() *SASOptions {
	return &SASOptions{
		Permissions: sas.BlobPermissions{
			Read:   true,
			Write:  false,
			Delete: false,
		},
		ExpiryDuration: 1 * time.Hour,
		UseServiceSAS:  false,
	}
}


// GenerateBlobSAS はBlobのSAS URLを生成（User Delegation SASまたはService SAS）
// Azurite環境を自動判定し、必要に応じてService SASにフォールバック
func GenerateBlobSAS(ctx context.Context, client *Client, containerName, blobName string, opts *SASOptions) (string, error) {
	if opts == nil {
		opts = DefaultSASOptions()
	}

	// Azurite環境の場合、デフォルト値を適用
	client.ApplyDefaults(opts)

	// HTTP環境でUser Delegation SAS強制使用の場合はエラー
	if client.IsHTTP() && opts.UseUserDelegationSAS {
		return "", fmt.Errorf("User Delegation SASはHTTP環境では利用できません。HTTPS環境を使用するか、UseUserDelegationSAS=falseに設定してService SASを使用してください")
	}

	// HTTP環境またはService SAS強制使用の場合は直接Service SAS生成
	if client.IsHTTP() || opts.UseServiceSAS {
		if client.IsHTTP() {
			fmt.Printf("HTTP環境検出: Service SASのみ対応\n")
		} else {
			fmt.Printf("Service SAS強制使用モード\n")
		}
		return generateServiceSAS(ctx, containerName, blobName, opts)
	}

	// HTTPS環境: User Delegation SAS試行（失敗時はService SASにフォールバック）
	fmt.Printf("User Delegation SAS生成を試行中...\n")
	userDelegationSAS, err := generateUserDelegationSAS(ctx, client.Client, containerName, blobName, opts)
	if err != nil {
		// User Delegation SAS失敗時のログ出力
		fmt.Printf("User Delegation SAS生成失敗、Service SASにフォールバック: %v\n", err)
		
		// Service SASの情報が不足している場合はエラー
		if opts.AccountKey == "" || opts.AccountName == "" {
			return "", fmt.Errorf("User Delegation SAS失敗、Service SAS用の認証情報も不足: %w", err)
		}
		
		return generateServiceSAS(ctx, containerName, blobName, opts)
	}

	fmt.Printf("User Delegation SAS生成成功\n")
	return userDelegationSAS, nil
}

// generateUserDelegationSAS はUser Delegation SASを生成
func generateUserDelegationSAS(ctx context.Context, client *azblob.Client, containerName, blobName string, opts *SASOptions) (string, error) {
	// SAS開始時刻設定（clock skew対応）
	// "Be careful with SAS start time. set the start time to be at least 15 minutes in the past"
	// 参考: https://learn.microsoft.com/en-us/azure/storage/common/storage-sas-overview#sas-token
	start := time.Now().Add(-15 * time.Minute)
	expiry := start.Add(opts.ExpiryDuration)

	// User Delegation Key取得
	startStr := start.UTC().Format(time.RFC3339)
	expiryStr := expiry.UTC().Format(time.RFC3339)
	keyInfo := service.KeyInfo{
		Start:  &startStr,
		Expiry: &expiryStr,
	}

	userDelegationCredential, err := client.ServiceClient().GetUserDelegationCredential(ctx, keyInfo, nil)
	if err != nil {
		return "", fmt.Errorf("User Delegation Key取得失敗: %w", err)
	}

	// SAS URL生成
	sasValues := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     start,
		ExpiryTime:    expiry,
		Permissions:   opts.Permissions.String(),
		ContainerName: containerName,
		BlobName:      blobName,
	}

	sasToken, err := sasValues.SignWithUserDelegation(userDelegationCredential)
	if err != nil {
		return "", fmt.Errorf("User Delegation SAS署名失敗: %w", err)
	}

	// 完全なSAS URLを構築
	blobURL := client.ServiceClient().NewContainerClient(containerName).NewBlobClient(blobName).URL()
	
	return fmt.Sprintf("%s?%s", blobURL, sasToken.Encode()), nil
}

// generateServiceSAS はService SASを生成
func generateServiceSAS(ctx context.Context, containerName, blobName string, opts *SASOptions) (string, error) {
	if opts.AccountKey == "" || opts.AccountName == "" {
		return "", fmt.Errorf("Service SAS生成にはAccountKeyとAccountNameが必要")
	}

	// SAS開始時刻設定（clock skew対応で15分前から開始）
	start := time.Now().Add(-15 * time.Minute)
	expiry := start.Add(opts.ExpiryDuration)

	// Shared Key Credential作成
	credential, err := azblob.NewSharedKeyCredential(opts.AccountName, opts.AccountKey)
	if err != nil {
		return "", fmt.Errorf("SharedKey Credential作成失敗: %w", err)
	}

	// SAS URL生成
	sasValues := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     start,
		ExpiryTime:    expiry,
		Permissions:   opts.Permissions.String(),
		ContainerName: containerName,
		BlobName:      blobName,
	}

	sasToken, err := sasValues.SignWithSharedKey(credential)
	if err != nil {
		return "", fmt.Errorf("Service SAS署名失敗: %w", err)
	}

	// 完全なSAS URLを構築
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net", opts.AccountName)
	blobURL := fmt.Sprintf("%s/%s/%s", serviceURL, containerName, blobName)
	
	return fmt.Sprintf("%s?%s", blobURL, sasToken.Encode()), nil
}

// GenerateContainerSAS はコンテナのSAS URLを生成
func GenerateContainerSAS(ctx context.Context, client *Client, containerName string, opts *SASOptions) (string, error) {
	if opts == nil {
		opts = DefaultSASOptions()
	}

	// Azurite環境の場合、デフォルト値を適用
	client.ApplyDefaults(opts)

	// HTTP環境でUser Delegation SAS強制使用の場合はエラー
	if client.IsHTTP() && opts.UseUserDelegationSAS {
		return "", fmt.Errorf("User Delegation SASはHTTP環境では利用できません。HTTPS環境を使用するか、UseUserDelegationSAS=falseに設定してService SASを使用してください")
	}

	// HTTP環境またはService SAS強制使用の場合は直接Service SAS生成
	if client.IsHTTP() || opts.UseServiceSAS {
		if client.IsHTTP() {
			fmt.Printf("HTTP環境検出: Service SASのみ対応\n")
		} else {
			fmt.Printf("Service SAS強制使用モード\n")
		}
		return generateContainerServiceSAS(ctx, containerName, opts)
	}

	// HTTPS環境: User Delegation SAS試行
	userDelegationSAS, err := generateContainerUserDelegationSAS(ctx, client.Client, containerName, opts)
	if err != nil {
		fmt.Printf("Container User Delegation SAS生成失敗、Service SASを試行: %v\n", err)
		
		if opts.AccountKey == "" || opts.AccountName == "" {
			return "", fmt.Errorf("User Delegation SAS失敗、Service SAS用の認証情報も不足: %w", err)
		}
		
		return generateContainerServiceSAS(ctx, containerName, opts)
	}

	return userDelegationSAS, nil
}

// generateContainerUserDelegationSAS はコンテナ用User Delegation SASを生成
func generateContainerUserDelegationSAS(ctx context.Context, client *azblob.Client, containerName string, opts *SASOptions) (string, error) {
	// SAS開始時刻設定（clock skew対応で15分前から開始）
	start := time.Now().Add(-15 * time.Minute)
	expiry := start.Add(opts.ExpiryDuration)

	startStr := start.UTC().Format(time.RFC3339)
	expiryStr := expiry.UTC().Format(time.RFC3339)
	keyInfo := service.KeyInfo{
		Start:  &startStr,
		Expiry: &expiryStr,
	}

	userDelegationCredential, err := client.ServiceClient().GetUserDelegationCredential(ctx, keyInfo, nil)
	if err != nil {
		return "", fmt.Errorf("User Delegation Key取得失敗: %w", err)
	}

	// コンテナ権限に変換
	containerPermissions := sas.ContainerPermissions{
		Read:   opts.Permissions.Read,
		Write:  opts.Permissions.Write,
		Delete: opts.Permissions.Delete,
		List:   true, // コンテナではList権限も追加
	}

	sasValues := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     start,
		ExpiryTime:    expiry,
		Permissions:   containerPermissions.String(),
		ContainerName: containerName,
	}

	sasToken, err := sasValues.SignWithUserDelegation(userDelegationCredential)
	if err != nil {
		return "", fmt.Errorf("Container User Delegation SAS署名失敗: %w", err)
	}

	containerURL := client.ServiceClient().NewContainerClient(containerName).URL()
	
	return fmt.Sprintf("%s?%s", containerURL, sasToken.Encode()), nil
}

// generateContainerServiceSAS はコンテナ用Service SASを生成
func generateContainerServiceSAS(ctx context.Context, containerName string, opts *SASOptions) (string, error) {
	if opts.AccountKey == "" || opts.AccountName == "" {
		return "", fmt.Errorf("Service SAS生成にはAccountKeyとAccountNameが必要")
	}

	// SAS開始時刻設定（clock skew対応で15分前から開始）
	start := time.Now().Add(-15 * time.Minute)
	expiry := start.Add(opts.ExpiryDuration)

	credential, err := azblob.NewSharedKeyCredential(opts.AccountName, opts.AccountKey)
	if err != nil {
		return "", fmt.Errorf("SharedKey Credential作成失敗: %w", err)
	}

	containerPermissions := sas.ContainerPermissions{
		Read:   opts.Permissions.Read,
		Write:  opts.Permissions.Write,
		Delete: opts.Permissions.Delete,
		List:   true,
	}

	sasValues := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     start,
		ExpiryTime:    expiry,
		Permissions:   containerPermissions.String(),
		ContainerName: containerName,
	}

	sasToken, err := sasValues.SignWithSharedKey(credential)
	if err != nil {
		return "", fmt.Errorf("Container Service SAS署名失敗: %w", err)
	}

	containerURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s", opts.AccountName, containerName)
	
	return fmt.Sprintf("%s?%s", containerURL, sasToken.Encode()), nil
}