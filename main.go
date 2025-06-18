package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/cors"
)

const (
	uploadDir = "./uploads"
	maxFileSize = 100 << 20 // 100MB
)

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

	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(consts.StatusBadRequest, utils.H{
			"error": "No file uploaded",
		})
		return
	}

	var uploadedFiles []string
	for _, file := range files {
		// 检查文件大小
		if file.Size > maxFileSize {
			c.JSON(consts.StatusBadRequest, utils.H{
				"error": fmt.Sprintf("File %s is too large. Max size is %d bytes", file.Filename, maxFileSize),
			})
			return
		}

		// 创建目标文件路径
		filename := filepath.Base(file.Filename)
		filepath := filepath.Join(uploadDir, filename)

		// 打开源文件
		src, err := file.Open()
		if err != nil {
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to open uploaded file: " + err.Error(),
			})
			return
		}
		defer src.Close()

		// 创建目标文件
		dst, err := os.Create(filepath)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to create file: " + err.Error(),
			})
			return
		}
		defer dst.Close()

		// 复制文件内容
		if _, err = io.Copy(dst, src); err != nil {
			c.JSON(consts.StatusInternalServerError, utils.H{
				"error": "Failed to save file: " + err.Error(),
			})
			return
		}

		uploadedFiles = append(uploadedFiles, filename)
	}

	c.JSON(consts.StatusOK, utils.H{
		"message": "Files uploaded successfully",
		"files":   uploadedFiles,
	})
}

// 列出所有文件
func listFiles(ctx context.Context, c *app.RequestContext) {
	files, err := os.ReadDir(uploadDir)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to read upload directory: " + err.Error(),
		})
		return
	}

	var fileList []map[string]interface{}
	for _, file := range files {
		if !file.IsDir() {
			info, err := file.Info()
			if err != nil {
				continue
			}
			fileList = append(fileList, map[string]interface{}{
				"name":    file.Name(),
				"size":    info.Size(),
				"modTime": info.ModTime().Format(time.RFC3339),
			})
		}
	}

	c.JSON(consts.StatusOK, utils.H{
		"files": fileList,
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

	filepath := filepath.Join(uploadDir, filename)
	
	// 检查文件是否存在
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		c.JSON(consts.StatusNotFound, utils.H{
			"error": "File not found",
		})
		return
	}

	// 删除文件
	if err := os.Remove(filepath); err != nil {
		c.JSON(consts.StatusInternalServerError, utils.H{
			"error": "Failed to delete file: " + err.Error(),
		})
		return
	}

	c.JSON(consts.StatusOK, utils.H{
		"message": "File deleted successfully",
	})
} 