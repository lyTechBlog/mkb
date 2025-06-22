package viking_db_tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DocumentUploadRequest represents the request structure for document upload
type DocumentUploadRequest struct {
	ResourceID string      `json:"resource_id"`
	AddType    string      `json:"add_type"`
	DocID      string      `json:"doc_id"`
	DocName    string      `json:"doc_name"`
	DocType    string      `json:"doc_type"`
	URL        string      `json:"url,omitempty"`
	Content    string      `json:"content,omitempty"`
	Meta       []MetaField `json:"meta,omitempty"`
}

// MetaField represents a metadata field for the document
type MetaField struct {
	FieldName  string      `json:"field_name"`
	FieldType  string      `json:"field_type"`
	FieldValue interface{} `json:"field_value"`
}

// DocumentUploadResponse represents the response structure for document upload
type DocumentUploadResponse struct {
	Code    int64                       `json:"code"`
	Message string                      `json:"message,omitempty"`
	Data    *DocumentUploadResponseData `json:"data,omitempty"`
}

// DocumentUploadResponseData represents the data part of the upload response
type DocumentUploadResponseData struct {
	DocID      string `json:"doc_id"`
	DocName    string `json:"doc_name"`
	CreateTime int64  `json:"create_time"`
	DocType    string `json:"doc_type"`
	Source     string `json:"source"`
}

const (
	DocumentUploadPath = "/api/knowledge/doc/add"
)

// UploadDocument uploads a document to the knowledge base
func UploadDocument(ctx context.Context, req *DocumentUploadRequest) (*DocumentUploadResponse, error) {
	// Prepare request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request using the existing PrepareRequest function
	httpReq := PrepareRequest("POST", DocumentUploadPath, body)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Execute request
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var uploadResp DocumentUploadResponse
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check if request was successful
	if uploadResp.Code != 0 {
		return &uploadResp, fmt.Errorf("upload failed with code %d: %s", uploadResp.Code, uploadResp.Message)
	}

	return &uploadResp, nil
}

// UploadDocumentByURL uploads a document using URL
func UploadDocumentByURL(ctx context.Context, resourceID, docID, docName, docType, urlPath string, meta []MetaField) (*DocumentUploadResponse, error) {
	req := &DocumentUploadRequest{
		ResourceID: resourceID,
		AddType:    "url",
		DocID:      docID,
		DocName:    docName,
		DocType:    docType,
		URL:        urlPath,
		Meta:       meta,
	}

	return UploadDocument(ctx, req)
}

// UploadDocumentByContent uploads a document using content
func UploadDocumentByContent(ctx context.Context, resourceID, docID, docName, docType, content string, meta []MetaField) (*DocumentUploadResponse, error) {
	req := &DocumentUploadRequest{
		ResourceID: resourceID,
		AddType:    "content",
		DocID:      docID,
		DocName:    docName,
		DocType:    docType,
		Content:    content,
		Meta:       meta,
	}

	return UploadDocument(ctx, req)
}

// CreateStringMetaField creates a string metadata field
func CreateStringMetaField(fieldName, fieldValue string) MetaField {
	return MetaField{
		FieldName:  fieldName,
		FieldType:  "string",
		FieldValue: fieldValue,
	}
}

// CreateBoolMetaField creates a boolean metadata field
func CreateBoolMetaField(fieldName string, fieldValue bool) MetaField {
	return MetaField{
		FieldName:  fieldName,
		FieldType:  "bool",
		FieldValue: fieldValue,
	}
}

// CreateIntMetaField creates an integer metadata field
// Note: The API doesn't support int field type, so we convert to string
func CreateIntMetaField(fieldName string, fieldValue int) MetaField {
	return MetaField{
		FieldName:  fieldName,
		FieldType:  "string",
		FieldValue: fmt.Sprintf("%d", fieldValue),
	}
}

// CreateFloatMetaField creates a float metadata field
// Note: The API might not support float field type, so we convert to string
func CreateFloatMetaField(fieldName string, fieldValue float64) MetaField {
	return MetaField{
		FieldName:  fieldName,
		FieldType:  "string",
		FieldValue: fmt.Sprintf("%f", fieldValue),
	}
}

// DocumentDeleteRequest represents the request structure for document deletion
type DocumentDeleteRequest struct {
	CollectionName string `json:"collection_name,omitempty"`
	Project        string `json:"project,omitempty"`
	ResourceID     string `json:"resource_id,omitempty"`
	DocID          string `json:"doc_id"`
}

// DocumentDeleteResponse represents the response structure for document deletion
type DocumentDeleteResponse struct {
	Code      int64  `json:"code"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

const (
	DocumentDeletePath = "/api/knowledge/doc/delete"
)

// DeleteDocument deletes a document from the knowledge base
func DeleteDocument(ctx context.Context, req *DocumentDeleteRequest) (*DocumentDeleteResponse, error) {
	// Prepare request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request using the existing PrepareRequest function
	httpReq := PrepareRequest("POST", DocumentDeletePath, body)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Execute request
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var deleteResp DocumentDeleteResponse
	if err := json.Unmarshal(respBody, &deleteResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check if request was successful
	if deleteResp.Code != 0 {
		return &deleteResp, fmt.Errorf("delete failed with code %d: %s", deleteResp.Code, deleteResp.Message)
	}

	return &deleteResp, nil
}

// DeleteDocumentByResourceID deletes a document using resource ID and doc ID
func DeleteDocumentByResourceID(ctx context.Context, resourceID, docID string) (*DocumentDeleteResponse, error) {
	req := &DocumentDeleteRequest{
		ResourceID: resourceID,
		DocID:      docID,
	}

	return DeleteDocument(ctx, req)
}

// DeleteDocumentByName deletes a document using collection name, project and doc ID
func DeleteDocumentByName(ctx context.Context, collectionName, project, docID string) (*DocumentDeleteResponse, error) {
	req := &DocumentDeleteRequest{
		CollectionName: collectionName,
		Project:        project,
		DocID:          docID,
	}

	return DeleteDocument(ctx, req)
}
