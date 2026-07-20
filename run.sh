#!/bin/bash
set -euo pipefail

# ──────────────────────────────────────────────
# lighttrust-api + litellm + Langfuse 全链路启动脚本
# 依次拉起：Redis → PostgreSQL → ClickHouse → MinIO
#          → 数据基础设施（迁移/建桶）
#          → litellm proxy → lighttrust-api → Langfuse web & worker
# ──────────────────────────────────────────────

# ── 颜色 ──
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()   { echo -e "${RED}[ERROR]${NC} $*"; }

# ── 常量 ──
LIGHTTRUST_DIR="/Users/nick/owner_projects/lighttrust-api"
LITELLM_DIR="/Users/nick/owner_projects/litellm"
LANGFUSE_DIR="/Users/nick/owner_projects/langfuse"
LITELLM_VENV="/Users/nick/owner_projects/litellm/.venv"
LITELLM_LOG="/tmp/litellm.log"
LT_LOG="/tmp/lighttrust-api.log"
LANGFUSE_LOG_DIR="/tmp/langfuse"
CLICKHOUSE_DATA="/tmp/ch-data"
MINIO_DATA="/tmp/minio-data"
MINIO_PORT=9090

# ── 清理 ──
cleanup() {
  info "收到退出信号，开始清理..."
  kill %1 %2 %3 %4 2>/dev/null || true
  wait 2>/dev/null || true
  info "所有进程已终止"
}
trap cleanup SIGINT SIGTERM EXIT

# ── 0. 环境检查 ──
info "=== 环境检查 ==="
for bin in go pnpm redis-server clickhouse minio migrate; do
  command -v "$bin" >/dev/null 2>&1 || { err "$bin 未安装"; exit 1; }
done

# 加载 litellm 的 .env（含 DEEPSEEK_API_KEY）
if [ -f "$LITELLM_DIR/.env" ]; then
  set -a; source "$LITELLM_DIR/.env"; set +a
  info "已加载 $LITELLM_DIR/.env"
else
  warn "$LITELLM_DIR/.env 不存在，DeepSeek API key 可能缺失"
fi

# ── 1. PostgreSQL ──
info "=== 启动 PostgreSQL@17 ==="
brew services start postgresql@17 2>/dev/null || true
for i in $(seq 1 15); do
  pg_isready -q && break
  sleep 1
done
info "PostgreSQL 就绪"

# ── 2. Redis ──
info "=== 启动 Redis ==="
brew services start redis 2>/dev/null || true
for i in $(seq 1 10); do
  redis-cli ping 2>/dev/null && break
  sleep 1
done
redis-cli CONFIG SET requirepass "myredissecret" 2>/dev/null || true
info "Redis 就绪（密码已设置）"

# ── 3. ClickHouse ──
info "=== 启动 ClickHouse ==="
mkdir -p "$CLICKHOUSE_DATA"
clickhouse server --path="$CLICKHOUSE_DATA" \
  --http_port=8123 --tcp_port=9000 \
  > /tmp/clickhouse.log 2>&1 &
CLICKHOUSE_PID=$!
for i in $(seq 1 30); do
  clickhouse client --user clickhouse --password clickhouse \
    --query "SELECT 1" 2>/dev/null && break
  sleep 1
done
info "ClickHouse 就绪 (PID $CLICKHOUSE_PID)"

info "=== 执行 ClickHouse 迁移 ==="
if [ -f "$LANGFUSE_DIR/packages/shared/clickhouse/scripts/up.sh" ]; then
  cd "$LANGFUSE_DIR/packages/shared"
  bash clickhouse/scripts/up.sh 2>&1 | tail -5
  cd "$LANGFUSE_DIR"
  info "ClickHouse 迁移完成"
else
  warn "ClickHouse 迁移脚本未找到，跳过"
fi

# ── 4. MinIO ──
info "=== 启动 MinIO ==="
mkdir -p "$MINIO_DATA"
export MINIO_ROOT_USER="minio"
export MINIO_ROOT_PASSWORD="miniosecret"
minio server "$MINIO_DATA" --address ":$MINIO_PORT" --console-address ":9091" \
  > /tmp/minio.log 2>&1 &
MINIO_PID=$!
for i in $(seq 1 15); do
  curl -s "http://localhost:$MINIO_PORT/minio/health/live" >/dev/null && break
  sleep 1
done
info "MinIO 就绪 (PID $MINIO_PID)"

info "=== 创建 MinIO bucket ==="
command -v mc >/dev/null 2>&1 || brew install minio-mc >/dev/null 2>&1
mc alias set local http://localhost:$MINIO_PORT minio miniosecret 2>/dev/null || true
mc mb local/local 2>/dev/null || info "bucket local 已存在"
info "MinIO bucket 就绪"

# ── 5. Langfuse 基础设施 ──
info "=== Langfuse Prisma 生成 ==="
cd "$LANGFUSE_DIR"

# .env 若缺失则从 .env.example 复制
if [ ! -f .env ]; then
  if [ -f .env.dev.example ]; then
    cp .env.dev.example .env
    warn ".env 从 .env.dev.example 复制而来，请检查配置"
  fi
fi

# 安装依赖（如未安装）
if [ ! -d node_modules ]; then
  pnpm install --frozen-lockfile 2>&1 | tail -3
fi

# 编译 shared 包
info "=== 编译 @langfuse/shared ==="
pnpm --filter @langfuse/shared run build 2>&1 | tail -3

info "=== Prisma 数据库迁移 ==="
pnpm run db:generate 2>&1 | tail -3
pnpm run db:migrate 2>&1 | tail -5 || warn "db:migrate 可能已是最新"

# ── 6. lighttrust-api（编译 + 启动） ──
info "=== 编译 lighttrust-api ==="
cd "$LIGHTTRUST_DIR"
go build -o /tmp/lt-final 2>&1 | tail -5
info "编译完成"

info "=== 启动 lighttrust-api ==="
export SQLITE_PATH="$LIGHTTRUST_DIR/one-api.db"
export REDIS_CONN_STRING="redis://:myredissecret@localhost:6379"
export BATCH_UPDATE_ENABLED=true
export SESSION_COOKIE_SECURE=false
export TZ=Asia/Shanghai
/tmp/lt-final > "$LT_LOG" 2>&1 &
LT_PID=$!
for i in $(seq 1 20); do
  curl -s -o /dev/null http://localhost:3000/api/status 2>/dev/null && break
  sleep 1
done
info "lighttrust-api 就绪 (PID $LT_PID, 端口 3000)"

# ── 7. litellm proxy ──
info "=== 启动 litellm proxy ==="
cd "$LITELLM_DIR"
source "$LITELLM_VENV/bin/activate"
export LITELLM_MASTER_KEY="${LITELLM_MASTER_KEY:-sk-litellm-master-key-1234}"
export DEEPSEEK_API_KEY="${DEEPSEEK_API_KEY:-}"
# 从 .env 重新读取（确保覆盖）
if [ -f .env ]; then
  set -a; source .env; set +a
fi
litellm --config proxy_server_config.yaml --port 4000 \
  > "$LITELLM_LOG" 2>&1 &
LITELLM_PID=$!
for i in $(seq 1 20); do
  curl -s -o /dev/null http://localhost:4000/health 2>/dev/null && break
  sleep 1
done
info "litellm 就绪 (PID $LITELLM_PID, 端口 4000)"

# ── 8. Langfuse worker ──
mkdir -p "$LANGFUSE_LOG_DIR"
info "=== 启动 Langfuse worker ==="
cd "$LANGFUSE_DIR"
pnpm run dev:worker > "$LANGFUSE_LOG_DIR/worker.log" 2>&1 &
WORKER_PID=$!
info "Langfuse worker 启动中 (PID $WORKER_PID)"

# ── 9. Langfuse web ──
info "=== 启动 Langfuse web ==="
cd "$LANGFUSE_DIR"
pnpm run dev:web > "$LANGFUSE_LOG_DIR/web.log" 2>&1 &
WEB_PID=$!
info "Langfuse web 启动中 (PID $WEB_PID)"

# ── 等待 web 就绪 ──
for i in $(seq 1 60); do
  curl -s -o /dev/null http://localhost:3001 2>/dev/null && break
  sleep 2
done
info "Langfuse web 就绪 (端口 3001)"

# ── 完成 ──
echo ""
info "=========================================="
info "  全部服务已启动！"
info "=========================================="
info "  lighttrust-api   → http://localhost:3000"
info "  litellm proxy    → http://localhost:4000"
info "  Langfuse web     → http://localhost:3001"
info "  Langfuse login   → admin@local.dev / Admin@123456!"
info "  MinIO console    → http://localhost:9091"
info "=========================================="
echo ""

# 保持前台运行
wait
