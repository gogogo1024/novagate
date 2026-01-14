#!/usr/bin/env bash
# Validate GitHub Actions workflows locally

set -e

echo "🔍 Validating GitHub Actions workflows..."
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check if actionlint is installed
if ! command -v actionlint &> /dev/null; then
    echo -e "${YELLOW}⚠️  actionlint not installed${NC}"
    echo "Install with: brew install actionlint (macOS) or go install github.com/rhysd/actionlint/cmd/actionlint@latest"
    echo ""
    echo "Skipping workflow validation..."
    exit 0
fi

# Validate each workflow file
workflow_dir=".github/workflows"
failed=0

for workflow in "$workflow_dir"/*.yml; do
    echo -e "${YELLOW}Checking: $(basename "$workflow")${NC}"
    
    if actionlint "$workflow"; then
        echo -e "${GREEN}✓ Valid${NC}"
    else
        echo -e "${RED}✗ Invalid${NC}"
        failed=$((failed + 1))
    fi
    echo ""
done

if [ $failed -eq 0 ]; then
    echo -e "${GREEN}✅ All workflows are valid!${NC}"
    exit 0
else
    echo -e "${RED}❌ $failed workflow(s) have errors${NC}"
    exit 1
fi
