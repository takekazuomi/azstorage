package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/takekazu/azstorage/blob"
)

// createClient は環境変数に基づいてAzuriteまたはAzureクライアントを作成
func createClient(ctx context.Context, useAzure bool) (*blob.Client, error) {
	if useAzure {
		return createAzureClient(ctx)
	}
	return createAzuriteClient(ctx)
}

// createAzureClient はAzure Storage用クライアントを作成
func createAzureClient(ctx context.Context) (*blob.Client, error) {
	fmt.Println("Azure Storage接続モード")

	// 生成済みStorage Account名読み込み
	storageNameBytes, err := os.ReadFile(".azure-storage-name")
	if err != nil {
		return nil, fmt.Errorf("Azure Storage Account名読み込み失敗: %w", err)
	}
	storageAccountName := strings.TrimSpace(string(storageNameBytes))

	// Azure Blob URL構築
	blobURL := fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccountName)

	// Azureクライアント作成
	client, err := blob.NewClient(ctx, blobURL)
	if err != nil {
		return nil, fmt.Errorf("Azureクライアント作成失敗: %w", err)
	}

	return client, nil
}

// createAzuriteClient はAzurite用クライアントを作成
func createAzuriteClient(ctx context.Context) (*blob.Client, error) {
	fmt.Println("Azurite接続モード（OAuth/Bearer Token認証）")

	// Azurite用URL
	blobURL := "https://localhost:20000/devstoreaccount1"

	// Azuriteクライアント作成（自己署名証明書対応）
	client, err := blob.NewClient(ctx, blobURL, blob.WithInsecureSkipVerify())
	if err != nil {
		return nil, fmt.Errorf("Azuriteクライアント作成失敗: %w", err)
	}

	return client, nil
}

// createContainer はコンテナを作成
func createContainer(ctx context.Context, client *blob.Client, containerName string) error {
	_, err := client.CreateContainer(ctx, containerName, nil)
	if err != nil && !strings.Contains(err.Error(), "ContainerAlreadyExists") {
		return fmt.Errorf("コンテナ作成失敗: %w", err)
	}
	fmt.Printf("コンテナ '%s' 作成成功\n", containerName)
	return nil
}

// uploadBlob はBlobをアップロード
func uploadBlob(ctx context.Context, client *blob.Client, containerName, blobName, data string) error {
	_, err := client.UploadStream(
		ctx,
		containerName,
		blobName,
		strings.NewReader(data),
		nil,
	)
	if err != nil {
		return fmt.Errorf("アップロード失敗: %w", err)
	}
	fmt.Printf("Blob '%s' アップロード成功\n", blobName)
	return nil
}

// downloadBlob はBlobをダウンロードして内容を返す
func downloadBlob(ctx context.Context, client *blob.Client, containerName, blobName string) (string, error) {
	resp, err := client.DownloadStream(ctx, containerName, blobName, nil)
	if err != nil {
		return "", fmt.Errorf("ダウンロード失敗: %w", err)
	}
	defer resp.Body.Close()

	downloadedData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("データ読み取り失敗: %w", err)
	}

	content := string(downloadedData)
	fmt.Printf("ダウンロードデータ: %s\n", content)
	return content, nil
}

// listBlobs はBlob一覧を表示
func listBlobs(ctx context.Context, client *blob.Client, containerName string) error {
	pager := client.NewListBlobsFlatPager(containerName, nil)
	fmt.Println("\nBlob一覧:")
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("一覧取得失敗: %w", err)
		}
		for _, blob := range page.Segment.BlobItems {
			fmt.Printf("  - %s (Size: %d bytes)\n", *blob.Name, *blob.Properties.ContentLength)
		}
	}
	return nil
}


// getTestData は接続先に応じたテストデータを返す
func getTestData(useAzure bool) string {
	if useAzure {
		return "Hello from Azure Storage!"
	}
	return "Hello from Azurite OAuth!"
}

func main() {
	// 環境変数による接続先切り替え
	useAzure := os.Getenv("USE_AZURE") == "true"
	ctx := context.Background()

	// クライアント作成
	client, err := createClient(ctx, useAzure)
	if err != nil {
		log.Fatal(err)
	}

	// テストパラメータ
	containerName := "test-container"
	blobName := "sample.txt"
	data := getTestData(useAzure)

	// Blob操作実行
	if err := createContainer(ctx, client, containerName); err != nil {
		log.Printf("コンテナ作成エラー（既存の可能性）: %v", err)
	}

	if err := uploadBlob(ctx, client, containerName, blobName, data); err != nil {
		log.Fatal(err)
	}

	if _, err := downloadBlob(ctx, client, containerName, blobName); err != nil {
		log.Fatal(err)
	}

	if err := listBlobs(ctx, client, containerName); err != nil {
		log.Fatal(err)
	}

	// SAS URL生成（両環境対応、Azurite自動判定）
	fmt.Println("\n=== SAS URL生成 ===")
	sasURL, err := blob.GenerateBlobSAS(ctx, client, containerName, blobName, nil)
	if err != nil {
		log.Printf("SAS生成エラー: %v", err)
	} else {
		if useAzure {
			fmt.Printf("Azure User Delegation SAS生成成功:\n%s\n", sasURL)
		} else {
			fmt.Printf("Azurite SAS生成成功:\n%s\n", sasURL)
		}
	}

	fmt.Println("\n処理完了")
}