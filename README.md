# 文件上传系统

基于 Hertz 框架开发的文件上传和管理系统，提供完整的前后端解决方案。

## 功能特性

- 📤 **文件上传**: 支持拖拽上传、多文件上传
- 📁 **文件管理**: 文件列表显示、下载、删除
- 🎨 **现代化UI**: 响应式设计，美观的用户界面
- ⚡ **高性能**: 基于 Hertz 框架，支持高并发
- 🔒 **安全可靠**: 文件大小限制、类型检查
- 📱 **移动端适配**: 支持手机和平板设备

## 技术栈

### 后端
- **Hertz**: 高性能 Go Web 框架
- **CORS**: 跨域资源共享支持
- **静态文件服务**: 内置前端文件服务

### 前端
- **原生JavaScript**: 无框架依赖
- **现代CSS**: Flexbox、Grid、动画效果
- **响应式设计**: 适配各种屏幕尺寸

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 运行服务器

```bash
go run main.go
```

服务器将在 `http://localhost:8888` 启动

### 3. 访问应用

打开浏览器访问 `http://localhost:8888` 即可使用文件上传系统

## API 接口

### 文件上传
```
POST /api/upload
Content-Type: multipart/form-data

参数:
- file: 要上传的文件（支持多文件）
```

### 获取文件列表
```
GET /api/files

返回:
{
  "files": [
    {
      "name": "example.txt",
      "size": 1024,
      "modTime": "2024-01-01T12:00:00Z"
    }
  ]
}
```

### 下载文件
```
GET /api/files/{filename}
```

### 删除文件
```
DELETE /api/files/{filename}
```

### 健康检查
```
GET /health
```

## 项目结构

```
.
├── main.go              # 后端服务器代码
├── go.mod               # Go 模块文件
├── static/              # 前端静态文件
│   ├── index.html       # 主页面
│   ├── style.css        # 样式文件
│   └── script.js        # JavaScript 代码
├── uploads/             # 文件上传目录（自动创建）
└── README.md           # 项目说明
```

## 配置说明

### 文件大小限制
默认最大文件大小为 100MB，可在 `main.go` 中修改：

```go
const maxFileSize = 100 << 20 // 100MB
```

### 上传目录
默认上传目录为 `./uploads`，可在 `main.go` 中修改：

```go
const uploadDir = "./uploads"
```

### 服务器端口
默认端口为 8888，可在 `main.go` 中修改：

```go
h := server.Default(server.WithHostPorts(":8888"))
```

## 部署说明

### 编译
```bash
go build -o file-upload-server main.go
```

### 运行
```bash
./file-upload-server
```

### 生产环境建议
1. 使用反向代理（如 Nginx）
2. 配置 HTTPS
3. 设置适当的文件权限
4. 考虑使用对象存储服务

## 浏览器兼容性

- Chrome 60+
- Firefox 55+
- Safari 12+
- Edge 79+

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！ 