# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
```bash
# Build the bot
go build -o discord-bot

# Run the bot
go run main.go

# Install dependencies
go mod download
go mod tidy
```

### Development
```bash
# Format code
go fmt ./...

# Lint (install golangci-lint first)
golangci-lint run

# Check for vulnerabilities
go mod verify
```

## Architecture Overview

This is a Discord bot written in Go that manages Chinese-language Discord servers. It uses a multi-database SQLite architecture to isolate data between guilds.

### Core Components

1. **Bot Initialization** (`main.go`): Loads environment variables, initializes databases, registers commands, and starts scheduled tasks.

2. **Command System** (`/handlers`): Event-driven architecture handling Discord interactions:
   - Preset messages with user mentions
   - Roll card game system
   - Forum post scanning and management
   - Leaderboard tracking

3. **Database Layer** (`/utils/database`): Multi-database SQLite system:
   - Guild databases: `./data/{guild_id}.db` - Per-server forum posts
   - User database: `./data/user.db` - Cross-server user data
   - Guild config: `./data/guilds.db` - Server configurations

4. **Scanner System** (`/scanner`): Concurrent forum scanning with worker pools:
   - Active scans every 3 hours (5:00, 13:00, 21:00)
   - Full scans every 7 days
   - Post deletion for removed/old content

### Environment Configuration

Required `.env` file:
```
BOT_TOKEN=your_discord_bot_token
LOG_CHANNEL_ID=channel_id_for_logs
DEVELOPER_USER_IDS=comma,separated,discord,ids
SUPER_ADMIN_ROLE_IDS=comma,separated,role,ids
DISABLE_INITIAL_SCAN=true/false
```

### Key Patterns

- **Interface-based design**: Avoids circular dependencies between packages
- **Concurrent processing**: Goroutines with worker pools for efficient scanning
- **Permission levels**: Guest < User < Admin < Developer
- **State persistence**: JSON files maintain state between restarts
- **Guild isolation**: Each Discord server has its own database

### Common Development Tasks

When modifying handlers:
1. Add handler function to appropriate subdirectory in `/handlers`
2. Register in `/handlers/command_handlers.go` or `/handlers/handlers.go`
3. Update command definitions in `/commands` if adding new slash commands

When working with databases:
1. Database operations go in `/utils/database/`
2. Follow existing patterns for prepared statements
3. Always use guild-specific databases for server data

When adding scheduled tasks:
1. Modify ticker setup in `/bot/run.go`
2. Add task logic to appropriate handler or scanner module