# 新卡推送排行榜 - 多服务器支持实现计划

## 需求概述
允许用户在创建新卡推送排行榜时指定查看特定服务器或全局的新卡排行榜。

## 技术架构设计

### 1. 命令结构修改
修改 `/new-cards` 命令，添加一个可选的 `scope` 参数：
- `current`: 当前服务器（默认）
- `global`: 全局所有服务器
- `server:<guild_id>`: 指定特定服务器

### 2. 数据聚合策略

#### 2.1 单服务器模式
- 保持现有逻辑不变
- 从指定服务器的数据库读取数据

#### 2.2 全局模式
- 遍历 `databaseMapping.json` 中的所有服务器
- 聚合所有数据库的统计信息
- 合并最新卡片列表

#### 2.3 跨服务器模式
- 验证用户是否有权限查看目标服务器
- 从目标服务器数据库读取数据

### 3. 权限控制

#### 3.1 权限规则
- **当前服务器**：需要超级管理员或开发者权限
- **其他服务器**：需要开发者权限或在目标服务器有超级管理员权限
- **全局**：仅限开发者权限

#### 3.2 权限检查流程
```go
1. 检查基础权限（超级管理员/开发者）
2. 如果是跨服务器查询：
   - 检查是否为开发者
   - 或检查用户在目标服务器的权限
3. 如果是全局查询：
   - 仅允许开发者
```

### 4. 数据库操作优化

#### 4.1 并发查询
- 使用 goroutines 并发查询多个数据库
- 使用 sync.WaitGroup 等待所有查询完成
- 使用 channel 收集结果

#### 4.2 错误处理
- 单个数据库查询失败不影响整体结果
- 记录失败的服务器信息
- 在结果中显示成功查询的服务器数量

### 5. UI/UX 设计

#### 5.1 命令参数
```
/new-cards [scope: 查看范围]
- scope选项：
  - 当前服务器（默认）
  - 全局所有服务器
  - 选择特定服务器...
```

#### 5.2 排行榜显示
- 标题显示查询范围
- 如果是全局/跨服务器，显示数据来源
- 显示各服务器的贡献比例（可选）

### 6. 实现步骤

1. **修改命令定义** (commands/defs/leaderboard.go)
   - 添加 scope 参数
   - 定义参数选项

2. **创建数据聚合函数** (utils/database/global_stats.go)
   - GetGlobalPostStats()
   - GetMultiServerPosts()
   - 并发查询实现

3. **修改处理器** (handlers/leaderboard/handler.go)
   - 解析 scope 参数
   - 根据 scope 调用相应的数据获取函数
   - 权限验证逻辑

4. **更新构建函数** (handlers/leaderboard/handler.go)
   - buildLeaderboardEmbeds() 支持多数据源
   - 添加数据源标识

5. **权限验证增强** (utils/auth.go)
   - 添加跨服务器权限检查函数
   - CheckCrossGuildPermission()

### 7. 测试场景

1. **基础功能测试**
   - 当前服务器排行榜（默认行为）
   - 指定其他服务器排行榜
   - 全局排行榜

2. **权限测试**
   - 普通用户无法使用
   - 管理员只能查看当前服务器
   - 开发者可以查看所有

3. **错误处理测试**
   - 无效的服务器ID
   - 数据库连接失败
   - 权限不足

4. **性能测试**
   - 多服务器并发查询
   - 大数据量处理

### 8. 数据结构示例

```go
// 扩展的排行榜状态
type ExtendedLeaderboardState struct {
    LeaderboardState
    Scope       string   // "current", "global", "server:xxx"
    SourceGuilds []string // 数据来源服务器列表
}

// 全局统计结果
type GlobalStats struct {
    TotalPosts      int
    GuildStats      map[string]int // 每个服务器的贡献
    LatestPosts     []model.Post
    TimeRangeStats  map[string]int // 今日、昨日等统计
}
```

### 9. 配置文件更新

可能需要在配置中添加：
- 全局查询的最大服务器数量限制
- 查询超时设置
- 缓存策略配置

### 10. 未来扩展

- 添加缓存机制提高全局查询性能
- 支持自定义服务器组合查询
- 导出排行榜数据功能
- 定时生成全局报告