package admin

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	// Hot collections - 高频访问数据，用 HNSW 索引（速度最快）
	usersHotCollectionName = "admin_users_hot"     // 最活跃的百万用户
	docsHotCollectionName  = "admin_documents_hot" // 最近100M文档（7天内）

	// Cold collections - 冷数据，用 IVF_SQ8 索引（省内存）
	docsColdCollectionName = "admin_documents_cold" // 历史文档（7天前）

	vectorDim = 384 // 使用 all-MiniLM-L6-v2 模型的维度

	// 热数据判断阈值（7天）
	hotDataThresholdDays = 7
)

// SearchService handles vector search operations
type SearchService struct {
	client client.Client
}

// NewSearchService creates a new Milvus search service
func NewSearchService(milvusAddr string) (*SearchService, error) {
	c, err := client.NewGrpcClient(context.Background(), milvusAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %w", err)
	}

	svc := &SearchService{client: c}

	// Initialize collections
	if err := svc.initCollections(); err != nil {
		return nil, fmt.Errorf("failed to initialize collections: %w", err)
	}

	return svc, nil
}

// initCollections creates collections if they don't exist
func (s *SearchService) initCollections() error {
	ctx := context.Background()

	// Hot collections - 用 HNSW 索引（速度最快）
	if err := s.createHotCollection(ctx, usersHotCollectionName); err != nil {
		return err
	}
	if err := s.createHotCollection(ctx, docsHotCollectionName); err != nil {
		return err
	}

	// Cold collections - 用 IVF_SQ8 索引（省内存）
	if err := s.createColdCollection(ctx, docsColdCollectionName); err != nil {
		return err
	}

	return nil
}

// createHotCollection 创建热数据 Collection（HNSW 索引 - 最快）
func (s *SearchService) createHotCollection(ctx context.Context, collectionName string) error {
	has, err := s.client.HasCollection(ctx, collectionName)
	if err != nil {
		return err
	}

	if has {
		log.Printf("✓ Collection %s (HOT) already exists", collectionName)
		return nil
	}

	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    fmt.Sprintf("Hot data search index for %s (HNSW)", collectionName),
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				AutoID:     false,
				TypeParams: map[string]string{"max_length": "100"},
			},
			{
				Name:       "text",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "1000"},
			},
			{
				Name:     "embedding",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", vectorDim),
				},
			},
		},
	}

	if err := s.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return err
	}

	// HNSW 索引 - 最快的搜索性能，内存占用 +10%
	// 参数：M=16（最大连接数），ef_construction=200（构造时搜索精度）
	idx, err := entity.NewIndexHNSW(entity.L2, 16, 200)
	if err != nil {
		return err
	}
	if err := s.client.CreateIndex(ctx, collectionName, "embedding", idx, false); err != nil {
		return err
	}

	if err := s.client.LoadCollection(ctx, collectionName, false); err != nil {
		return err
	}

	log.Printf("✓ Collection %s (HOT - HNSW) created", collectionName)
	return nil
}

// createColdCollection 创建冷数据 Collection（IVF_SQ8 索引 - 省内存）
func (s *SearchService) createColdCollection(ctx context.Context, collectionName string) error {
	has, err := s.client.HasCollection(ctx, collectionName)
	if err != nil {
		return err
	}

	if has {
		log.Printf("✓ Collection %s (COLD) already exists", collectionName)
		return nil
	}

	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    fmt.Sprintf("Cold data search index for %s (IVF_SQ8)", collectionName),
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				AutoID:     false,
				TypeParams: map[string]string{"max_length": "100"},
			},
			{
				Name:       "text",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "1000"},
			},
			{
				Name:     "embedding",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", vectorDim),
				},
			},
		},
	}

	if err := s.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return err
	}

	// IVF_SQ8 索引 - 内存占用 1/4，牺牲少量精度（量化）
	// 参数：nlist=1024（大规模数据）
	idx, err := entity.NewIndexIvfSQ8(entity.L2, 1024)
	if err != nil {
		return err
	}
	if err := s.client.CreateIndex(ctx, collectionName, "embedding", idx, false); err != nil {
		return err
	}

	if err := s.client.LoadCollection(ctx, collectionName, false); err != nil {
		return err
	}

	log.Printf("✓ Collection %s (COLD - IVF_SQ8) created", collectionName)
	return nil
}

// isHotData 判断数据是否为热数据（7天内为热数据）
func (s *SearchService) isHotData(createdAt string) bool {
	if createdAt == "" {
		return true // 默认新数据为热数据
	}

	// 尝试解析 RFC3339 格式
	t, err := parseTime(createdAt)
	if err != nil {
		return true // 解析失败时默认为热数据
	}

	return time.Since(t) < time.Duration(hotDataThresholdDays)*24*time.Hour
}

// parseTime 解析多种时间格式
func parseTime(timeStr string) (time.Time, error) {
	// 尝试 RFC3339 格式
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t, nil
	}
	// 尝试 RFC3339Nano 格式
	if t, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
		return t, nil
	}
	// 尝试标准时间格式
	return time.Parse("2006-01-02 15:04:05", timeStr)
}

// calculateHNSWParam 计算 HNSW 搜索参数
// ef 参数控制搜索精度，值越大精度越高但速度越慢
// 建议范围：10-100（推荐20-50）
func (s *SearchService) calculateHNSWParam(keyword string) int {
	// 根据关键词长度调整 ef
	// 短关键词（1-3字）：ef=20（快速）
	// 中等关键词（4-8字）：ef=30
	// 长关键词（9+字）：ef=50（精确）
	switch len(keyword) {
	case 1, 2, 3:
		return 20
	case 4, 5, 6, 7, 8:
		return 30
	default:
		return 50
	}
}

// IndexUser adds or updates a user in the search index (always hot, no aging)
func (s *SearchService) IndexUser(ctx context.Context, user User) error {
	// Combine searchable fields
	text := fmt.Sprintf("%s %s %s", user.ID, user.Name, user.Email)

	// Generate embedding
	embedding := s.generateSimpleEmbedding(text)

	// Users always go to hot collection (no cold user data)
	idColumn := entity.NewColumnVarChar("id", []string{user.ID})
	textColumn := entity.NewColumnVarChar("text", []string{text})
	embeddingColumn := entity.NewColumnFloatVector("embedding", vectorDim, [][]float32{embedding})

	_, err := s.client.Insert(ctx, usersHotCollectionName, "",
		idColumn, textColumn, embeddingColumn)

	return err
}

// IndexDocument adds or updates a document in the search index (hot or cold based on age)
func (s *SearchService) IndexDocument(ctx context.Context, doc Document) error {
	text := fmt.Sprintf("%s %s %s", doc.ID, doc.Title, doc.Category)

	embedding := s.generateSimpleEmbedding(text)

	idColumn := entity.NewColumnVarChar("id", []string{doc.ID})
	textColumn := entity.NewColumnVarChar("text", []string{text})
	embeddingColumn := entity.NewColumnFloatVector("embedding", vectorDim, [][]float32{embedding})

	// Route to hot or cold collection based on creation date
	collectionName := docsHotCollectionName
	if !s.isHotData(doc.CreatedAt) {
		collectionName = docsColdCollectionName
	}

	_, err := s.client.Insert(ctx, collectionName, "",
		idColumn, textColumn, embeddingColumn)

	return err
}

// SearchUsers performs vector similarity search on users
// Users always stored in hot collection (HNSW index)
func (s *SearchService) SearchUsers(ctx context.Context, keyword string, topK int) ([]string, error) {
	if keyword == "" {
		return nil, nil
	}

	embedding := s.generateSimpleEmbedding(keyword)
	ef := s.calculateHNSWParam(keyword) // For HNSW index

	// HNSW 搜索参数
	sp, _ := entity.NewIndexHNSWSearchParam(ef)
	result, err := s.client.Search(
		ctx,
		usersHotCollectionName,
		[]string{},
		"",
		[]string{"id"},
		[]entity.Vector{entity.FloatVector(embedding)},
		"embedding",
		entity.L2,
		topK,
		sp,
	)

	if err != nil {
		return nil, err
	}

	var ids []string
	if len(result) > 0 {
		for i := 0; i < result[0].ResultCount; i++ {
			id, _ := result[0].IDs.Get(i)
			ids = append(ids, id.(string))
		}
	}

	return ids, nil
}

// SearchDocuments performs vector similarity search on documents
// Searches both hot and cold collections, merges results
func (s *SearchService) SearchDocuments(ctx context.Context, keyword string, topK int) ([]string, error) {
	if keyword == "" {
		return nil, nil
	}

	embedding := s.generateSimpleEmbedding(keyword)

	// 分别搜索热和冷集合
	hotIDs, err := s.searchDocumentsInCollection(ctx, usersHotCollectionName, embedding, topK, s.calculateHNSWParam(keyword), "HNSW")
	if err != nil {
		log.Printf("hot collection search failed: %v", err)
		hotIDs = []string{}
	}

	coldIDs, err := s.searchDocumentsInCollection(ctx, docsColdCollectionName, embedding, topK, s.calculateNProbe(keyword), "IVF_SQ8")
	if err != nil {
		log.Printf("cold collection search failed: %v", err)
		coldIDs = []string{}
	}

	// 合并结果（热数据优先）
	ids := append(hotIDs, coldIDs...)

	// 如果超过 topK，则截取
	if len(ids) > topK {
		ids = ids[:topK]
	}

	return ids, nil
}

// searchDocumentsInCollection 在指定集合中搜索文档
func (s *SearchService) searchDocumentsInCollection(
	ctx context.Context,
	collectionName string,
	embedding []float32,
	topK int,
	param interface{}, // HNSWSearchParam 或 nprobe (int)
	indexType string,
) ([]string, error) {
	var sp entity.SearchParam

	switch indexType {
	case "HNSW":
		hsp, _ := entity.NewIndexHNSWSearchParam(param.(int))
		sp = hsp
	case "IVF_SQ8":
		isp, _ := entity.NewIndexIvfSQ8SearchParam(param.(int))
		sp = isp
	default:
		return nil, fmt.Errorf("unknown index type: %s", indexType)
	}

	result, err := s.client.Search(
		ctx,
		collectionName,
		[]string{},
		"",
		[]string{"id"},
		[]entity.Vector{entity.FloatVector(embedding)},
		"embedding",
		entity.L2,
		topK,
		sp,
	)

	if err != nil {
		return nil, err
	}

	var ids []string
	if len(result) > 0 {
		for i := 0; i < result[0].ResultCount; i++ {
			id, _ := result[0].IDs.Get(i)
			ids = append(ids, id.(string))
		}
	}

	return ids, nil
}

// calculateNProbe 根据搜索关键词长度动态调整探测范围
// 短关键词 (1-3 char) -> 粗搜 (nprobe=4)
// 中关键词 (4-10 char) -> 中搜 (nprobe=8)
// 长关键词 (>10 char) -> 精搜 (nprobe=16)
func (s *SearchService) calculateNProbe(keyword string) int {
	keywordLen := len(strings.TrimSpace(keyword))
	switch {
	case keywordLen <= 3:
		return 4 // Very short - fast but broad search
	case keywordLen <= 10:
		return 8 // Medium - balanced
	default:
		return 16 // Long query - detailed search
	}
}

// generateSimpleEmbedding 生成简化的向量表示（演示用）
// 生产环境应替换为真实的 embedding API（OpenAI/HuggingFace 等）
func (s *SearchService) generateSimpleEmbedding(text string) []float32 {
	// 简化版：基于字符哈希生成伪向量
	embedding := make([]float32, vectorDim)
	for i, c := range text {
		idx := i % vectorDim
		embedding[idx] += float32(c) / 1000.0
	}
	// 归一化
	var sum float32
	for _, v := range embedding {
		sum += v * v
	}
	if sum > 0 {
		norm := float32(1.0) / float32(len(text))
		for i := range embedding {
			embedding[i] *= norm
		}
	}
	return embedding
}

// DeleteUser removes a user from the search index
func (s *SearchService) DeleteUser(ctx context.Context, userID string) error {
	expr := fmt.Sprintf("id == '%s'", userID)
	return s.client.Delete(ctx, usersHotCollectionName, "", expr)
}

// DeleteDocument removes a document from the search index
// Must delete from both hot and cold collections
func (s *SearchService) DeleteDocument(ctx context.Context, docID string) error {
	expr := fmt.Sprintf("id == '%s'", docID)

	// Try to delete from hot collection (ignore error if not found)
	_ = s.client.Delete(ctx, docsHotCollectionName, "", expr)

	// Try to delete from cold collection (ignore error if not found)
	_ = s.client.Delete(ctx, docsColdCollectionName, "", expr)

	return nil
}

// Close closes the Milvus client
func (s *SearchService) Close() error {
	return s.client.Close()
}
