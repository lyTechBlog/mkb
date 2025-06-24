package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"tos_tool"
	"viking_db_tool"

	"file-upload-server/react_agent"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/http1/resp"
	"github.com/hertz-contrib/cors"
)

const (
	uploadDir   = "./uploads"
	maxFileSize = 100 << 20 // 100MB
)

// getDocTypeByExtension 根据文件后缀确定文档类型
func getDocTypeByExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	log.Printf("INFO: Determining document type for file: %s, extension: %s", filename, ext)

	// 非结构化文档支持类型
	unstructuredTypes := map[string]string{
		".txt":      "txt",
		".doc":      "doc",
		".docx":     "docx",
		".pdf":      "pdf",
		".markdown": "markdown",
		".md":       "markdown",
		".pptx":     "pptx",
	}

	// 结构化文档支持类型
	structuredTypes := map[string]string{
		".xlsx":  "xlsx",
		".csv":   "csv",
		".jsonl": "jsonl",
	}

	// 特殊处理：faq.xlsx 文件
	if strings.Contains(strings.ToLower(filename), "faq") && ext == ".xlsx" {
		log.Printf("INFO: Detected FAQ file, returning type: faq.xlsx")
		return "faq.xlsx"
	}

	// 先检查非结构化类型
	if docType, exists := unstructuredTypes[ext]; exists {
		log.Printf("INFO: File classified as unstructured document type: %s", docType)
		return docType
	}

	// 再检查结构化类型
	if docType, exists := structuredTypes[ext]; exists {
		log.Printf("INFO: File classified as structured document type: %s", docType)
		return docType
	}

	// 默认返回 txt
	log.Printf("INFO: File type not recognized, using default type: txt")
	return "txt"
}

// 会话状态管理
type StepResult struct {
	StepNumber      int       `json:"step_number"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	ExpectedOutcome string    `json:"expected_outcome"`
	ExecutionResult string    `json:"execution_result"`
	ExecutedAt      time.Time `json:"executed_at"`
	Status          string    `json:"status"` // "success", "failed", "pending"
}

type PlanSession struct {
	UserID        string                 `json:"user_id"`
	SessionID     string                 `json:"session_id"`
	OriginalQuery string                 `json:"original_query"`
	PlanData      map[string]interface{} `json:"plan_data"`
	StepResults   []StepResult           `json:"step_results"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	mutex         sync.RWMutex           `json:"-"`
}

// 全局会话存储
var (
	planSessions = make(map[string]*PlanSession)
	sessionMutex sync.RWMutex
)

// 创建新的计划会话
func createPlanSession(userID, query string, planData map[string]interface{}) *PlanSession {
	sessionID := fmt.Sprintf("%s_%d", userID, time.Now().Unix())

	session := &PlanSession{
		UserID:        userID,
		SessionID:     sessionID,
		OriginalQuery: query,
		PlanData:      planData,
		StepResults:   make([]StepResult, 0),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	sessionMutex.Lock()
	planSessions[sessionID] = session
	sessionMutex.Unlock()

	log.Printf("INFO: [SESSION] Created new plan session: %s for user: %s", sessionID, userID)
	return session
}

// 获取计划会话
func getPlanSession(sessionID string) (*PlanSession, bool) {
	sessionMutex.RLock()
	session, exists := planSessions[sessionID]
	sessionMutex.RUnlock()
	return session, exists
}

// 添加步骤执行结果
func (ps *PlanSession) AddStepResult(result StepResult) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// 查找是否已存在该步骤的结果，如果存在则更新，否则添加
	found := false
	for i, existing := range ps.StepResults {
		if existing.StepNumber == result.StepNumber {
			ps.StepResults[i] = result
			found = true
			break
		}
	}

	if !found {
		ps.StepResults = append(ps.StepResults, result)
	}

	ps.UpdatedAt = time.Now()
	log.Printf("INFO: [SESSION] Added step result for session %s, step %d", ps.SessionID, result.StepNumber)
}

// 获取前面步骤的执行结果
func (ps *PlanSession) GetPreviousStepResults(currentStepNumber int) []StepResult {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	var previousResults []StepResult
	for _, result := range ps.StepResults {
		if result.StepNumber < currentStepNumber && result.Status == "success" {
			previousResults = append(previousResults, result)
		}
	}

	return previousResults
}

// 构建包含前面步骤结果的上下文
func (ps *PlanSession) BuildContextWithPreviousResults(currentStepNumber int) string {
	previousResults := ps.GetPreviousStepResults(currentStepNumber)

	if len(previousResults) == 0 {
		return ""
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("\n\n# 当前计划的前置步骤执行结果\n")
	contextBuilder.WriteString("以下是当前执行计划中前面已完成步骤的执行结果，你可以参考这些信息来执行当前步骤：\n\n")

	for _, result := range previousResults {
		contextBuilder.WriteString(fmt.Sprintf("## 步骤 %d: %s\n", result.StepNumber, result.Title))
		contextBuilder.WriteString(fmt.Sprintf("**步骤描述**: %s\n", result.Description))
		contextBuilder.WriteString(fmt.Sprintf("**期望成果**: %s\n", result.ExpectedOutcome))
		contextBuilder.WriteString(fmt.Sprintf("**执行时间**: %s\n", result.ExecutedAt.Format("2006-01-02 15:04:05")))
		contextBuilder.WriteString(fmt.Sprintf("**实际执行结果**:\n%s\n\n", result.ExecutionResult))
		contextBuilder.WriteString("---\n\n")
	}

	contextBuilder.WriteString("请基于以上前置步骤的执行结果来完成当前步骤。如果前面的步骤结果包含了你需要的信息（如数据、文件路径、API响应等），请直接使用这些信息，避免重复执行相同的操作。\n")

	return contextBuilder.String()
}

// 清理过期会话（可选，用于内存管理）
func cleanupExpiredSessions() {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	expireTime := time.Now().Add(-24 * time.Hour) // 24小时过期
	for sessionID, session := range planSessions {
		if session.UpdatedAt.Before(expireTime) {
			delete(planSessions, sessionID)
			log.Printf("INFO: [SESSION] Cleaned up expired session: %s", sessionID)
		}
	}
}

func main() {
	log.Printf("INFO: Starting MKB server...")

	// 创建上传目录
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create upload directory: %v", err)
		panic(fmt.Sprintf("Failed to create upload directory: %v", err))
	}
	log.Printf("INFO: Upload directory created/verified: %s", uploadDir)

	h := server.Default(server.WithHostPorts("127.0.0.1:80"), server.WithStreamBody(true), server.WithTransport(standard.NewTransporter))

	// 配置CORS
	h.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	log.Printf("INFO: CORS middleware configured")

	// 根路径返回前端页面
	h.GET("/mkb", func(ctx context.Context, c *app.RequestContext) {
		log.Printf("INFO: Serving frontend page for request: %s %s", c.Method(), c.Request.URI().String())
		c.File("./static/index.html")
	})

	// 静态文件服务
	h.Static("/static", "./static")
	log.Printf("INFO: Static file serving configured for /static")

	// API路由
	api := h.Group("/api")
	{
		api.POST("/upload", uploadFile)
		api.GET("/files", listFiles)
		api.GET("/files/:filename", downloadFile)
		api.DELETE("/files/:filename", deleteFile)
		api.POST("/chat/stream", chatWithKnowledgeBaseStream)
		api.POST("/plan/stream", planWithKnowledgeBaseStream)
		api.POST("/plan/step", handlePlanStep)
		api.GET("/plan/session/:session_id", getPlanSessionStatus)
		api.GET("/plan/session/:session_id/context/:step_number", getPlanStepContext)
		api.GET("/documents/status", getDocumentStatus)
	}
	log.Printf("INFO: API routes configured")

	// 健康检查
	h.GET("/health", func(ctx context.Context, c *app.RequestContext) {
		log.Printf("INFO: Health check request: %s %s", c.Method(), c.Request.URI().String())
		c.JSON(consts.StatusOK, utils.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// 测试流式输出
	h.GET("/flush/chunk", func(ctx context.Context, c *app.RequestContext) {
		// Hijack the writer of response
		c.Response.HijackWriter(resp.NewChunkedBodyWriter(&c.Response, c.GetWriter()))

		for i := 0; i < 10; i++ {
			c.Write([]byte(fmt.Sprintf("chunk %d: %s", i, strings.Repeat("hi~", i)))) // nolint: errcheck
			c.Flush()                                                                 // nolint: errcheck
			time.Sleep(200 * time.Millisecond)
		}
	})

	log.Printf("INFO: Server starting on http://0.0.0.0:8888 (accessible from external IPs)")

	// 启动定期清理过期会话的goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // 每小时清理一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cleanupExpiredSessions()
			}
		}
	}()

	h.Spin()
}

// 文件上传处理
func uploadFile(ctx context.Context, c *app.RequestContext) {
	log.Printf("INFO: [UPLOAD] Starting file upload request: %s %s", c.Method(), c.Request.URI().String())
	startTime := time.Now()

	// 解析multipart表单
	form, err := c.MultipartForm()
	if err != nil {
		log.Printf("ERROR: [UPLOAD] Failed to parse form: %v", err)
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Failed to parse form: " + err.Error(),
		})
		return
	}
	log.Printf("INFO: [UPLOAD] Multipart form parsed successfully")

	// 获取用户ID
	userIDs := form.Value["user_id"]
	if len(userIDs) == 0 || userIDs[0] == "" {
		log.Printf("ERROR: [UPLOAD] User ID is missing or empty")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}
	userID := userIDs[0]
	log.Printf("INFO: [UPLOAD] Processing upload for user ID: %s", userID)

	files := form.File["file"]
	if len(files) == 0 {
		log.Printf("ERROR: [UPLOAD] No files uploaded")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "No file uploaded",
		})
		return
	}
	log.Printf("INFO: [UPLOAD] Found %d files to process", len(files))

	var uploadedFiles []map[string]interface{}
	for i, file := range files {
		log.Printf("INFO: [UPLOAD] Processing file %d/%d: %s (size: %d bytes)", i+1, len(files), file.Filename, file.Size)

		// 检查文件大小
		if file.Size > maxFileSize {
			log.Printf("ERROR: [UPLOAD] File %s exceeds maximum size: %d > %d", file.Filename, file.Size, maxFileSize)
			c.JSON(consts.StatusBadRequest, utils.H{
				"error": fmt.Sprintf("File %s is too large. Max size is %d bytes", file.Filename, maxFileSize),
			})
			return
		}

		// 创建临时文件路径
		filename := filepath.Base(file.Filename)
		tempFilePath := filepath.Join(uploadDir, filename)
		log.Printf("INFO: [UPLOAD] Creating temporary file: %s", tempFilePath)

		// 打开源文件
		src, err := file.Open()
		if err != nil {
			log.Printf("ERROR: [UPLOAD] Failed to open uploaded file %s: %v", file.Filename, err)
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to open uploaded file: " + err.Error(),
			})
			return
		}
		defer src.Close()

		// 创建临时文件
		dst, err := os.Create(tempFilePath)
		if err != nil {
			log.Printf("ERROR: [UPLOAD] Failed to create temporary file %s: %v", tempFilePath, err)
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to create temporary file: " + err.Error(),
			})
			return
		}

		// 复制文件内容到临时文件
		if _, err = io.Copy(dst, src); err != nil {
			dst.Close()
			log.Printf("ERROR: [UPLOAD] Failed to save temporary file %s: %v", tempFilePath, err)
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to save temporary file: " + err.Error(),
			})
			return
		}
		dst.Close()
		log.Printf("INFO: [UPLOAD] Temporary file created successfully: %s", tempFilePath)

		// 调用TOS上传方法，在路径中包含用户ID
		objectKey := fmt.Sprintf("uploads/%s/%s", userID, filename)
		log.Printf("INFO: [UPLOAD] Uploading to TOS with object key: %s", objectKey)
		preSignedURL, err := tos_tool.UploadFileWithEnvConfig(tempFilePath, objectKey)
		if err != nil {
			// 清理临时文件
			os.Remove(tempFilePath)
			log.Printf("ERROR: [UPLOAD] Failed to upload to TOS: %v", err)
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to upload to TOS: " + err.Error(),
			})
			return
		}
		log.Printf("INFO: [UPLOAD] Successfully uploaded to TOS, pre-signed URL generated")

		// 清理临时文件
		os.Remove(tempFilePath)
		log.Printf("INFO: [UPLOAD] Temporary file cleaned up: %s", tempFilePath)

		uploadedFiles = append(uploadedFiles, map[string]interface{}{
			"name":    filename,
			"url":     preSignedURL,
			"user_id": userID,
		})

		// 检查知识库是否存在
		knowledgeBaseName := "kb_" + userID // 使用用户id作为知识库名称
		project := "default"
		log.Printf("INFO: [UPLOAD] Checking knowledge base existence: %s in project: %s", knowledgeBaseName, project)
		exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
		if err != nil {
			log.Printf("ERROR: [UPLOAD] Failed to check knowledge base existence: %v", err)
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to check knowledge base existence: " + err.Error(),
			})
			return
		}

		if !exists {
			// 如果知识库不存在，创建知识库
			log.Printf("INFO: [UPLOAD] Knowledge base does not exist, creating new one: %s", knowledgeBaseName)
			createResp, err := viking_db_tool.CreateKnowledgeBase(ctx, knowledgeBaseName, "Knowledge base for user documents", "unstructured_data", project)
			if err != nil {
				log.Printf("ERROR: [UPLOAD] Failed to create knowledge base: %v", err)
				c.JSON(consts.StatusInternalServerError, utils.H{
					"error": "Failed to create knowledge base: " + err.Error(),
				})
				return
			}
			if createResp.Code != 0 {
				log.Printf("ERROR: [UPLOAD] Failed to create knowledge base, code: %d, message: %s", createResp.Code, createResp.Message)
				c.JSON(consts.StatusInternalServerError, utils.H{
					"error": "Failed to create knowledge base: " + createResp.Message,
				})
				return
			}
			resourceID = createResp.Data.ResourceID
			log.Printf("INFO: [UPLOAD] Successfully created knowledge base: %s with ResourceID: %s", knowledgeBaseName, resourceID)
		} else {
			log.Printf("INFO: [UPLOAD] Knowledge base exists: %s with ResourceID: %s", knowledgeBaseName, resourceID)
		}

		// 上传文件到Viking DB
		// docID只保留字符和数字以及_和-
		docID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(filename, "")
		docName := filename
		docType := getDocTypeByExtension(filename)
		meta := []viking_db_tool.MetaField{
			viking_db_tool.CreateStringMetaField("行业", "企业服务"),
			viking_db_tool.CreateStringMetaField("用户ID", userID),
		}

		log.Printf("INFO: [UPLOAD] Uploading document to Viking DB - docID: %s, docName: %s, docType: %s", docID, docName, docType)
		// 上传文件到Viking DB
		response, err := viking_db_tool.UploadDocumentByURL(
			ctx,
			resourceID,
			docID,
			docName,
			docType,
			preSignedURL,
			meta,
		)

		if err != nil {
			log.Printf("ERROR: [UPLOAD] Failed to upload to Viking DB: %v", err)
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to upload to Viking DB: " + err.Error(),
			})
			return
		}

		log.Printf("INFO: [UPLOAD] Successfully uploaded document to Viking DB: %s", response)
	}

	duration := time.Since(startTime)
	log.Printf("INFO: [UPLOAD] Upload request completed successfully in %v for user %s, uploaded %d files", duration, userID, len(uploadedFiles))
	c.JSON(consts.StatusOK, utils.H{
		"message": "Files uploaded successfully to TOS",
		"files":   uploadedFiles,
	})
}

// 列出所有文件
func listFiles(ctx context.Context, c *app.RequestContext) {
	log.Printf("INFO: [LIST_FILES] Starting list files request: %s %s", c.Method(), c.Request.URI().String())
	startTime := time.Now()

	// 获取用户ID参数
	userID := c.Query("user_id")
	if userID == "" {
		log.Printf("ERROR: [LIST_FILES] User ID is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}
	log.Printf("INFO: [LIST_FILES] Listing files for user ID: %s", userID)

	// 使用TOS工具列出用户上传的文件
	prefix := fmt.Sprintf("uploads/%s/", userID)
	log.Printf("INFO: [LIST_FILES] Searching TOS with prefix: %s", prefix)
	files, err := tos_tool.ListFilesWithEnvConfig(prefix)
	if err != nil {
		log.Printf("ERROR: [LIST_FILES] Failed to list files from TOS: %v", err)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to list files from TOS: " + err.Error(),
		})
		return
	}

	// 为每个文件添加用户ID信息
	for i := range files {
		files[i]["user_id"] = userID
	}

	duration := time.Since(startTime)
	log.Printf("INFO: [LIST_FILES] List files request completed in %v for user %s, found %d files", duration, userID, len(files))
	c.JSON(consts.StatusOK, utils.H{
		"files":   files,
		"user_id": userID,
	})
}

// 下载文件
func downloadFile(ctx context.Context, c *app.RequestContext) {
	filename := c.Param("filename")
	log.Printf("INFO: [DOWNLOAD] Starting file download request: %s %s, filename: %s", c.Method(), c.Request.URI().String(), filename)

	if filename == "" {
		log.Printf("ERROR: [DOWNLOAD] Filename is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Filename is required",
		})
		return
	}

	filepath := filepath.Join(uploadDir, filename)
	log.Printf("INFO: [DOWNLOAD] Attempting to serve file: %s", filepath)

	// 检查文件是否存在
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		log.Printf("ERROR: [DOWNLOAD] File not found: %s", filepath)
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "File not found",
		})
		return
	}

	log.Printf("INFO: [DOWNLOAD] Serving file: %s", filepath)
	c.File(filepath)
}

// 删除文件
func deleteFile(ctx context.Context, c *app.RequestContext) {
	filename := c.Param("filename")
	log.Printf("INFO: [DELETE] Starting file delete request: %s %s, filename: %s", c.Method(), c.Request.URI().String(), filename)
	startTime := time.Now()

	if filename == "" {
		log.Printf("ERROR: [DELETE] Filename is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Filename is required",
		})
		return
	}

	// 获取用户ID参数
	userID := c.Query("user_id")
	if userID == "" {
		log.Printf("ERROR: [DELETE] User ID is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}
	log.Printf("INFO: [DELETE] Deleting file %s for user ID: %s", filename, userID)

	// 构建TOS对象键
	objectKey := fmt.Sprintf("uploads/%s/%s", userID, filename)
	log.Printf("INFO: [DELETE] Deleting from TOS with object key: %s", objectKey)

	// 删除TOS中的文件
	err := tos_tool.DeleteFileWithEnvConfig(objectKey)
	if err != nil {
		log.Printf("ERROR: [DELETE] Failed to delete file from TOS: %v", err)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to delete file from TOS: " + err.Error(),
		})
		return
	}
	log.Printf("INFO: [DELETE] Successfully deleted file from TOS")

	// 检查知识库是否存在
	knowledgeBaseName := "kb_" + userID
	project := "default"
	log.Printf("INFO: [DELETE] Checking knowledge base existence: %s in project: %s", knowledgeBaseName, project)
	exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
	if err != nil {
		// 如果检查知识库存在性失败，记录错误但不影响TOS删除的成功响应
		log.Printf("WARN: [DELETE] Failed to check knowledge base existence: %v", err)
		duration := time.Since(startTime)
		log.Printf("INFO: [DELETE] Delete request completed in %v for user %s (TOS only)", duration, userID)
		c.JSON(consts.StatusOK, utils.H{
			"message": "File deleted successfully from TOS, but failed to check knowledge base",
		})
		return
	}

	if exists {
		// 生成与上传时相同的docID
		docID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(filename, "")
		log.Printf("INFO: [DELETE] Deleting document from knowledge base - docID: %s", docID)

		// 删除知识库中的文档
		deleteResp, err := viking_db_tool.DeleteDocumentByResourceID(ctx, resourceID, docID)
		if err != nil {
			// 如果删除知识库文档失败，记录错误但不影响TOS删除的成功响应
			log.Printf("WARN: [DELETE] Failed to delete document from knowledge base: %v", err)
			duration := time.Since(startTime)
			log.Printf("INFO: [DELETE] Delete request completed in %v for user %s (TOS only)", duration, userID)
			c.JSON(consts.StatusOK, utils.H{
				"message": "File deleted successfully from TOS, but failed to delete from knowledge base",
			})
			return
		}

		if deleteResp.Code != 0 {
			log.Printf("WARN: [DELETE] Failed to delete document from knowledge base, code: %d, message: %s", deleteResp.Code, deleteResp.Message)
			duration := time.Since(startTime)
			log.Printf("INFO: [DELETE] Delete request completed in %v for user %s (TOS only)", duration, userID)
			c.JSON(consts.StatusOK, utils.H{
				"message": "File deleted successfully from TOS, but failed to delete from knowledge base",
			})
			return
		}

		log.Printf("INFO: [DELETE] Successfully deleted document from knowledge base: %s", docID)
		duration := time.Since(startTime)
		log.Printf("INFO: [DELETE] Delete request completed in %v for user %s (TOS + Knowledge Base)", duration, userID)
		c.JSON(consts.StatusOK, utils.H{
			"message": "File deleted successfully from both TOS and knowledge base",
		})
	} else {
		// 知识库不存在，只删除TOS文件
		log.Printf("INFO: [DELETE] Knowledge base not found, only TOS file deleted")
		duration := time.Since(startTime)
		log.Printf("INFO: [DELETE] Delete request completed in %v for user %s (TOS only)", duration, userID)
		c.JSON(consts.StatusOK, utils.H{
			"message": "File deleted successfully from TOS (knowledge base not found)",
		})
	}
}

// 流式知识库对话处理
func chatWithKnowledgeBaseStream(ctx context.Context, c *app.RequestContext) {
	// Hijack the writer of response
	c.Response.HijackWriter(resp.NewChunkedBodyWriter(&c.Response, c.GetWriter()))

	log.Printf("INFO: [CHAT_STREAM] Starting streaming chat request: %s %s", c.Method(), c.Request.URI().String())
	startTime := time.Now()

	// 发送开始状态
	c.Write([]byte("data: {\"status\": \"开始处理请求...\", \"progress\": 10}\n\n"))
	time.Sleep(300 * time.Millisecond)
	c.Flush()

	var request struct {
		UserID   string                        `json:"user_id"`
		Query    string                        `json:"query"`
		Messages []viking_db_tool.MessageParam `json:"messages"`
	}

	if err := c.BindJSON(&request); err != nil {
		log.Printf("ERROR: [CHAT_STREAM] Failed to bind JSON request: %v", err)
		c.Write([]byte(fmt.Sprintf("data: {\"error\": \"Invalid request format: %s\"}\n\n", err.Error())))
		return
	}

	if request.UserID == "" {
		log.Printf("ERROR: [CHAT_STREAM] User ID is missing")
		c.Write([]byte("data: {\"error\": \"User ID is required\"}\n\n"))
		return
	}

	if request.Query == "" {
		log.Printf("ERROR: [CHAT_STREAM] Query is missing")
		c.Write([]byte("data: {\"error\": \"Query is required\"}\n\n"))
		return
	}

	log.Printf("INFO: [CHAT_STREAM] Processing streaming chat for user ID: %s, query: %s, messages count: %d", request.UserID, request.Query, len(request.Messages))

	// 发送验证通过状态
	c.Write([]byte("data: {\"status\": \"请求验证通过，开始处理...\", \"progress\": 20}\n\n"))
	time.Sleep(300 * time.Millisecond)
	c.Flush()

	// 检查知识库是否存在
	knowledgeBaseName := "kb_" + request.UserID
	project := "default"

	// 发送检查知识库状态
	c.Write([]byte(fmt.Sprintf("data: {\"status\": \"正在检查知识库: %s\", \"progress\": 30}\n\n", knowledgeBaseName)))
	time.Sleep(300 * time.Millisecond)
	c.Flush()

	log.Printf("INFO: [CHAT_STREAM] Checking knowledge base existence: %s in project: %s", knowledgeBaseName, project)
	exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
	if err != nil {
		log.Printf("ERROR: [CHAT_STREAM] Failed to check knowledge base existence: %v", err)
		c.Write([]byte(fmt.Sprintf("data: {\"error\": \"Failed to check knowledge base existence: %s\"}\n\n", err.Error())))
		return
	}

	if !exists {
		log.Printf("ERROR: [CHAT_STREAM] Knowledge base not found for user: %s", request.UserID)
		c.Write([]byte("data: {\"error\": \"Knowledge base not found. Please upload some files first.\"}\n\n"))
		return
	}

	// 发送知识库存在状态
	c.Write([]byte(fmt.Sprintf("data: {\"status\": \"知识库检查完成，资源ID: %s\", \"progress\": 40}\n\n", resourceID)))
	time.Sleep(300 * time.Millisecond)
	c.Flush()

	// 设置知识库检索参数
	viking_db_tool.CollectionName = knowledgeBaseName
	viking_db_tool.Project = project
	viking_db_tool.ResourceID = resourceID
	viking_db_tool.Query = request.Query
	log.Printf("INFO: [CHAT_STREAM] Knowledge base parameters set - Collection: %s, Project: %s, ResourceID: %s", knowledgeBaseName, project, resourceID)

	// 构建检索请求参数
	searchReq := viking_db_tool.CollectionSearchKnowledgeRequest{
		Name:        knowledgeBaseName,
		Project:     project,
		ResourceId:  resourceID,
		Query:       request.Query,
		Limit:       5, // 限制返回结果数量
		DenseWeight: 0.5,
		Preprocessing: viking_db_tool.PreProcessing{
			NeedInstruction:  true,
			ReturnTokenUsage: true,
			Rewrite:          false,
			Messages:         request.Messages, // 使用传入的聊天历史
		},
		Postprocessing: viking_db_tool.PostProcessing{
			RerankSwitch:        false,
			RetrieveCount:       25,
			GetAttachmentLink:   true,
			ChunkGroup:          true,
			ChunkDiffusionCount: 0,
		},
	}

	// 执行知识库检索
	c.Write([]byte("data: {\"status\": \"正在检索相关知识库内容...\", \"progress\": 50}\n\n"))
	time.Sleep(300 * time.Millisecond)
	c.Flush()

	log.Printf("INFO: [CHAT_STREAM] Executing knowledge base search...")
	searchResp, err := viking_db_tool.SearchKnowledgeWithParams(ctx, searchReq)
	if err != nil {
		log.Printf("ERROR: [CHAT_STREAM] Failed to search knowledge base: %v", err)
		c.Write([]byte(fmt.Sprintf("data: {\"error\": \"Failed to search knowledge base: %s\"}\n\n", err.Error())))
		return
	}

	if searchResp.Code != 0 {
		log.Printf("ERROR: [CHAT_STREAM] Knowledge base search failed, code: %d, message: %s", searchResp.Code, searchResp.Message)
		c.Write([]byte(fmt.Sprintf("data: {\"error\": \"Knowledge base search failed: %s\"}\n\n", searchResp.Message)))
		return
	}

	// 发送检索完成状态
	c.Write([]byte("data: {\"status\": \"知识库检索完成，正在生成提示词...\", \"progress\": 60}\n\n"))
	time.Sleep(300 * time.Millisecond)
	c.Flush()

	log.Printf("INFO: [CHAT_STREAM] Knowledge base search completed successfully")

	// 生成提示词
	log.Printf("INFO: [CHAT_STREAM] Generating prompt from search results...")
	prompt, images, err := viking_db_tool.GeneratePrompt(searchResp)
	if err != nil {
		log.Printf("ERROR: [CHAT_STREAM] Failed to generate prompt: %v", err)
		c.Write([]byte(fmt.Sprintf("data: {\"error\": \"Failed to generate prompt: %s\"}\n\n", err.Error())))
		return
	}

	// 发送提示词生成完成状态
	if len(images) > 0 {
		c.Write([]byte(fmt.Sprintf("data: {\"status\": \"提示词生成完成，检测到 %d 张图片，准备多模态对话...\", \"progress\": 70}\n\n", len(images))))
	} else {
		c.Write([]byte("data: {\"status\": \"提示词生成完成，准备文本对话...\", \"progress\": 70}\n\n"))
	}
	time.Sleep(300 * time.Millisecond)
	c.Flush()

	log.Printf("INFO: [CHAT_STREAM] Prompt generated successfully, images count: %d", len(images))

	// 构建对话消息
	var messages []viking_db_tool.MessageParam
	if len(images) > 0 {
		// 对于Vision模型，需要将图片链接拼接到Message中
		log.Printf("INFO: [CHAT_STREAM] Building multimodal message with %d images", len(images))
		var multiModalMessage []*viking_db_tool.ChatCompletionMessageContentPart
		multiModalMessage = append(multiModalMessage, &viking_db_tool.ChatCompletionMessageContentPart{
			Type: viking_db_tool.ChatCompletionMessageContentPartTypeText,
			Text: request.Query,
		})
		for _, imageURL := range images {
			multiModalMessage = append(multiModalMessage, &viking_db_tool.ChatCompletionMessageContentPart{
				Type:     viking_db_tool.ChatCompletionMessageContentPartTypeImageURL,
				ImageURL: &viking_db_tool.ChatMessageImageURL{URL: imageURL},
			})
		}

		messages = []viking_db_tool.MessageParam{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: multiModalMessage,
			},
		}
	} else {
		// 普通文本LLM模型
		log.Printf("INFO: [CHAT_STREAM] Building text-only message")
		messages = []viking_db_tool.MessageParam{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: request.Query,
			},
		}
	}

	// 调用流式大模型生成回答
	c.Write([]byte("data: {\"status\": \"正在调用AI模型生成回答...\", \"progress\": 80}\n\n"))
	time.Sleep(300 * time.Millisecond)
	c.Flush()

	log.Printf("INFO: [CHAT_STREAM] Starting streaming LLM response generation...")
	chunkCount := 0

	err = viking_db_tool.ChatCompletionStreamWithCallback(ctx, messages, func(chunk string, isDone bool, usage *viking_db_tool.ModelTokenUsage, err error) {
		if err != nil {
			log.Printf("ERROR: [CHAT_STREAM] Stream callback error: %v", err)
			c.Write([]byte(fmt.Sprintf("data: {\"error\": \"%s\"}\n\n", err.Error())))
			return
		}

		if chunk != "" {
			chunkCount++
			log.Printf("INFO: [CHAT_STREAM] Sending chunk %d: %s", chunkCount, chunk)
			// 发送文本块
			chunkResponse := map[string]interface{}{
				"chunk": chunk,
				"done":  false,
			}
			chunkResponseJSON, _ := json.Marshal(chunkResponse)
			c.Write([]byte("data: " + string(chunkResponseJSON) + "\n\n"))
			c.Flush() // 立即发送数据
		}

		if isDone {
			log.Printf("INFO: [CHAT_STREAM] Stream completed, total chunks: %d", chunkCount)
			// 发送完成消息
			finalResponse := map[string]interface{}{
				"done":  true,
				"usage": usage,
			}
			finalResponseJSON, _ := json.Marshal(finalResponse)
			c.Write([]byte("data: " + string(finalResponseJSON) + "\n\n"))
			c.Flush() // 立即发送数据
		}
	})

	if err != nil {
		log.Printf("ERROR: [CHAT_STREAM] Failed to generate streaming response: %v", err)
		c.Write([]byte(fmt.Sprintf("data: {\"error\": \"Failed to generate response: %s\"}\n\n", err.Error())))
		return
	}

	duration := time.Since(startTime)
	log.Printf("INFO: [CHAT_STREAM] Streaming chat request completed in %v for user %s", duration, request.UserID)
}

// 查询文档处理状态
func getDocumentStatus(ctx context.Context, c *app.RequestContext) {
	//log.Printf("INFO: [DOC_STATUS] Starting document status request: %s %s", c.Method(), c.Request.URI().String())
	//startTime := time.Now()

	// 获取用户ID参数
	userID := c.Query("user_id")
	if userID == "" {
		//log.Printf("ERROR: [DOC_STATUS] User ID is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}
	//log.Printf("INFO: [DOC_STATUS] Getting document status for user ID: %s", userID)

	// 检查知识库是否存在
	knowledgeBaseName := "kb_" + userID
	project := "default"
	//log.Printf("INFO: [DOC_STATUS] Checking knowledge base existence: %s in project: %s", knowledgeBaseName, project)
	exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
	if err != nil {
		//log.Printf("ERROR: [DOC_STATUS] Failed to check knowledge base existence: %v", err)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to check knowledge base existence: " + err.Error(),
		})
		return
	}

	if !exists {
		//log.Printf("ERROR: [DOC_STATUS] Knowledge base not found for user: %s", userID)
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "Knowledge base not found",
		})
		return
	}

	// 获取文档处理状态
	//log.Printf("INFO: [DOC_STATUS] Getting document processing status for resource ID: %s", resourceID)
	docStatus, err := viking_db_tool.GetDocumentProcessingStatus(ctx, resourceID)
	if err != nil {
		//log.Printf("ERROR: [DOC_STATUS] Failed to get document status: %v", err)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to get document status: " + err.Error(),
		})
		return
	}

	//duration := time.Since(startTime)
	//log.Printf("INFO: [DOC_STATUS] Document status request completed in %v for user %s", duration, userID)
	c.JSON(consts.StatusOK, utils.H{
		"document_status": docStatus,
		"user_id":         userID,
	})
}

// 非流式知识库计划处理
func planWithKnowledgeBaseStream(ctx context.Context, c *app.RequestContext) {
	log.Printf("INFO: [PLAN] Starting plan request: %s %s", c.Method(), c.Request.URI().String())
	startTime := time.Now()

	var request struct {
		UserID   string                        `json:"user_id"`
		Query    string                        `json:"query"`
		Messages []viking_db_tool.MessageParam `json:"messages"`
	}

	if err := c.BindJSON(&request); err != nil {
		log.Printf("ERROR: [PLAN] Failed to bind JSON request: %v", err)
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Invalid request format: " + err.Error(),
		})
		return
	}

	if request.UserID == "" {
		log.Printf("ERROR: [PLAN] User ID is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}

	if request.Query == "" {
		log.Printf("ERROR: [PLAN] Query is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Query is required",
		})
		return
	}

	log.Printf("INFO: [PLAN] Processing plan for user ID: %s, query: %s, messages count: %d", request.UserID, request.Query, len(request.Messages))

	// 检查知识库是否存在
	knowledgeBaseName := "kb_" + request.UserID
	project := "default"

	log.Printf("INFO: [PLAN] Checking knowledge base existence: %s in project: %s", knowledgeBaseName, project)
	exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
	if err != nil {
		log.Printf("ERROR: [PLAN] Failed to check knowledge base existence: %v", err)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to check knowledge base existence: " + err.Error(),
		})
		return
	}

	if !exists {
		log.Printf("ERROR: [PLAN] Knowledge base not found for user: %s", request.UserID)
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "Knowledge base not found. Please upload some files first.",
		})
		return
	}

	// 设置知识库检索参数
	viking_db_tool.CollectionName = knowledgeBaseName
	viking_db_tool.Project = project
	viking_db_tool.ResourceID = resourceID
	viking_db_tool.Query = request.Query
	log.Printf("INFO: [PLAN] Knowledge base parameters set - Collection: %s, Project: %s, ResourceID: %s", knowledgeBaseName, project, resourceID)

	// 构建检索请求参数
	searchReq := viking_db_tool.CollectionSearchKnowledgeRequest{
		Name:        knowledgeBaseName,
		Project:     project,
		ResourceId:  resourceID,
		Query:       request.Query,
		Limit:       5, // 限制返回结果数量
		DenseWeight: 0.5,
		Preprocessing: viking_db_tool.PreProcessing{
			NeedInstruction:  true,
			ReturnTokenUsage: true,
			Rewrite:          false,
			Messages:         request.Messages, // 使用传入的聊天历史
		},
		Postprocessing: viking_db_tool.PostProcessing{
			RerankSwitch:        false,
			RetrieveCount:       25,
			GetAttachmentLink:   true,
			ChunkGroup:          true,
			ChunkDiffusionCount: 0,
		},
	}

	// 执行知识库检索
	log.Printf("INFO: [PLAN] Executing knowledge base search...")
	searchResp, err := viking_db_tool.SearchKnowledgeWithParams(ctx, searchReq)
	if err != nil {
		log.Printf("ERROR: [PLAN] Failed to search knowledge base: %v", err)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to search knowledge base: " + err.Error(),
		})
		return
	}

	if searchResp.Code != 0 {
		log.Printf("ERROR: [PLAN] Knowledge base search failed, code: %d, message: %s", searchResp.Code, searchResp.Message)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Knowledge base search failed: " + searchResp.Message,
		})
		return
	}

	log.Printf("INFO: [PLAN] Knowledge base search completed successfully")

	// 生成计划模式的提示词
	log.Printf("INFO: [PLAN] Generating plan prompt from search results...")
	prompt, images, err := generatePlanPrompt(searchResp)
	if err != nil {
		log.Printf("ERROR: [PLAN] Failed to generate plan prompt: %v", err)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to generate plan prompt: " + err.Error(),
		})
		return
	}

	log.Printf("INFO: [PLAN] Plan prompt generated successfully, images count: %d", len(images))

	// 构建对话消息
	var messages []viking_db_tool.MessageParam
	if len(images) > 0 {
		// 对于Vision模型，需要将图片链接拼接到Message中
		log.Printf("INFO: [PLAN] Building multimodal message with %d images", len(images))
		var multiModalMessage []*viking_db_tool.ChatCompletionMessageContentPart
		multiModalMessage = append(multiModalMessage, &viking_db_tool.ChatCompletionMessageContentPart{
			Type: viking_db_tool.ChatCompletionMessageContentPartTypeText,
			Text: request.Query,
		})
		for _, imageURL := range images {
			multiModalMessage = append(multiModalMessage, &viking_db_tool.ChatCompletionMessageContentPart{
				Type:     viking_db_tool.ChatCompletionMessageContentPartTypeImageURL,
				ImageURL: &viking_db_tool.ChatMessageImageURL{URL: imageURL},
			})
		}

		messages = []viking_db_tool.MessageParam{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: multiModalMessage,
			},
		}
	} else {
		// 普通文本LLM模型
		log.Printf("INFO: [PLAN] Building text-only message")
		messages = []viking_db_tool.MessageParam{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: request.Query,
			},
		}
	}

	// 调用非流式大模型生成计划
	log.Printf("INFO: [PLAN] Starting LLM plan generation...")
	response, err := viking_db_tool.ChatCompletion(ctx, messages)
	if err != nil {
		log.Printf("ERROR: [PLAN] Failed to generate plan: %v", err)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to generate plan: " + err.Error(),
		})
		return
	}

	if response.Code != 0 {
		log.Printf("ERROR: [PLAN] LLM response failed, code: %d, message: %s", response.Code, response.Message)
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "LLM response failed: " + response.Message,
		})
		return
	}

	duration := time.Since(startTime)
	log.Printf("INFO: [PLAN] Plan request completed in %v for user %s", duration, request.UserID)

	// 解析计划数据并创建会话
	var planData map[string]interface{}
	if err := json.Unmarshal([]byte(response.Data.GenerateAnswer), &planData); err != nil {
		log.Printf("WARN: [PLAN] Failed to parse plan data as JSON, treating as plain text: %v", err)
		planData = map[string]interface{}{
			"raw_plan": response.Data.GenerateAnswer,
		}
	}

	// 创建计划会话
	session := createPlanSession(request.UserID, request.Query, planData)

	c.JSON(consts.StatusOK, utils.H{
		"plan":       response.Data.GenerateAnswer,
		"usage":      response.Data.Usage,
		"user_id":    request.UserID,
		"session_id": session.SessionID,
		"duration":   duration.String(),
	})
}

// 生成计划模式的提示词
func generatePlanPrompt(resp *viking_db_tool.CollectionSearchKnowledgeResponse) (string, []string, error) {
	if resp == nil {
		return "", nil, fmt.Errorf("response is nil")
	}
	if resp.Code != 0 {
		return "", nil, fmt.Errorf(resp.Message)
	}

	var promptBuilder strings.Builder
	var imageURLs []string
	usingVLM := strings.Contains(viking_db_tool.ModelName, "vision")
	imageCnt := 0

	// 计划模式的系统提示词
	promptBuilder.WriteString(`# 任务
你是一位专业的项目规划师和任务分解专家。你的任务是根据用户的目标和需求，制定一个执行计划,计划尽量简单，并考虑到已有的可使用的工具。这些计划的每一步，我会让agent去执行，所以请确保计划的每一步都是可执行的。

你的计划需要满足以下要求：
1. 将用户的目标分解为具体的、可执行的步骤
2. 基于提供的参考资料，确保计划的可行性和准确性
3. 如果某步骤需要依赖前面的结果，不要将当前步骤分为两个，而是在一步中完成
4. 最后一步是结论总结员，清晰简洁地总结答案

# 输出格式
请严格按照以下JSON格式输出执行计划，不要包含任何其他文本：

{
  "goal_analysis": "分析用户的具体目标和要求",
  "steps": [
    {
		"step_number": 1,
		"title": "步骤标题",
		"description": "具体操作描述",
		"expected_outcome": "可衡量的成果",
    }
  ],
}

# 可用工具
高德mcp server

# 参考资料
<context>
`)

	// 添加检索到的内容
	for _, item := range resp.Data.ResultList {
		if item.Content != "" {
			content := getContentForPrompt(item, imageCnt)
			promptBuilder.WriteString(content)
			promptBuilder.WriteString("\n\n")
		}

		// 处理图片
		if usingVLM && len(item.ChunkAttachmentList) > 0 && item.ChunkAttachmentList[0].Link != "" {
			imageURLs = append(imageURLs, item.ChunkAttachmentList[0].Link)
			imageCnt++
		}
	}

	promptBuilder.WriteString(`</context>

现在请你根据提供的参考资料，为用户制定一个详细的执行计划。`)

	return promptBuilder.String(), imageURLs, nil
}

// getContentForPrompt 生成内容提示
func getContentForPrompt(item *viking_db_tool.CollectionSearchResponseItem, imageNum int) string {
	content := item.Content

	if item.OriginalQuestion != "" {
		return fmt.Sprintf("当询问到相似问题时，请参考对应答案进行回答：问题：\"%s\"。答案：\"%s\"",
			item.OriginalQuestion, content)
	}

	if imageNum > 0 && len(item.ChunkAttachmentList) > 0 && item.ChunkAttachmentList[0].Link != "" {
		placeholder := fmt.Sprintf("<img>图片%d</img>", imageNum)
		return content + placeholder
	}

	return content
}

// 处理计划步骤点击事件
func handlePlanStep(ctx context.Context, c *app.RequestContext) {
	log.Printf("INFO: [PLAN_STEP] Starting plan step request: %s %s", c.Method(), c.Request.URI().String())
	startTime := time.Now()

	var request struct {
		UserID          string `json:"user_id"`
		SessionID       string `json:"session_id"`
		StepNumber      int    `json:"step_number"`
		Title           string `json:"title"`
		Description     string `json:"description"`
		ExpectedOutcome string `json:"expected_outcome"`
	}

	if err := c.BindJSON(&request); err != nil {
		log.Printf("ERROR: [PLAN_STEP] Failed to bind JSON request: %v", err)
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Invalid request format: " + err.Error(),
		})
		return
	}

	if request.UserID == "" {
		log.Printf("ERROR: [PLAN_STEP] User ID is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}

	if request.SessionID == "" {
		log.Printf("ERROR: [PLAN_STEP] Session ID is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Session ID is required",
		})
		return
	}

	log.Printf("INFO: [PLAN_STEP] User %s executing step %d: %s (Session: %s)", request.UserID, request.StepNumber, request.Title, request.SessionID)
	log.Printf("INFO: [PLAN_STEP] Step details - Description: %s, Expected Outcome: %s", request.Description, request.ExpectedOutcome)

	// 获取计划会话
	session, exists := getPlanSession(request.SessionID)
	if !exists {
		log.Printf("ERROR: [PLAN_STEP] Session not found: %s", request.SessionID)
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "Session not found. Please regenerate the plan.",
		})
		return
	}

	// 创建步骤结果记录
	stepResult := StepResult{
		StepNumber:      request.StepNumber,
		Title:           request.Title,
		Description:     request.Description,
		ExpectedOutcome: request.ExpectedOutcome,
		ExecutedAt:      time.Now(),
		Status:          "pending",
	}

	// 使用 React Agent 执行步骤
	log.Printf("INFO: [PLAN_STEP] Initializing React Agent for step execution...")

	// 创建 React Agent
	cfg := &react_agent.Config{}
	agent, err := react_agent.GetReactAgentWithAllTools(ctx, cfg)
	if err != nil {
		log.Printf("ERROR: [PLAN_STEP] Failed to create React Agent: %v", err)
		stepResult.Status = "failed"
		stepResult.ExecutionResult = "Failed to initialize React Agent: " + err.Error()
		session.AddStepResult(stepResult)

		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to initialize React Agent: " + err.Error(),
		})
		return
	}

	// 获取前面步骤的执行结果作为上下文
	previousContext := session.BuildContextWithPreviousResults(request.StepNumber)
	log.Printf("INFO: [PLAN_STEP] Built context from %d previous steps", len(session.GetPreviousStepResults(request.StepNumber)))

	// 构建执行任务的提示词，包含前面步骤的结果
	taskPrompt := fmt.Sprintf(`你正在执行一个多步骤计划中的某个步骤。请执行以下任务步骤：

## 当前步骤信息
步骤 %d: %s

**描述**: %s
**期望结果**: %s

%s

## 执行指导
请根据上述描述执行相应的操作，并提供详细的执行结果。注意事项：
1. 如果需要使用工具来完成任务，请调用相应的工具
2. 如果前面的步骤结果对当前步骤有帮助，请充分利用这些信息
3. 避免重复执行前面步骤已经完成的操作
4. 确保你的执行结果能够为后续步骤提供有用的信息`,
		request.StepNumber, request.Title, request.Description, request.ExpectedOutcome, previousContext)

	log.Printf("INFO: [PLAN_STEP] Executing step with React Agent...")

	// 使用 React Agent 生成响应
	message, err := agent.Generate(ctx, []*schema.Message{
		{
			Role:    "user",
			Content: taskPrompt,
		},
	})

	if err != nil {
		log.Printf("ERROR: [PLAN_STEP] React Agent execution failed: %v", err)
		stepResult.Status = "failed"
		stepResult.ExecutionResult = "React Agent execution failed: " + err.Error()
		session.AddStepResult(stepResult)

		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to execute step with React Agent: " + err.Error(),
		})
		return
	}

	// 添加调试信息
	log.Printf("INFO: [PLAN_STEP] React Agent response - Role: %s, Content: %s", message.Role, message.Content)
	if message.Content == "" {
		log.Printf("WARN: [PLAN_STEP] React Agent returned empty content")
	}

	// 更新步骤结果
	stepResult.Status = "success"
	stepResult.ExecutionResult = message.Content
	session.AddStepResult(stepResult)

	duration := time.Since(startTime)
	log.Printf("INFO: [PLAN_STEP] Plan step request completed in %v for user %s", duration, request.UserID)

	c.JSON(consts.StatusOK, utils.H{
		"message": "Step executed successfully with React Agent",
		"step_info": utils.H{
			"step_number":      request.StepNumber,
			"title":            request.Title,
			"description":      request.Description,
			"expected_outcome": request.ExpectedOutcome,
		},
		"execution_result":    message.Content,
		"previous_steps_used": len(session.GetPreviousStepResults(request.StepNumber)),
		"user_id":             request.UserID,
		"session_id":          request.SessionID,
		"duration":            duration.String(),
	})
}

// 获取计划会话状态
func getPlanSessionStatus(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("session_id")
	log.Printf("INFO: [PLAN_SESSION_STATUS] Starting plan session status request: %s %s", c.Method(), c.Request.URI().String())
	startTime := time.Now()

	if sessionID == "" {
		log.Printf("ERROR: [PLAN_SESSION_STATUS] Session ID is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Session ID is required",
		})
		return
	}

	session, exists := getPlanSession(sessionID)
	if !exists {
		log.Printf("ERROR: [PLAN_SESSION_STATUS] Session not found: %s", sessionID)
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "Session not found",
		})
		return
	}

	log.Printf("INFO: [PLAN_SESSION_STATUS] Getting plan session status for session ID: %s", sessionID)

	c.JSON(consts.StatusOK, utils.H{
		"session_id":     session.SessionID,
		"user_id":        session.UserID,
		"original_query": session.OriginalQuery,
		"plan_data":      session.PlanData,
		"step_results":   session.StepResults,
		"created_at":     session.CreatedAt.Format(time.RFC3339),
		"updated_at":     session.UpdatedAt.Format(time.RFC3339),
	})

	duration := time.Since(startTime)
	log.Printf("INFO: [PLAN_SESSION_STATUS] Plan session status request completed in %v for session %s", duration, sessionID)
}

// 获取计划步骤上下文
func getPlanStepContext(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("session_id")
	stepNumber := c.Param("step_number")
	log.Printf("INFO: [PLAN_STEP_CONTEXT] Starting plan step context request: %s %s", c.Method(), c.Request.URI().String())
	startTime := time.Now()

	if sessionID == "" {
		log.Printf("ERROR: [PLAN_STEP_CONTEXT] Session ID is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Session ID is required",
		})
		return
	}

	if stepNumber == "" {
		log.Printf("ERROR: [PLAN_STEP_CONTEXT] Step number is missing")
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Step number is required",
		})
		return
	}

	session, exists := getPlanSession(sessionID)
	if !exists {
		log.Printf("ERROR: [PLAN_STEP_CONTEXT] Session not found: %s", sessionID)
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "Session not found",
		})
		return
	}

	log.Printf("INFO: [PLAN_STEP_CONTEXT] Getting plan step context for session ID: %s, step number: %s", sessionID, stepNumber)

	stepNum, err := strconv.Atoi(stepNumber)
	if err != nil {
		log.Printf("ERROR: [PLAN_STEP_CONTEXT] Invalid step number: %s", stepNumber)
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Invalid step number",
		})
		return
	}

	stepResults := session.GetPreviousStepResults(stepNum)
	if len(stepResults) == 0 {
		log.Printf("INFO: [PLAN_STEP_CONTEXT] No previous step results found for session: %s, step: %s", sessionID, stepNumber)
		c.JSON(consts.StatusOK, utils.H{
			"context": "",
		})
		return
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("\n\n# 当前计划的前置步骤执行结果\n")
	contextBuilder.WriteString("以下是当前执行计划中前面已完成步骤的执行结果，你可以参考这些信息来执行当前步骤：\n\n")

	for _, result := range stepResults {
		contextBuilder.WriteString(fmt.Sprintf("## 步骤 %d: %s\n", result.StepNumber, result.Title))
		contextBuilder.WriteString(fmt.Sprintf("**步骤描述**: %s\n", result.Description))
		contextBuilder.WriteString(fmt.Sprintf("**期望成果**: %s\n", result.ExpectedOutcome))
		contextBuilder.WriteString(fmt.Sprintf("**执行时间**: %s\n", result.ExecutedAt.Format("2006-01-02 15:04:05")))
		contextBuilder.WriteString(fmt.Sprintf("**实际执行结果**:\n%s\n\n", result.ExecutionResult))
		contextBuilder.WriteString("---\n\n")
	}

	contextBuilder.WriteString("请基于以上前置步骤的执行结果来完成当前步骤。如果前面的步骤结果包含了你需要的信息（如数据、文件路径、API响应等），请直接使用这些信息，避免重复执行相同的操作。\n")

	context := contextBuilder.String()

	duration := time.Since(startTime)
	log.Printf("INFO: [PLAN_STEP_CONTEXT] Plan step context request completed in %v for session %s, step %s", duration, sessionID, stepNumber)
	c.JSON(consts.StatusOK, utils.H{
		"context": context,
	})
}
