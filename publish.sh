#!/bin/bash
# Publish petri-pilot changes to GitHub and deploy to pflow.dev
#
# Usage: ./publish.sh [OPTIONS] [commit message]
#
# Options:
#   --deploy     Deploy only (pull latest, rebuild, restart)
#   --status     Show service status and recent logs
#   --help       Show this help message
#
# Examples:
#   ./publish.sh "Add new MCP tool"      # Commit, push, and deploy
#   ./publish.sh --deploy                # Just deploy (no commit)
#   ./publish.sh --status                # Check service status

set -e

REMOTE_HOST="pflow.dev"
REMOTE_USER="ubuntu"
TMUX_SESSION="servers"
TMUX_WINDOW="0"  # pflow-pilot window
PROJECT_DIR="~/Workspace/petri-pilot"
SERVICE_PORT="8083"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

show_help() {
    cat << 'EOF'
publish.sh - Publish petri-pilot to GitHub and deploy to production

USAGE
    ./publish.sh [OPTIONS] [commit message]

OPTIONS
    --deploy     Deploy only: pull latest code, rebuild, and restart
    --status     Show service status and recent logs
    --help       Show this help message

EXAMPLES
    ./publish.sh "Add new MCP tool"
        Commit changes, push to GitHub, and deploy to server

    ./publish.sh --deploy
        Pull latest from GitHub, rebuild, and restart service (no commit)

    ./publish.sh --status
        Check if service is running and show recent logs

ENVIRONMENT
    The remote server needs these environment variables (set in ~/start_servers.sh):
      - GITHUB_CLIENT_ID
      - GITHUB_CLIENT_SECRET

SERVER
    Host:    pflow.dev (user: ubuntu)
    Tmux:    servers:0 (pflow-pilot)
    Port:    8083
EOF
}

# Check service status
status() {
    echo -e "${BLUE}==> Checking petri-pilot status on $REMOTE_HOST...${NC}"
    echo ""

    # Check if process is running
    if ssh "$REMOTE_USER@$REMOTE_HOST" "pgrep -f 'petri-pilot.*$SERVICE_PORT'" > /dev/null 2>&1; then
        echo -e "${GREEN}Service is running${NC}"
        echo ""
        echo "Process:"
        ssh "$REMOTE_USER@$REMOTE_HOST" "ps aux | grep 'petri-pilot.*$SERVICE_PORT' | grep -v grep"
    else
        echo -e "${RED}Service is NOT running${NC}"
    fi

    echo ""
    echo -e "${YELLOW}Recent tmux output:${NC}"
    ssh "$REMOTE_USER@$REMOTE_HOST" "tmux capture-pane -t $TMUX_SESSION:$TMUX_WINDOW -p | tail -20"
}

# Deploy: pull latest, rebuild, and restart
deploy() {
    echo -e "${YELLOW}==> Deploying to $REMOTE_HOST...${NC}"

    # Pull latest
    ssh "$REMOTE_USER@$REMOTE_HOST" "cd $PROJECT_DIR && git pull origin main"

    # Rebuild
    echo -e "${YELLOW}==> Rebuilding...${NC}"
    ssh "$REMOTE_USER@$REMOTE_HOST" "cd $PROJECT_DIR && go build -o petri-pilot ./cmd/petri-pilot"

    # Stop current process
    echo -e "${YELLOW}==> Restarting service...${NC}"
    ssh "$REMOTE_USER@$REMOTE_HOST" "tmux send-keys -t $TMUX_SESSION:$TMUX_WINDOW C-c"
    sleep 2

    # Start with environment variables
    # The start command runs the MCP server with demo apps
    ssh "$REMOTE_USER@$REMOTE_HOST" "tmux send-keys -t $TMUX_SESSION:$TMUX_WINDOW './petri-pilot serve -port $SERVICE_PORT tic-tac-toe zk-tic-tac-toe coffeeshop erc20-token blog-post task-manager support-ticket' Enter"

    # Wait for startup
    sleep 2

    # Verify it started
    echo ""
    if ssh "$REMOTE_USER@$REMOTE_HOST" "pgrep -f 'petri-pilot.*$SERVICE_PORT'" > /dev/null 2>&1; then
        echo -e "${GREEN}==> Deployed and running!${NC}"
    else
        echo -e "${RED}==> Warning: Service may not have started correctly${NC}"
        echo "Check logs with: ./publish.sh --status"
    fi

    echo ""
    echo -e "${YELLOW}Recent output:${NC}"
    ssh "$REMOTE_USER@$REMOTE_HOST" "tmux capture-pane -t $TMUX_SESSION:$TMUX_WINDOW -p | tail -10"
}

# Parse arguments
DO_DEPLOY=false
DO_STATUS=false
MESSAGE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --deploy)
            DO_DEPLOY=true
            shift
            ;;
        --status)
            DO_STATUS=true
            shift
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        -*)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information."
            exit 1
            ;;
        *)
            MESSAGE="$1"
            shift
            ;;
    esac
done

# Handle status mode
if $DO_STATUS; then
    status
    exit 0
fi

# Handle deploy-only mode
if $DO_DEPLOY && [ -z "$MESSAGE" ]; then
    deploy
    exit 0
fi

# Full publish mode requires commit message
if [ -z "$MESSAGE" ]; then
    echo "Usage: ./publish.sh [OPTIONS] \"commit message\""
    echo ""
    echo "Options:"
    echo "  --deploy     Deploy only (pull latest, rebuild, restart)"
    echo "  --status     Show service status and recent logs"
    echo "  --help       Show full help"
    echo ""
    echo "Current changes:"
    git status --short
    exit 1
fi

# Show what will be committed
echo -e "${YELLOW}==> Staging changes...${NC}"
git status --short

# Stage changes (exclude generated/ and other artifacts)
git add -A

# Check if there's anything to commit
if git diff --cached --quiet; then
    echo "No changes to commit."
    exit 0
fi

# Commit with co-author
echo ""
echo -e "${YELLOW}==> Committing...${NC}"
git commit -m "$MESSAGE

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"

# Push to origin
echo ""
echo -e "${YELLOW}==> Pushing to origin...${NC}"
git push origin main

# Deploy to remote
deploy

echo ""
echo -e "${GREEN}==> Published and deployed!${NC}"
