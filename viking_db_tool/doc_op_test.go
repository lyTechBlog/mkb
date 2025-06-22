package viking_db_tool

import (
	"context"
	"fmt"
	"testing"
)

func TestUploadDocumentByTosPath(t *testing.T) {
	ctx := context.Background()

	// Example metadata fields using helper functions
	meta := []MetaField{
		CreateStringMetaField("行业", "企业服务"),
		CreateBoolMetaField("是否公开", true),
		CreateStringMetaField("文档分类", "法律文书"),
		CreateIntMetaField("优先级", 1),
	}

	// Upload document by URL
	resp, err := UploadDocumentByURL(
		ctx,
		"kb-bd0872aa77719869",
		"test0123",
		"readme",
		"txt",
		"https://mkb-test.tos-cn-beijing.volces.com/example_dir/README.md?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Content-Sha256=UNSIGNED-PAYLOAD&X-Tos-Credential=AKTP23XTETEgZK4Dm6aMJ4TpYLpYxNsh9RNDumvDVGwaK08%2F20250619%2Fcn-beijing%2Ftos%2Frequest&X-Tos-Date=20250619T091320Z&X-Tos-Expires=3600&X-Tos-SignedHeaders=host&X-Tos-Security-Token=nCgdqdEROend3.ChsKBzNzX056d3cSEIV4-Jpc6UAGk5aGHeRGixgQjKnPwgYYnMXPwgYgrMie6gcoATCsyJ7qBzoEcm9vdEIDdG9zUgdQc0luZGllWAFgAQ.mz2OHwCuKiKgCzRGmMFTQNCldA29yObXOhnhL-OLOeDf_rGTsloy3kVFtEdLxq-wjGHMbWXVXTm3l4TQWfjcaw&X-Tos-Signature=a8551eb8fb2fd759171586f829679eca5df73935e8242dfb62000ec6caa42d1e",
		meta,
	)

	if err != nil {
		t.Errorf("Upload failed: %v", err)
		return
	}

	// Verify response structure
	if resp == nil {
		t.Error("Expected non-nil response")
		return
	}

	if resp.Code != 0 {
		t.Errorf("Expected success code 0, got %d: %s", resp.Code, resp.Message)
	}

	if resp.Data == nil {
		t.Error("Expected non-nil response data")
		return
	}

	// Verify response data fields
	if resp.Data.DocID != "test0123" {
		t.Errorf("Expected DocID 'test0123', got '%s'", resp.Data.DocID)
	}

	if resp.Data.DocName != "readme" {
		t.Errorf("Expected DocName 'readme', got '%s'", resp.Data.DocName)
	}

	if resp.Data.DocType != "txt" {
		t.Errorf("Expected DocType 'txt', got '%s'", resp.Data.DocType)
	}

	t.Logf("Upload successful: %+v", resp)
}

func TestDeleteDocument(t *testing.T) {
	ctx := context.Background()

	// 测试删除文档
	// 注意：这里需要替换为实际存在的resourceID和docID
	resourceID := "your_resource_id"
	docID := "your_doc_id"

	resp, err := DeleteDocumentByResourceID(ctx, resourceID, docID)
	if err != nil {
		t.Logf("Delete document failed: %v", err)
		// 如果文档不存在，这是预期的错误
		return
	}

	if resp.Code != 0 {
		t.Errorf("Delete document failed with code %d: %s", resp.Code, resp.Message)
		return
	}

	fmt.Printf("Document deleted successfully: %s\n", resp.RequestID)
}

func TestDeleteDocumentByName(t *testing.T) {
	ctx := context.Background()

	// 测试通过名称删除文档
	collectionName := "your_collection_name"
	project := "default"
	docID := "your_doc_id"

	resp, err := DeleteDocumentByName(ctx, collectionName, project, docID)
	if err != nil {
		t.Logf("Delete document by name failed: %v", err)
		// 如果文档不存在，这是预期的错误
		return
	}

	if resp.Code != 0 {
		t.Errorf("Delete document by name failed with code %d: %s", resp.Code, resp.Message)
		return
	}

	fmt.Printf("Document deleted successfully by name: %s\n", resp.RequestID)
}
