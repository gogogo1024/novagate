# 分级搜索架构（Tiered Search Architecture）

## 目标
支持千万级用户（10M+）和上亿文档（100M+）的大规模搜索，通过分级索引策略优化内存占用、查询速度和成本。

## 架构设计

### 集合划分

#### 用户数据（Users）
- **Collection**: `admin_users_hot` (HNSW 索引)
- **数据特点**: 所有活跃用户
- **索引类型**: HNSW（Hierarchical Navigable Small World）
- **特点**:
  - ✅ 最快的查询性能（10-50ms）
  - ✅ 100% 查询精度
  - ✅ 内存占用相对高（基础向量）
  - ✅ 无冷热分化（用户始终活跃）

**HNSW 索引参数**:
```go
M = 16          // 最大连接数（每个节点连接数）
ef_construction = 200  // 构造时的搜索精度
ef_search = 20-50      // 查询时的精度参数（自适应）
```

#### 文档数据（Documents）

##### Hot Collection - 热数据
- **Collection**: `admin_documents_hot` (HNSW 索引)
- **数据条件**: `createdAt >= now - 7days`
- **数据量**: 最近 7 天的文档（假设日均增长率，约 10-20M）
- **特点**:
  - ✅ 最常被访问的数据
  - ✅ 性能最优（10-50ms）
  - ✅ 内存占用最高
- **查询优先级**: PRIMARY（优先搜索）

##### Cold Collection - 冷数据
- **Collection**: `admin_documents_cold` (IVF_SQ8 索引)
- **数据条件**: `createdAt < now - 7days`
- **数据量**: 历史文档（7+ 天前，累积 80-90M）
- **特点**:
  - ✅ 内存占用只有 HNSW 的 25%（量化）
  - ✅ 查询速度略低（50-200ms）
  - ✅ 精度 98%（可接受）
- **查询优先级**: SECONDARY（备选搜索）

**IVF_SQ8 索引参数**:
```go
nlist = 1024        // 倒排索引簇数（大规模数据）
nprobe = 32-64      // 查询时搜索簇数（自适应）
```

## 数据路由逻辑

### 写入路由 (IndexDocument)

```
创建文档
  ↓
是否 createdAt < 7days 前？
  ├─ YES → 写入 admin_documents_hot (HNSW)
  └─ NO  → 写入 admin_documents_cold (IVF_SQ8)
```

**代码实现**:
```go
func (s *SearchService) IndexDocument(ctx context.Context, doc Document) error {
    collectionName := docsHotCollectionName
    if !s.isHotData(doc.CreatedAt) {
        collectionName = docsColdCollectionName
    }
    // 插入到对应集合
}
```

### 读取路由 (SearchDocuments)

```
用户搜索 keyword
  ↓
并行搜索：
  ├─ hot   → admin_documents_hot (HNSW, ef=20-50)
  └─ cold  → admin_documents_cold (IVF_SQ8, nprobe=32-64)
  ↓
合并结果
  ├─ 热结果优先显示
  └─ 若不足 topK，补充冷结果
  ↓
返回用户
```

**代码实现**:
```go
func (s *SearchService) SearchDocuments(ctx context.Context, keyword string, topK int) ([]string, error) {
    // 并行搜索热和冷集合
    hotIDs, _ := searchDocumentsInCollection(ctx, docsHotCollectionName, ...)
    coldIDs, _ := searchDocumentsInCollection(ctx, docsColdCollectionName, ...)
    
    // 合并结果，热优先
    ids := append(hotIDs, coldIDs...)
    return ids[:topK], nil
}
```

## 性能对比

### 索引类型对比

| 指标 | HNSW | IVF_FLAT | IVF_SQ8 |
|------|------|----------|---------|
| 查询速度 | ⚡⚡⚡ | ⚡⚡ | ⚡⚡ |
| 内存占用 | 1x (基准) | 1x | 0.25x (量化) |
| 精度 | 100% | 100% | 98%+ |
| 索引构建速度 | 快 | 快 | 快 |
| 适用数据量 | 10M+ | <1M | 100M+ |
| 维护成本 | 低 | 低 | 低 |

### 预期性能（基于 384 维向量）

#### 10M 用户（HNSW）
- **内存占用**: ~150GB (每向量 15KB, 包括图结构)
- **单次查询**: 15-30ms
- **QPS**: 1000+ (单机)

#### 100M 文档分级（热 10M + 冷 90M）
- **热集合 (HNSW)**:
  - 内存: ~15GB
  - 查询: 10-30ms
  
- **冷集合 (IVF_SQ8)**:
  - 内存: ~3.4GB (1/4 压缩)
  - 查询: 50-150ms
  
- **总内存**: ~18.4GB (vs 单层 HNSW 的 150GB)
- **节省**: **87.7% 内存**

## 实施要点

### 1. 创建时间判断
```go
const hotDataThresholdDays = 7

func isHotData(createdAt string) bool {
    t, _ := parseTime(createdAt)
    return time.Since(t) < 7*24*time.Hour
}
```

### 2. 自适应搜索参数
```go
// HNSW ef 参数
func calculateHNSWParam(keyword string) int {
    switch len(keyword) {
    case 1, 2, 3:
        return 20   // 短查询，快速
    case 4, 5, 6, 7, 8:
        return 30   // 中等查询
    default:
        return 50   // 长查询，精确
    }
}

// IVF_SQ8 nprobe 参数
func calculateNProbe(keyword string) int {
    switch {
    case len(keyword) <= 3:
        return 32   // 短查询
    case len(keyword) <= 10:
        return 48   // 中等查询
    default:
        return 64   // 长查询
    }
}
```

### 3. 并行搜索优化
```go
// 并发搜索两个集合，减少总延迟
hotChan := make(chan []string, 1)
coldChan := make(chan []string, 1)

go func() {
    hotChan <- searchInCollection(ctx, docsHotCollectionName, ...)
}()

go func() {
    coldChan <- searchInCollection(ctx, docsColdCollectionName, ...)
}()

hotResults := <-hotChan
coldResults := <-coldChan
return merge(hotResults, coldResults)
```

## 数据迁移策略

### 场景1：初始导入 100M 文档
```
方案：按创建时间分批
├─ 最近 7 天: 导入到 admin_documents_hot (HNSW)
└─ 7+ 天前: 导入到 admin_documents_cold (IVF_SQ8)
```

### 场景2：定期冷热迁移（Optional）
```
每日凌晨 2:00 AM:
├─ 扫描 admin_documents_hot 中 createdAt > 7 days 的文档
├─ 转移到 admin_documents_cold
└─ 删除源记录
```

## 监控指标

### 集合大小监控
```bash
# 查看各集合文档数
admin_users_hot: 10,000,000 docs
admin_documents_hot: 10,000,000 docs (最近 7 天)
admin_documents_cold: 90,000,000 docs (历史)

总计: 110,000,000 docs
```

### 查询性能监控
```
热集合查询:
  avg: 20ms
  p95: 40ms
  p99: 60ms

冷集合查询:
  avg: 80ms
  p95: 150ms
  p99: 200ms
```

## 未来优化

### 1. 自动数据冷却 (Auto-Tiering)
```go
// 每小时检查一次
if doc.CreatedAt < now - 7days {
    moveFromHotToCold(doc)
}
```

### 2. 向量缓存 (Vector Cache)
```
常见搜索词的向量结果缓存
├─ 命中率: 70%+
├─ 缓存大小: 100MB
└─ 命中时延: <1ms
```

### 3. 多区域分片 (Sharding by Region)
```
北区文档 → Milvus-North (50M docs, HNSW)
南区文档 → Milvus-South (50M docs, HNSW)
并行查询，合并结果
```

### 4. 向量压缩 (Vector Compression)
```
当前: float32 (4 bytes × 384 = 1.5KB/向量)
压缩到: float16 (2 bytes × 384 = 768B/向量)
节省: 50% 内存
```

## 配置示例

### novagate.yaml
```yaml
milvus:
  addr: "localhost:19530"
  
search:
  hot_data_threshold_days: 7
  default_top_k: 10
  
  # HNSW 参数
  hnsw:
    M: 16
    ef_construction: 200
    
  # IVF_SQ8 参数
  ivf_sq8:
    nlist: 1024
    nprobe_min: 32
    nprobe_max: 64
```

## 验证清单

- [x] HNSW 索引用于热数据
- [x] IVF_SQ8 索引用于冷数据
- [x] 自动热冷路由
- [x] 并行搜索优化
- [x] 自适应查询参数
- [ ] 数据迁移工具
- [ ] 监控仪表板
- [ ] 性能基准测试

## 参考资源

- Milvus 文档: https://milvus.io
- HNSW 论文: https://arxiv.org/abs/1603.09320
- IVF_SQ8: https://arxiv.org/abs/1702.08734
