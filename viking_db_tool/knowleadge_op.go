package viking_db_tool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/volcengine/volc-sdk-golang/base"
)

var AK = "AKLTZmRkY2Q1N2ZkZDlhNDIxNWIzNzUzZmRiNzY5ZGYwM2M"
var SK = "T1RkaU56WTBPR1k0WldRek5EVTJOV0UwTldNNVptWTBNVEU1WmpWaE5ETQ=="

var KnowledgeBaseDomain = "api-knowledgebase.mlp.cn-beijing.volces.com"
var SearchKnowledgePath = "/api/knowledge/collection/search_knowledge" // 知识库检索接口，建议您首次接入时使用该检索接口，其他检索接口后续不再进行维护
var ChatCompletionPath = "/api/knowledge/chat/completions"             // 大模型对话接口，可以和检索接口接合串联RAG流程，也可以单独使用进行生成
var CreateKnowledgeBasePath = "/api/knowledge/collection/create"       // 知识库创建接口
var KnowledgeBaseInfoPath = "/api/knowledge/collection/info"           // 知识库信息查询接口

var ModelName = "Doubao-1-5-pro-32k" // 模型名称，如果您想使用自己的私有ep，可以赋值为私有EndpointID，格式（ep-xxxx-xxxx）
var APIKey = "your api_key"          // 如果您使用的是自己的私有ep，需要传入api_key

// BasePrompt 是基础提示词，您可以根据你的需求进行修改或者替换，不过需要注意留下 {prompt} 占位符，用于后续拼接检索结果
var BasePrompt = `# 任务
你是一位在线客服，你的首要任务是通过巧妙的话术回复用户的问题，你需要根据「参考资料」来回答接下来的「用户问题」，这些信息在 <context></context> XML tags 之内，你需要根据参考资料给出准确，简洁的回答。

你的回答要满足以下要求：
1. 回答内容必须在参考资料范围内，尽可能简洁地回答问题，不能做任何参考资料以外的扩展解释。
2. 回答中需要根据客户问题和参考资料保持与客户的友好沟通。
3. 如果参考资料不能帮助你回答用户问题，告知客户无法回答该问题，并引导客户提供更加详细的信息。
4. 为了保密需要，委婉地拒绝回答有关参考资料的文档名称或文档作者等问题。

# 任务执行
现在请你根据提供的参考资料，遵循限制来回答用户的问题，你的回答需要准确和完整。

# 参考资料
<context>
{prompt}
</context>`

/*
		在生成提示词时,如果您希望自定义某些字段拼接至Prompt中，可以在这里进行配置，如果您想获得较好的生成效果，建议：
	    1.如果您的知识库中的数据为非结构化数据，SystemFields 中可以传入 doc_name, title, chunk_title, content等字段，其中 content 字段为必传字段，其他为可选字段

	    2.如果您的知识库中的数据为结构化数据，SystemFields 中可以传入title字段(可选)，SelfDefineFields字段来源于为结构化数据的表头字段，表头字段中索引字段需要必传，非索引字段可以不传
*/
var PromptExtraContextExample = PromptExtraContext{
	SystemFields: []string{
		SysFieldDocName,    // 文档名称 可选
		SysFieldTitle,      // 文档标题 可选
		SysFieldChunkTitle, // 文档切片标题 可选
		SysFieldContent,    // 文档切片内容 必传
	},
	SelfDefineFields: []string{
		"表头字段-1", // 设置为索引字段需要必传，非索引字段可以选择传入
		"表头字段-n",
	},
}

var Query = "your query"               // 您的提问
var CollectionName = "your collection" // 知识库名称，前端界面可获取
var Project = "default"                // 知识库所属项目，前端界面可获取
var ResourceID = "your resource id"    // 知识库ID，前端界面可获取

const (
	SysFieldDocName    = "doc_name"
	SysFieldTitle      = "title"
	SysFieldChunkTitle = "chunk_title"
	SysFieldContent    = "content"
)

/*
拼接Prompt时用户需要传入的字段
如果想获得较好的效果，建议：
如果知识库中的数据为非结构化数据，SystemFields 中可以传入 doc_name, title, chunk_title, content等字段，其中 content 字段为必传字段，其他为可选字段
如果知识库中的数据为结构化数据，SystemFields 中可以传入title字段(可选)，SelfDefineFields字段来源于为结构化数据的表头字段，表头字段中索引字段需要必传，非索引字段可以不传
*/
type PromptExtraContext struct {
	SelfDefineFields []string `json:"self_define_fields"`
	SystemFields     []string `json:"system_fields"`
}

/*
检索接口请求参数
*/
type CollectionSearchKnowledgeRequest struct {
	Name           string          `json:"name,omitempty"`
	Project        string          `json:"project,omitempty"`
	ResourceId     string          `json:"resource_id,omitempty"`
	Query          string          `json:"query"`
	Limit          int32           `json:"limit"`
	QueryParam     *QueryParamInfo `json:"query_param"`
	DenseWeight    float32         `json:"dense_weight"`
	MdSearch       bool            `json:"md_search"`
	Preprocessing  PreProcessing   `json:"pre_processing,omitempty"`
	Postprocessing PostProcessing  `json:"post_processing,omitempty"`
}

type QueryParamInfo struct {
	DocFilter interface{} `json:"doc_filter"`
}

type MessageParam struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}
type ChatCompletionMessageContent struct {
	StringValue *string
	ListValue   []*ChatCompletionMessageContentPart
}

type ChatMessageImageURL struct {
	URL string `json:"url,omitempty"`
}

type ChatCompletionMessageContentPartType string

const (
	ChatCompletionMessageContentPartTypeText     ChatCompletionMessageContentPartType = "text"
	ChatCompletionMessageContentPartTypeImageURL ChatCompletionMessageContentPartType = "image_url"
)

type ChatCompletionMessageContentPart struct {
	Type     ChatCompletionMessageContentPartType `json:"type,omitempty"`
	Text     string                               `json:"text,omitempty"`
	ImageURL *ChatMessageImageURL                 `json:"image_url,omitempty"`
}

/*
检索接口预处理参数Part，详细介绍见官方文档
*/
type PreProcessing struct {
	NeedInstruction  bool           `json:"need_instruction"`
	Rewrite          bool           `json:"rewrite"`
	Messages         []MessageParam `json:"messages"`
	ReturnTokenUsage bool           `json:"return_token_usage"`
}

/*
检索接口后处理参数Part，详细介绍见官方文档
*/
type PostProcessing struct {
	RerankSwitch        bool                   `json:"rerank_switch"`
	RerankModel         string                 `json:"rerank_model,omitempty"`
	RerankOnlyChunk     bool                   `json:"rerank_only_chunk"`
	RetrieveCount       int32                  `json:"retrieve_count"`
	EndpointID          string                 `json:"endpoint_id"`
	ChunkDiffusionCount int32                  `json:"chunk_diffusion_count"`
	ChunkGroup          bool                   `json:"chunk_group"`
	ChunkScoreAggrType  string                 `json:"chunk_score_aggr_type,omitempty"`
	ChunkExtraContent   map[string]interface{} `json:"chunk_extra_content"`
	GetAttachmentLink   bool                   `json:"get_attachment_link"`
}

/*
检索接口返回参数结构体，详细介绍见官方文档
*/
type CollectionSearchKnowledgeResponse struct {
	Code    int64                                  `json:"code"`
	Message string                                 `json:"message,omitempty"`
	Data    *CollectionSearchKnowledgeResponseData `json:"data,omitempty"`
}
type CollectionSearchKnowledgeResponseData struct {
	CollectionName string                          `json:"collection_name"`
	Count          int32                           `json:"count"`
	RewriteQuery   string                          `json:"rewrite_query,omitempty"` // 改写后的query,如果是多轮对话的首次请求，该字段为空，表示不改写，从第二个问题开始进行改写
	TokenUsage     *TotalTokenUsage                `json:"token_usage,omitempty"`
	ResultList     []*CollectionSearchResponseItem `json:"result_list,omitempty"`
}

/*
检索接口各个阶段模型调用量详情，详细介绍见官方文档
*/
type TotalTokenUsage struct {
	EmbeddingUsage *ModelTokenUsage `json:"embedding_token_usage,omitempty"`
	RerankUsage    *int64           `json:"rerank_token_usage,omitempty"`
	LLMUsage       *ModelTokenUsage `json:"llm_token_usage,omitempty"`
	RewriteUsage   *ModelTokenUsage `json:"rewrite_token_usage,omitempty"`
}

/*
检索接口返回切片的详情，详细介绍见官方文档
*/
type CollectionSearchResponseItem struct {
	Id                  string                              `json:"id"`
	Content             string                              `json:"content"`
	MdContent           string                              `json:"md_content,omitempty"`
	Score               float64                             `json:"score"`
	PointId             string                              `json:"point_id"`
	OriginText          string                              `json:"origin_text,omitempty"`
	OriginalQuestion    string                              `json:"original_question,omitempty"`
	ChunkTitle          string                              `json:"chunk_title,omitempty"`
	ChunkId             int                                 `json:"chunk_id"`
	ProcessTime         int64                               `json:"process_time"`
	RerankScore         float64                             `json:"rerank_score,omitempty"`
	DocInfo             CollectionSearchResponseItemDocInfo `json:"doc_info,omitempty"`
	RecallPosition      int32                               `json:"recall_position"`
	RerankPosition      int32                               `json:"rerank_position,omitempty"`
	ChunkType           string                              `json:"chunk_type,omitempty"`
	ChunkSource         string                              `json:"chunk_source,omitempty"`
	UpdateTime          int64                               `json:"update_time"`
	ChunkAttachmentList []ChunkAttachment                   `json:"chunk_attachment,omitempty"`
	TableChunkFields    []PointTableChunkField              `json:"table_chunk_fields,omitempty"`
	OriginalCoordinate  *ChunkPositions                     `json:"original_coordinate,omitempty"`
}

type CollectionSearchResponseItemDocInfo struct {
	Docid      string `json:"doc_id"`
	DocName    string `json:"doc_name"`
	CreateTime int64  `json:"create_time"`
	DocType    string `json:"doc_type"`
	DocMeta    string `json:"doc_meta,omitempty"`
	Source     string `json:"source"`
	Title      string `json:"title,omitempty"`
}

type ChunkAttachment struct {
	UUID    string `json:"uuid,omitempty"`
	Caption string `json:"caption"`
	Type    string `json:"type"`
	Link    string `json:"link,omitempty"`
}

type PointTableChunkField struct {
	FieldName  string      `json:"field_name"`
	FieldValue interface{} `json:"field_value"`
}

type ChunkPositions struct {
	PageNo []int       `json:"page_no"`
	BBox   [][]float64 `json:"bbox"`
}

/*
	模型生成(LLM)请求参数结构体，详细介绍见官方文档
*/

type CollectionChatCompletionRequest struct {
	Model            string         `json:"model"`
	ModelVersion     string         `json:"model_version"`
	APIKey           string         `json:"api_key"`
	Messages         []MessageParam `json:"messages"`
	MaxTokens        int32          `json:"max_tokens"`
	Temperature      float32        `json:"temperature"`
	Stream           bool           `json:"stream"`
	ReturnTokenUsage bool           `json:"return_token_usage"`
}

/*
	模型生成(LLM)请求参数结构体，详细介绍见官方文档
*/

type CollectionChatCompletionResponse struct {
	Code    int64                                 `json:"code"`
	Message string                                `json:"message,omitempty"`
	Data    *CollectionChatCompletionResponseData `json:"data,omitempty"`
}

type CollectionChatCompletionResponseData struct {
	GenerateAnswer   string `json:"generated_answer"` // 模型生成文本
	Usage            string `json:"usage"`
	ReasoningContent string `json:"reasoning_content,omitempty"` //模型推理过程内容，仅推理模型会有
}

type ModelTokenUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`     // 请求文本的分词数
	CompletionTokens int64 `json:"completion_tokens"` // 生成文本的分词数, 对话模型才有值, 其他模型都是0
	TotalTokens      int64 `json:"total_tokens"`      // PromptTokens + CompletionTokens
}

/*
知识库创建请求参数结构体
*/
type CreateKnowledgeBaseRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     int    `json:"version"`
	Project     string `json:"project"`
}

type IndexConfigDetails struct {
	Fields             []string `json:"fields"`
	Quant              string   `json:"quant"`
	CPUQuota           int      `json:"cpu_quota"`
	EmbeddingModel     string   `json:"embedding_model"`
	EmbeddingDimension int      `json:"embedding_dimension"`
}

type TableField struct {
	FieldType   string `json:"field_type"`
	FieldName   string `json:"field_name"`
	IfEmbedding bool   `json:"if_embedding"`
	IfFilter    bool   `json:"if_filter"`
}

/*
知识库创建响应参数结构体
*/
type CreateKnowledgeBaseResponse struct {
	Code    int64                            `json:"code"`
	Message string                           `json:"message,omitempty"`
	Data    *CreateKnowledgeBaseResponseData `json:"data,omitempty"`
}

type CreateKnowledgeBaseResponseData struct {
	ResourceID string `json:"resource_id"`
	Name       string `json:"name"`
	Project    string `json:"project"`
}

/*
知识库信息查询请求参数结构体
*/
type KnowledgeBaseInfoRequest struct {
	Name    string `json:"name"`
	Project string `json:"project"`
}

/*
知识库信息查询响应参数结构体
*/
type KnowledgeBaseInfoResponse struct {
	Code    int64                          `json:"code"`
	Message string                         `json:"message,omitempty"`
	Data    *KnowledgeBaseInfoResponseData `json:"data,omitempty"`
}

type KnowledgeBaseInfoResponseData struct {
	ResourceID  string `json:"resource_id"`
	Name        string `json:"name"`
	Project     string `json:"project"`
	Description string `json:"description"`
	Version     int    `json:"version"`
	CreateTime  int64  `json:"create_time"`
	UpdateTime  int64  `json:"update_time"`
}

func ParseJsonUseNumber(input []byte, target interface{}) error {
	var d *json.Decoder
	var err error
	d = json.NewDecoder(bytes.NewBuffer(input))
	if d == nil {
		return fmt.Errorf("ParseJsonUseNumber init NewDecoder failed")
	}
	d.UseNumber()
	err = d.Decode(&target)
	if err != nil {
		return fmt.Errorf("ParseJsonUseNumber Decode failed, err: %s", err.Error())
	}
	return nil
}

func SerializeToJsonBytesUseNumber(source interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	err := encoder.Encode(source)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// scanDoubleCRLF 是一个 bufio.SplitFunc，用于分隔 \r\n\r\n
func scanDoubleCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// 查找 \r\n\r\n 分隔符
	if i := bytes.Index(data, []byte("\r\n\r\n")); i >= 0 {
		// 返回位置后的分隔符
		return i + 4, data[0:i], nil
	}
	if atEOF && strings.Contains(string(data), "\"end\":true") {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// isVisionModel 检查是否是视觉模型
func isVisionModel(modelName string) bool {
	return strings.Contains(modelName, "vision")
}

func PrepareRequest(method string, path string, body []byte) *http.Request {
	u := url.URL{
		Scheme: "https",
		Host:   KnowledgeBaseDomain,
		Path:   path,
	}
	req, _ := http.NewRequest(strings.ToUpper(method), u.String(), bytes.NewReader(body))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Host", KnowledgeBaseDomain)
	credential := base.Credentials{
		AccessKeyID:     AK,
		SecretAccessKey: SK,
		Service:         "air",
		Region:          "cn-north-1",
	}
	req = credential.Sign(req)
	return req
}

/*
		知识库检索请求参数生成：以下详细展示了部分参数的传递规则，其余参数请参考官方接口文档，如果您想快速接入进行测试，可以只传入参数集合的最小集，其余参数可以使用默认值。
	   	必传参数如下：
	   	1. resourceId (也可使用resource_id 或 name + project, 二选一)
	   	2. query：用户问题
*/
func GenerateSearchKnowledgeReqParams() CollectionSearchKnowledgeRequest {
	return CollectionSearchKnowledgeRequest{
		Name:    CollectionName, // 知识库名称
		Project: Project,        // 知识库项目名称
		//ResourceId:  ResourceID,     // 知识库resource_id (二选一，查询时，可使用resource_id 或 name + project)
		Query:       Query, // 用户问题
		Limit:       10,    // 返回数量, 不传递默认返回10条
		DenseWeight: 0.5,   //混合搜索的权重
		Preprocessing: PreProcessing{
			NeedInstruction:  true,
			ReturnTokenUsage: true,
			Rewrite:          false, // 问题改写开关，默认不开启
			Messages: []MessageParam{ // 仅在使用改写或意图识别时需要传且必传Messages
				{
					Role:    "system",
					Content: ChatCompletionMessageContent{},
				},
				{
					Role: "user",
					Content: ChatCompletionMessageContent{
						StringValue: &Query,
					},
				},
			},
		},
		Postprocessing: PostProcessing{
			RerankSwitch:        false, // 重排开关，默认不开启
			RetrieveCount:       25,    //进入重排的切片数量，重排打开时生效，需要大于limit,当limit=10，默认值为25
			GetAttachmentLink:   true,  // 是否返回原始图片，仅当创建知识库开启 OCR 时生效，否则自动跳过图片
			ChunkGroup:          true,  //是否对召回切片按照文档进行聚合
			ChunkDiffusionCount: 0,     //切片扩散数量-是否召回切片的临近切片，如 1 代表额外召回当前切片的上下各一个切片
		},
	}
}

func SearchKnowledge(ctx context.Context) (*CollectionSearchKnowledgeResponse, error) {
	searchKnowledgeReqParams := GenerateSearchKnowledgeReqParams()
	searchKnowledgeReqParamsBytes, err := SerializeToJsonBytesUseNumber(searchKnowledgeReqParams)
	if err != nil {
		return nil, err
	}
	req := PrepareRequest("POST", SearchKnowledgePath, searchKnowledgeReqParamsBytes)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var searchKnowledgeResp *CollectionSearchKnowledgeResponse
	err = ParseJsonUseNumber(body, &searchKnowledgeResp)
	if err != nil {
		return nil, err
	}
	return searchKnowledgeResp, nil
}

// SearchKnowledgeWithParams 使用自定义参数进行知识库检索
func SearchKnowledgeWithParams(ctx context.Context, searchReq CollectionSearchKnowledgeRequest) (*CollectionSearchKnowledgeResponse, error) {
	searchReqBytes, err := SerializeToJsonBytesUseNumber(searchReq)
	if err != nil {
		return nil, err
	}
	req := PrepareRequest("POST", SearchKnowledgePath, searchReqBytes)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var searchKnowledgeResp *CollectionSearchKnowledgeResponse
	err = ParseJsonUseNumber(body, &searchKnowledgeResp)
	if err != nil {
		return nil, err
	}
	return searchKnowledgeResp, nil
}

func GenerateChatCompletionReqParams(stream bool, messages []MessageParam) *CollectionChatCompletionRequest {
	return &CollectionChatCompletionRequest{
		Model:            ModelName, // 如果使用私有ep，此处替换为私有ep即可，格式 ep-xxx-xxx
		ModelVersion:     "",        // 模型版本，使用公有接入点时，可以选择指定模型版本，不指定则服务会自动指定默认版本
		Stream:           stream,    // 模型结果是否流式返回
		ReturnTokenUsage: true,      // 是否返回token使用情况
		MaxTokens:        4096,      // 最大token数
		Temperature:      0.7,       // 模型温度,取值范围0~1，值越大随机性越大
		APIKey:           APIKey,    // 使用私有ep时，必须传递此参数才能生效
		Messages:         messages,  // 模型对话信息
	}
}

// 非流式调用
func ChatCompletion(ctx context.Context, messages []MessageParam) (*CollectionChatCompletionResponse, error) {
	chatCompletionReqParams := GenerateChatCompletionReqParams(false, messages)
	chatCompletionReqParamsBytes, err := SerializeToJsonBytesUseNumber(chatCompletionReqParams)
	if err != nil {
		return nil, err
	}

	request := PrepareRequest("POST", ChatCompletionPath, chatCompletionReqParamsBytes)
	client := &http.Client{
		Timeout: time.Second * 120,
	}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var chatCompletionResp *CollectionChatCompletionResponse
	err = ParseJsonUseNumber(body, &chatCompletionResp)
	if err != nil {
		return nil, err
	}
	return chatCompletionResp, nil
}

// 流式调用
func ChatCompletionStream(ctx context.Context, messages []MessageParam) (answer string, usage *ModelTokenUsage, err error) {
	chatCompletionReqParams := GenerateChatCompletionReqParams(true, messages)
	chatCompletionReqParamsBytes, err := SerializeToJsonBytesUseNumber(chatCompletionReqParams)
	if err != nil {
		return "", nil, err
	}

	request := PrepareRequest("POST", ChatCompletionPath, chatCompletionReqParamsBytes)
	client := &http.Client{
		Timeout: time.Second * 120,
	}
	request.Header.Set("Accept", "text/event-stream")
	resp, err := client.Do(request)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	// 读取流式返回
	scanner := bufio.NewScanner(resp.Body)
	// 指定分隔符函数
	scanner.Split(scanDoubleCRLF)

	var answerBuilder strings.Builder
	var modelTokenUsage ModelTokenUsage

	buf := make([]byte, 0, 150*1024)
	scanner.Buffer(buf, 150*1024) // 可以按需调整scanner的大小

	// 读取数据
	for scanner.Scan() {
		streamLine := scanner.Text()
		if len(streamLine) < 5 {
			continue
		}
		streamJson := streamLine[5:]
		var chatCompletionResponse CollectionChatCompletionResponse
		err := ParseJsonUseNumber([]byte(streamJson), &chatCompletionResponse)
		if err != nil {
			return "", nil, err
		}
		// 获取流式返回的内容
		fmt.Println(chatCompletionResponse.Data.GenerateAnswer)

		answerBuilder.WriteString(chatCompletionResponse.Data.GenerateAnswer)

		// 最后一条流式返回中，携带本次请求token使用信息
		if chatCompletionResponse.Data.Usage != "" {
			err := ParseJsonUseNumber([]byte(chatCompletionResponse.Data.Usage), &modelTokenUsage)
			if err != nil {
				return "", nil, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	return answerBuilder.String(), &modelTokenUsage, nil
}

// getContentForPrompt 生成内容提示
func getContentForPrompt(item *CollectionSearchResponseItem, imageNum int) string {
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

func GeneratePrompt(resp *CollectionSearchKnowledgeResponse) (string, []string, error) {
	if resp == nil {
		return "", nil, fmt.Errorf("response is nil")
	}
	if resp.Code != 0 {
		return "", nil, fmt.Errorf(resp.Message)
	}

	var promptBuilder strings.Builder
	var imageURLs []string
	usingVLM := isVisionModel(ModelName)
	imageCnt := 0

	for _, point := range resp.Data.ResultList {
		// 对vision模型需要额外处理图片链接
		if usingVLM && len(point.ChunkAttachmentList) > 0 {
			link := point.ChunkAttachmentList[0].Link
			if link != "" {
				imageURLs = append(imageURLs, link)
				imageCnt++
			}
		}

		// 处理系统字段
		docInfo := point.DocInfo

		// 拼接用户指定的系统字段
		for _, sysField := range PromptExtraContextExample.SystemFields {
			switch sysField {
			case SysFieldDocName:
				promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", sysField, docInfo.DocName))
			case SysFieldTitle:
				promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", sysField, docInfo.Title))
			case SysFieldChunkTitle:
				promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", sysField, point.ChunkTitle))
			case SysFieldContent:
				promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", sysField, getContentForPrompt(point, imageCnt)))
			}
		}

		// 结构化数据- 拼接用户指定的自定义字段
		for _, selfField := range PromptExtraContextExample.SelfDefineFields {
			for _, tableChunkField := range point.TableChunkFields {
				if tableChunkField.FieldName == selfField {
					promptBuilder.WriteString(fmt.Sprintf("%s: %v\n", tableChunkField.FieldName, tableChunkField.FieldValue))
				}
			}
		}
		promptBuilder.WriteString("---\n")
	}

	// 基础提示词模板替换
	finalPrompt := strings.Replace(BasePrompt, "{prompt}", promptBuilder.String(), -1)
	return finalPrompt, imageURLs, nil
}

// RAG 检索增强生成流程串联
func RAG(ctx context.Context, stream bool) error {
	// 知识库检索
	searchResp, err := SearchKnowledge(ctx)
	if err != nil {
		return err
	}

	// 生成提示词
	prompt, images, err := GeneratePrompt(searchResp)
	if err != nil {
		return err
	}
	fmt.Printf("提示词：%s\n", prompt)

	// 生成Chat的Message结构体，拼接message对话, 问题对应role为user，系统对应role为system, 答案对应role为assistant, 内容对应content
	var messages []MessageParam
	if len(images) > 0 {
		// 对于Vision模型，需要将图片链接拼接到Message中
		var multiModalMessage []*ChatCompletionMessageContentPart
		multiModalMessage = append(multiModalMessage, &ChatCompletionMessageContentPart{
			Type: ChatCompletionMessageContentPartTypeText,
			Text: Query,
		})
		for _, imageURL := range images {
			multiModalMessage = append(multiModalMessage, &ChatCompletionMessageContentPart{
				Type:     ChatCompletionMessageContentPartTypeImageURL,
				ImageURL: &ChatMessageImageURL{URL: imageURL},
			})
		}

		messages = []MessageParam{
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
		// 如果使用的是普通的文本LLM模型，使用该分支拼接生成message
		messages = []MessageParam{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: Query,
			},
		}
	}

	if stream {
		// 流式调用
		answer, usage, err := ChatCompletionStream(ctx, messages)
		if err != nil {
			return err
		}
		fmt.Printf("大模型流式调用返回结果：%s\n", answer)
		fmt.Printf("大模型流式调用返回token使用情况：%+v\n", usage)
	} else {
		// 非流式调用
		ChatCompletionResponse, err := ChatCompletion(ctx, messages)
		if err != nil {
			return err
		}
		if ChatCompletionResponse.Code != 0 {
			return fmt.Errorf(ChatCompletionResponse.Message)
		}

		answer := ChatCompletionResponse.Data.GenerateAnswer
		fmt.Printf("非流式大模型返回结果：%s\n", answer)

		var modelTokenUsage ModelTokenUsage
		err = ParseJsonUseNumber([]byte(ChatCompletionResponse.Data.Usage), &modelTokenUsage)
		if err != nil {
			return err
		}
		fmt.Printf("非流式大模型返回token使用情况：%+v\n", modelTokenUsage)
	}
	return nil
}

/*
知识库创建函数
*/
func CreateKnowledgeBase(ctx context.Context, name, description, dataType, project string) (*CreateKnowledgeBaseResponse, error) {
	// 构建创建知识库的请求参数
	createReq := CreateKnowledgeBaseRequest{
		Name:        name,
		Description: description,
		Version:     2, // 使用标准版本，不支持自定义索引配置
		Project:     project,
	}

	// 序列化请求参数
	createReqBytes, err := SerializeToJsonBytesUseNumber(createReq)
	if err != nil {
		return nil, err
	}

	// 准备HTTP请求
	req := PrepareRequest("POST", CreateKnowledgeBasePath, createReqBytes)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var createResp *CreateKnowledgeBaseResponse
	err = ParseJsonUseNumber(body, &createResp)
	if err != nil {
		return nil, err
	}

	return createResp, nil
}

/*
创建结构化数据知识库的辅助函数
*/
func CreateStructuredDataKnowledgeBase(ctx context.Context, name, description, project string) (*CreateKnowledgeBaseResponse, error) {
	// 调用创建知识库函数
	return CreateKnowledgeBase(ctx, name, description, "structured_data", project)
}

/*
获取知识库信息
*/
func GetKnowledgeBaseInfo(ctx context.Context, name, project string) (*KnowledgeBaseInfoResponse, error) {
	// 构建请求参数
	infoReq := KnowledgeBaseInfoRequest{
		Name:    name,
		Project: project,
	}

	// 序列化请求参数
	infoReqBytes, err := SerializeToJsonBytesUseNumber(infoReq)
	if err != nil {
		return nil, err
	}

	// 准备HTTP请求
	req := PrepareRequest("POST", KnowledgeBaseInfoPath, infoReqBytes)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var infoResp *KnowledgeBaseInfoResponse
	err = ParseJsonUseNumber(body, &infoResp)
	if err != nil {
		return nil, err
	}

	return infoResp, nil
}

/*
检查知识库是否存在
*/
func CheckKnowledgeBaseExists(ctx context.Context, name, project string) (bool, string, error) {
	infoResp, err := GetKnowledgeBaseInfo(ctx, name, project)
	if err != nil {
		return false, "", err
	}

	// 如果返回码为0，说明知识库存在
	if infoResp.Code == 0 {
		return true, infoResp.Data.ResourceID, nil
	}

	// 如果返回码不为0，说明知识库不存在或查询失败
	return false, "", nil
}

/*
文档列表查询请求参数结构体
*/
type DocumentListRequest struct {
	CollectionName string   `json:"collection_name,omitempty"`
	Project        string   `json:"project,omitempty"`
	ResourceID     string   `json:"resource_id,omitempty"`
	Offset         int      `json:"offset,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	DocIDs         []string `json:"doc_ids,omitempty"`
}

/*
文档列表查询响应参数结构体
*/
type DocumentListResponse struct {
	Code    int64                     `json:"code"`
	Message string                    `json:"message,omitempty"`
	Data    *DocumentListResponseData `json:"data,omitempty"`
}

type DocumentListResponseData struct {
	CollectionName string         `json:"collection_name"`
	TotalNum       int            `json:"total_num"`
	Count          int            `json:"count"`
	DocList        []DocumentInfo `json:"doc_list"`
}

type DocumentInfo struct {
	CollectionName string                   `json:"collection_name"`
	DocName        string                   `json:"doc_name"`
	DocID          string                   `json:"doc_id"`
	AddType        string                   `json:"add_type"`
	DocType        string                   `json:"doc_type"`
	CreateTime     int64                    `json:"create_time"`
	AddedBy        string                   `json:"added_by"`
	UpdateTime     int64                    `json:"update_time"`
	URL            string                   `json:"url,omitempty"`
	PointNum       int                      `json:"point_num"`
	Status         DocumentProcessingStatus `json:"status"`
	TotalTokens    int64                    `json:"total_tokens,omitempty"`
}

type DocumentPointInfo struct {
	CollectionName string                   `json:"collection_name"`
	PointID        string                   `json:"point_id"`
	ProcessTime    int64                    `json:"process_time"`
	Content        string                   `json:"content"`
	ChunkTitle     string                   `json:"chunk_title,omitempty"`
	DocInfo        DocumentPointInfoDocInfo `json:"doc_info"`
}

type DocumentPointInfoDocInfo struct {
	DocID      string `json:"doc_id"`
	DocName    string `json:"doc_name"`
	CreateTime int64  `json:"create_time"`
	DocType    string `json:"doc_type"`
	DocMeta    string `json:"doc_meta,omitempty"`
	Source     string `json:"source"`
	Title      string `json:"title,omitempty"`
}

const (
	DocumentListPath = "/api/knowledge/doc/list"
	DocumentInfoPath = "/api/knowledge/doc/info"
)

/*
文档信息查询请求参数结构体
*/
type DocumentInfoRequest struct {
	CollectionName   string `json:"collection_name,omitempty"`
	Project          string `json:"project,omitempty"`
	ResourceID       string `json:"resource_id,omitempty"`
	DocID            string `json:"doc_id"`
	ReturnTokenUsage bool   `json:"return_token_usage,omitempty"`
}

/*
文档信息查询响应参数结构体
*/
type DocumentInfoResponse struct {
	Code    int64                     `json:"code"`
	Message string                    `json:"message,omitempty"`
	Data    *DocumentInfoResponseData `json:"data,omitempty"`
}

type DocumentInfoResponseData struct {
	CollectionName string                   `json:"collection_name"`
	DocName        string                   `json:"doc_name"`
	DocID          string                   `json:"doc_id"`
	AddType        string                   `json:"add_type"`
	DocType        string                   `json:"doc_type"`
	CreateTime     int64                    `json:"create_time"`
	AddedBy        string                   `json:"added_by"`
	UpdateTime     int64                    `json:"update_time"`
	URL            string                   `json:"url,omitempty"`
	PointNum       int                      `json:"point_num"`
	Status         DocumentProcessingStatus `json:"status"`
	TotalTokens    int64                    `json:"total_tokens,omitempty"`
}

type DocumentProcessingStatus struct {
	ProcessStatus int `json:"process_status"`
}

// DocumentStatusInfo 表示文档处理状态的详细信息
type DocumentStatusInfo struct {
	DocID         string `json:"doc_id"`
	DocName       string `json:"doc_name"`
	ProcessStatus int    `json:"process_status"`
	StatusText    string `json:"status_text"`
	IsCompleted   bool   `json:"is_completed"`
}

// getStatusText 根据process_status返回对应的状态文本
func getStatusText(status int) string {
	switch status {
	case 0:
		return "处理完成"
	case 1:
		return "处理失败"
	case 2, 3:
		return "排队中"
	case 5:
		return "删除中"
	case 6:
		return "处理中"
	default:
		return "未知状态"
	}
}

/*
查询单个文档信息
*/
func GetDocumentInfo(ctx context.Context, req DocumentInfoRequest) (*DocumentInfoResponse, error) {
	// 序列化请求参数
	reqBytes, err := SerializeToJsonBytesUseNumber(req)
	if err != nil {
		return nil, err
	}

	// 准备HTTP请求
	httpReq := PrepareRequest("POST", DocumentInfoPath, reqBytes)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var infoResp DocumentInfoResponse
	err = ParseJsonUseNumber(body, &infoResp)
	if err != nil {
		return nil, err
	}

	return &infoResp, nil
}

/*
检查文档是否处理完成 - 使用 doc/info 接口
*/
func CheckDocumentProcessingStatus(ctx context.Context, resourceID, docID string) (bool, error) {
	req := DocumentInfoRequest{
		ResourceID: resourceID,
		DocID:      docID,
	}

	resp, err := GetDocumentInfo(ctx, req)
	if err != nil {
		return false, err
	}

	// 如果返回码不为0，说明查询失败
	if resp.Code != 0 {
		return false, fmt.Errorf("query failed with code %d: %s", resp.Code, resp.Message)
	}

	// 根据文档状态判断是否处理完成
	// process_status: 0-处理完成, 1-处理失败, 2或3-排队中, 5-删除中, 6-处理中
	return resp.Data.Status.ProcessStatus == 0, nil
}

/*
获取知识库中所有文档的处理状态 - 直接从文档列表响应中获取
*/
func GetDocumentProcessingStatus(ctx context.Context, resourceID string) ([]DocumentStatusInfo, error) {
	// 获取文档列表
	req := DocumentListRequest{
		ResourceID: resourceID,
		Limit:      100, // 获取足够多的结果
	}

	resp, err := GetDocumentList(ctx, req)
	if err != nil {
		return nil, err
	}

	// 如果返回码不为0，说明查询失败
	if resp.Code != 0 {
		return nil, fmt.Errorf("query failed with code %d: %s", resp.Code, resp.Message)
	}

	// 构建文档处理状态列表
	var docStatusList []DocumentStatusInfo

	// 直接从文档列表中获取处理状态
	for _, doc := range resp.Data.DocList {
		statusInfo := DocumentStatusInfo{
			DocID:         doc.DocID,
			DocName:       doc.DocName,
			ProcessStatus: doc.Status.ProcessStatus,
			StatusText:    getStatusText(doc.Status.ProcessStatus),
			IsCompleted:   doc.Status.ProcessStatus == 0, // 0表示处理完成
		}
		docStatusList = append(docStatusList, statusInfo)
	}

	return docStatusList, nil
}

/*
查询知识库中的文档列表
*/
func GetDocumentList(ctx context.Context, req DocumentListRequest) (*DocumentListResponse, error) {
	// 序列化请求参数
	reqBytes, err := SerializeToJsonBytesUseNumber(req)
	if err != nil {
		return nil, err
	}

	// 准备HTTP请求
	httpReq := PrepareRequest("POST", DocumentListPath, reqBytes)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var listResp DocumentListResponse
	err = ParseJsonUseNumber(body, &listResp)
	if err != nil {
		return nil, err
	}

	return &listResp, nil
}

/*
func main() {
	ctx := context.Background()

	// 创建知识库示例
	createResp, err := CreateStructuredDataKnowledgeBase(ctx, "apiexample", "test", "default")
	if err != nil {
		fmt.Printf("create knowledge base failed: %v\n", err)
		return
	}
	if createResp.Code != 0 {
		fmt.Printf("create knowledge base failed: %s\n", createResp.Message)
		return
	}
	fmt.Printf("知识库创建成功，ResourceID: %s, Name: %s, Project: %s\n",
		createResp.Data.ResourceID, createResp.Data.Name, createResp.Data.Project)

	// 仅使用知识库检索
	//searchResp, err := SearchKnowledge(ctx)
	//if err != nil {
	//	fmt.Printf("search knowledge failed: %v", err)
	//	return
	//}
	//searchRespStr, _ := SerializeToJsonBytesUseNumber(searchResp)
	//fmt.Printf("知识库检索结果: %v", string(searchRespStr))

	// RAG流程-非流式
	//err := RAG(ctx, false)
	//if err != nil {
	//	fmt.Errorf("RAG err: %v", err)
	//	return
	//}

	// RAG流程-流式
	//err := RAG(ctx, true)
	//if err != nil {
	//	fmt.Errorf("RAG err: %v", err)
	//	return
	//}
}
*/
