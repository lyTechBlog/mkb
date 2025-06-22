# 文档状态查询功能更新

## 概述

根据火山引擎官方文档，已将文档状态查询功能从使用 `/api/knowledge/point/list` 接口更新为使用 `/api/knowledge/doc/info` 接口。

## 修改内容

### 1. 新增接口和结构体

在 `viking_db_tool/knowleadge_op.go` 中新增了以下内容：

#### 新增常量
```go
const (
    DocumentInfoPath = "/api/knowledge/doc/info"
)
```

#### 新增请求结构体
```go
type DocumentInfoRequest struct {
    CollectionName string `json:"collection_name,omitempty"`
    Project        string `json:"project,omitempty"`
    ResourceID     string `json:"resource_id,omitempty"`
    DocID          string `json:"doc_id"`
    ReturnTokenUsage bool `json:"return_token_usage,omitempty"`
}
```

#### 新增响应结构体
```go
type DocumentInfoResponse struct {
    Code    int64                    `json:"code"`
    Message string                   `json:"message,omitempty"`
    Data    *DocumentInfoResponseData `json:"data,omitempty"`
}

type DocumentInfoResponseData struct {
    CollectionName string                 `json:"collection_name"`
    DocName        string                 `json:"doc_name"`
    DocID          string                 `json:"doc_id"`
    AddType        string                 `json:"add_type"`
    DocType        string                 `json:"doc_type"`
    CreateTime     int64                  `json:"create_time"`
    AddedBy        string                 `json:"added_by"`
    UpdateTime     int64                  `json:"update_time"`
    URL            string                 `json:"url,omitempty"`
    PointNum       int                    `json:"point_num"`
    Status         DocumentProcessingStatus `json:"status"`
    TotalTokens    int64                  `json:"total_tokens,omitempty"`
}

type DocumentProcessingStatus struct {
    ProcessStatus int `json:"process_status"`
}
```

### 2. 新增函数

#### GetDocumentInfo
```go
func GetDocumentInfo(ctx context.Context, req DocumentInfoRequest) (*DocumentInfoResponse, error)
```
- 功能：查询单个文档的详细信息
- 参数：文档信息查询请求
- 返回：文档信息响应

### 3. 更新现有函数

#### CheckDocumentProcessingStatus
- **更新前**：通过查询 `point_list` 来判断文档是否处理完成
- **更新后**：直接使用 `doc/info` 接口查询文档状态
- **状态判断**：`process_status` 字段
  - `0`: 处理中
  - `1`: 处理完成
  - `2`: 处理失败

#### GetDocumentProcessingStatus
- **更新前**：直接返回有 `point` 的文档为已处理
- **更新后**：先获取文档列表，然后逐个查询每个文档的状态

## 接口对比

### 旧接口：/api/knowledge/point/list
- 返回所有切片信息
- 需要通过切片存在性判断文档状态
- 信息不够准确

### 新接口：/api/knowledge/doc/info
- 返回文档详细信息
- 直接提供处理状态字段
- 信息更准确和完整

## 使用示例

### 查询单个文档状态
```go
isProcessed, err := viking_db_tool.CheckDocumentProcessingStatus(ctx, resourceID, docID)
if err != nil {
    // 处理错误
}
if isProcessed {
    fmt.Println("文档已处理完成")
} else {
    fmt.Println("文档正在处理中")
}
```

### 查询所有文档状态
```go
docStatus, err := viking_db_tool.GetDocumentProcessingStatus(ctx, resourceID)
if err != nil {
    // 处理错误
}
for docID, status := range docStatus {
    if status {
        fmt.Printf("文档 %s 已处理完成\n", docID)
    } else {
        fmt.Printf("文档 %s 正在处理中\n", docID)
    }
}
```

### 查询文档详细信息
```go
infoReq := viking_db_tool.DocumentInfoRequest{
    ResourceID: resourceID,
    DocID:      docID,
}
infoResp, err := viking_db_tool.GetDocumentInfo(ctx, infoReq)
if err != nil {
    // 处理错误
}
if infoResp.Code == 0 {
    fmt.Printf("文档名称: %s\n", infoResp.Data.DocName)
    fmt.Printf("处理状态: %d\n", infoResp.Data.Status.ProcessStatus)
    fmt.Printf("切片数量: %d\n", infoResp.Data.PointNum)
}
```

## 前端兼容性

前端代码无需修改，因为：
1. API 接口路径保持不变 (`/api/documents/status`)
2. 返回的数据格式保持不变
3. 状态判断逻辑保持不变

## 测试

可以使用 `test_doc_status.go` 文件来测试新的文档状态查询功能：

```bash
go run test_doc_status.go
```

注意：运行测试前需要替换 `resourceID` 和 `docID` 为实际的值。

## 优势

1. **准确性更高**：直接使用文档状态字段，而不是通过切片推断
2. **信息更完整**：可以获取文档的详细信息，包括处理时间、切片数量等
3. **性能更好**：对于单个文档状态查询，直接调用专用接口
4. **符合官方文档**：使用官方推荐的接口

## 注意事项

1. 新的实现会先获取文档列表，然后逐个查询状态，对于大量文档可能会有性能影响
2. 如果文档不存在，`doc/info` 接口会返回错误，需要适当处理
3. 建议在生产环境中添加适当的缓存机制来优化性能 