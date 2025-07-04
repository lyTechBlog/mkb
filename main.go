package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"tos_tool"
	"viking_db_tool"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/cors"
)

const (
	uploadDir   = "./uploads"
	maxFileSize = 100 << 20 // 100MB
)

// getDocTypeByExtension 根据文件后缀确定文档类型
func getDocTypeByExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

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
		return "faq.xlsx"
	}

	// 先检查非结构化类型
	if docType, exists := unstructuredTypes[ext]; exists {
		return docType
	}

	// 再检查结构化类型
	if docType, exists := structuredTypes[ext]; exists {
		return docType
	}

	// 默认返回 txt
	return "txt"
}

func main() {
	// 创建上传目录
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create upload directory: %v", err))
	}

	h := server.Default(server.WithHostPorts("0.0.0.0:8888"))

	// 配置CORS
	h.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 根路径返回前端页面
	h.GET("/mkb", func(ctx context.Context, c *app.RequestContext) {
		c.File("./static/index.html")
	})

	// 静态文件服务
	h.Static("/static", "./static")

	// API路由
	api := h.Group("/api")
	{
		api.POST("/upload", uploadFile)
		api.GET("/files", listFiles)
		api.GET("/files/:filename", downloadFile)
		api.DELETE("/files/:filename", deleteFile)
		api.POST("/chat", chatWithKnowledgeBase)
		api.GET("/documents/status", getDocumentStatus)
	}

	// 健康检查
	h.GET("/health", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, utils.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	fmt.Println("Server starting on http://0.0.0.0:8888 (accessible from external IPs)")
	h.Spin()
}

// 文件上传处理
func uploadFile(ctx context.Context, c *app.RequestContext) {
	// 解析multipart表单
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Failed to parse form: " + err.Error(),
		})
		return
	}

	// 获取用户ID
	userIDs := form.Value["user_id"]
	if len(userIDs) == 0 || userIDs[0] == "" {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}
	userID := userIDs[0]

	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "No file uploaded",
		})
		return
	}

	var uploadedFiles []map[string]interface{}
	for _, file := range files {
		// 检查文件大小
		if file.Size > maxFileSize {
			c.JSON(consts.StatusBadRequest, utils.H{
				"error": fmt.Sprintf("File %s is too large. Max size is %d bytes", file.Filename, maxFileSize),
			})
			return
		}

		// 创建临时文件路径
		filename := filepath.Base(file.Filename)
		tempFilePath := filepath.Join(uploadDir, filename)

		// 打开源文件
		src, err := file.Open()
		if err != nil {
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to open uploaded file: " + err.Error(),
			})
			return
		}
		defer src.Close()

		// 创建临时文件
		dst, err := os.Create(tempFilePath)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to create temporary file: " + err.Error(),
			})
			return
		}

		// 复制文件内容到临时文件
		if _, err = io.Copy(dst, src); err != nil {
			dst.Close()
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to save temporary file: " + err.Error(),
			})
			return
		}
		dst.Close()

		// 调用TOS上传方法，在路径中包含用户ID
		objectKey := fmt.Sprintf("uploads/%s/%s", userID, filename)
		preSignedURL, err := tos_tool.UploadFileWithEnvConfig(tempFilePath, objectKey)
		if err != nil {
			// 清理临时文件
			os.Remove(tempFilePath)
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to upload to TOS: " + err.Error(),
			})
			return
		}

		// 清理临时文件
		os.Remove(tempFilePath)

		uploadedFiles = append(uploadedFiles, map[string]interface{}{
			"name":    filename,
			"url":     preSignedURL,
			"user_id": userID,
		})

		// 检查知识库是否存在
		knowledgeBaseName := "kb_" + userID // 使用用户id作为知识库名称
		project := "default"
		exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to check knowledge base existence: " + err.Error(),
			})
			return
		}

		if !exists {
			// 如果知识库不存在，创建知识库
			createResp, err := viking_db_tool.CreateKnowledgeBase(ctx, knowledgeBaseName, "Knowledge base for user documents", "unstructured_data", project)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, utils.H{
					"error": "Failed to create knowledge base: " + err.Error(),
				})
				return
			}
			if createResp.Code != 0 {
				c.JSON(consts.StatusInternalServerError, utils.H{
					"error": "Failed to create knowledge base: " + createResp.Message,
				})
				return
			}
			resourceID = createResp.Data.ResourceID
			fmt.Printf("Created knowledge base: %s with ResourceID: %s\n", knowledgeBaseName, resourceID)
		} else {
			fmt.Printf("Knowledge base exists: %s with ResourceID: %s\n", knowledgeBaseName, resourceID)
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
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to upload to Viking DB: " + err.Error(),
			})
			return
		}

		fmt.Printf("Uploaded document to Viking DB: %s\n", response)
	}

	c.JSON(consts.StatusOK, utils.H{
		"message": "Files uploaded successfully to TOS",
		"files":   uploadedFiles,
	})
}

// 列出所有文件
func listFiles(ctx context.Context, c *app.RequestContext) {
	// 获取用户ID参数
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}

	// 使用TOS工具列出用户上传的文件
	prefix := fmt.Sprintf("uploads/%s/", userID)
	files, err := tos_tool.ListFilesWithEnvConfig(prefix)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to list files from TOS: " + err.Error(),
		})
		return
	}

	// 为每个文件添加用户ID信息
	for i := range files {
		files[i]["user_id"] = userID
	}

	c.JSON(consts.StatusOK, utils.H{
		"files":   files,
		"user_id": userID,
	})
}

// 下载文件
func downloadFile(ctx context.Context, c *app.RequestContext) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Filename is required",
		})
		return
	}

	filepath := filepath.Join(uploadDir, filename)

	// 检查文件是否存在
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "File not found",
		})
		return
	}

	c.File(filepath)
}

// 删除文件
func deleteFile(ctx context.Context, c *app.RequestContext) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Filename is required",
		})
		return
	}

	// 获取用户ID参数
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}

	// 构建TOS对象键
	objectKey := fmt.Sprintf("uploads/%s/%s", userID, filename)

	// 删除TOS中的文件
	err := tos_tool.DeleteFileWithEnvConfig(objectKey)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to delete file from TOS: " + err.Error(),
		})
		return
	}

	// 检查知识库是否存在
	knowledgeBaseName := "kb_" + userID
	project := "default"
	exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
	if err != nil {
		// 如果检查知识库存在性失败，记录错误但不影响TOS删除的成功响应
		fmt.Printf("Failed to check knowledge base existence: %v\n", err)
		c.JSON(consts.StatusOK, utils.H{
			"message": "File deleted successfully from TOS, but failed to check knowledge base",
		})
		return
	}

	if exists {
		// 生成与上传时相同的docID
		docID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(filename, "")

		// 删除知识库中的文档
		deleteResp, err := viking_db_tool.DeleteDocumentByResourceID(ctx, resourceID, docID)
		if err != nil {
			// 如果删除知识库文档失败，记录错误但不影响TOS删除的成功响应
			fmt.Printf("Failed to delete document from knowledge base: %v\n", err)
			c.JSON(consts.StatusOK, utils.H{
				"message": "File deleted successfully from TOS, but failed to delete from knowledge base",
			})
			return
		}

		if deleteResp.Code != 0 {
			fmt.Printf("Failed to delete document from knowledge base: code %d, message %s\n", deleteResp.Code, deleteResp.Message)
			c.JSON(consts.StatusOK, utils.H{
				"message": "File deleted successfully from TOS, but failed to delete from knowledge base",
			})
			return
		}

		fmt.Printf("Successfully deleted document from knowledge base: %s\n", docID)
		c.JSON(consts.StatusOK, utils.H{
			"message": "File deleted successfully from both TOS and knowledge base",
		})
	} else {
		// 知识库不存在，只删除TOS文件
		c.JSON(consts.StatusOK, utils.H{
			"message": "File deleted successfully from TOS (knowledge base not found)",
		})
	}
}

// 知识库对话处理
func chatWithKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	var request struct {
		UserID   string                        `json:"user_id"`
		Query    string                        `json:"query"`
		Messages []viking_db_tool.MessageParam `json:"messages"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Invalid request format: " + err.Error(),
		})
		return
	}

	if request.UserID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}

	if request.Query == "" {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "Query is required",
		})
		return
	}

	// 检查知识库是否存在
	knowledgeBaseName := "kb_" + request.UserID
	project := "default"
	exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to check knowledge base existence: " + err.Error(),
		})
		return
	}

	if !exists {
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
	searchResp, err := viking_db_tool.SearchKnowledgeWithParams(ctx, searchReq)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to search knowledge base: " + err.Error(),
		})
		return
	}

	if searchResp.Code != 0 {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Knowledge base search failed: " + searchResp.Message,
		})
		return
	}

	// 生成提示词
	prompt, images, err := viking_db_tool.GeneratePrompt(searchResp)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to generate prompt: " + err.Error(),
		})
		return
	}

	// 构建对话消息
	var messages []viking_db_tool.MessageParam
	if len(images) > 0 {
		// 对于Vision模型，需要将图片链接拼接到Message中
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

	// 调用大模型生成回答
	chatResp, err := viking_db_tool.ChatCompletion(ctx, messages)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to generate response: " + err.Error(),
		})
		return
	}

	if chatResp.Code != 0 {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Chat completion failed: " + chatResp.Message,
		})
		return
	}

	// 返回生成的回答
	c.JSON(consts.StatusOK, utils.H{
		"answer": chatResp.Data.GenerateAnswer,
		"usage":  chatResp.Data.Usage,
	})
}

// 查询文档处理状态
func getDocumentStatus(ctx context.Context, c *app.RequestContext) {
	// 获取用户ID参数
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "User ID is required",
		})
		return
	}

	// 检查知识库是否存在
	knowledgeBaseName := "kb_" + userID
	project := "default"
	exists, resourceID, err := viking_db_tool.CheckKnowledgeBaseExists(ctx, knowledgeBaseName, project)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to check knowledge base existence: " + err.Error(),
		})
		return
	}

	if !exists {
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "Knowledge base not found",
		})
		return
	}

	// 获取文档处理状态
	docStatus, err := viking_db_tool.GetDocumentProcessingStatus(ctx, resourceID)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to get document status: " + err.Error(),
		})
		return
	}

	c.JSON(consts.StatusOK, utils.H{
		"document_status": docStatus,
		"user_id":         userID,
	})
}
