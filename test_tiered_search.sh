#!/bin/bash
# 测试分级搜索架构

BASE_URL="http://localhost:8888"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}=== 分级搜索架构测试 ===${NC}\n"

# 1. 创建用户
echo -e "${YELLOW}1️⃣ 创建测试用户...${NC}"
curl -s -X POST "$BASE_URL/api/users" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Machine Learning Engineer",
    "email": "ml.engineer@example.com",
    "department": "AI"
  }' | jq '.data.id' -r > /tmp/user_id.txt

USER_ID=$(cat /tmp/user_id.txt)
echo "✓ 创建用户: $USER_ID"

# 2. 创建文档
echo -e "\n${YELLOW}2️⃣ 创建热数据文档（应进入 HOT 集合）...${NC}"
curl -s -X POST "$BASE_URL/api/documents" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Machine Learning Fundamentals",
    "category": "Tutorial",
    "content": "Deep learning, neural networks, transformers"
  }' | jq '.data.id' -r > /tmp/doc_id.txt

DOC_ID=$(cat /tmp/doc_id.txt)
echo "✓ 创建文档: $DOC_ID"

# 3. 等待索引完成
echo -e "\n${YELLOW}3️⃣ 等待向量索引...${NC}"
sleep 2

# 4. 搜索测试
echo -e "\n${YELLOW}4️⃣ 搜索测试 - 关键词: 'machine'${NC}"
SEARCH_RESULT=$(curl -s "$BASE_URL/api/users?keyword=machine" | jq '.data | length')
echo "✓ 搜索用户结果数: $SEARCH_RESULT"

if [ "$SEARCH_RESULT" -gt 0 ]; then
  echo -e "${GREEN}✅ 用户搜索成功（从 hot 集合）${NC}"
else
  echo -e "${RED}❌ 用户搜索失败${NC}"
fi

echo -e "\n${YELLOW}5️⃣ 搜索测试 - 关键词: 'learning'${NC}"
DOC_SEARCH=$(curl -s "$BASE_URL/api/documents?keyword=learning" | jq '.data | length')
echo "✓ 搜索文档结果数: $DOC_SEARCH"

if [ "$DOC_SEARCH" -gt 0 ]; then
  echo -e "${GREEN}✅ 文档搜索成功（从 hot 集合）${NC}"
else
  echo -e "${RED}❌ 文档搜索失败${NC}"
fi

# 6. 查询 Milvus 集合统计
echo -e "\n${YELLOW}6️⃣ Milvus 集合统计信息${NC}"
docker exec novagate-milvus \
  /milvus/bin/milvus-cli query -u default -p Milvus \
  -i 'show collections' 2>/dev/null || true

echo -e "\n${GREEN}=== 测试完成 ===${NC}"
echo "建议后续测试："
echo "1. 修改文档的 createdAt 为 7天前，验证自动进入冷集合"
echo "2. 大规模数据模拟（10M+ 用户、100M+ 文档）"
echo "3. 性能对比：HNSW vs IVF_SQ8 vs IVF_FLAT"
