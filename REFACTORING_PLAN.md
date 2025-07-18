# 项目重构行动计划 (Project Refactoring Action Plan)

## 1. 概述 (Overview)

当前项目的重构工作已因基础架构存在严重缺陷而陷入停滞。本计划旨在提供一个清晰、分阶段的路线图，以纠正现有问题、稳固技术基础，并最终达成一个真正健壮、安全、可维护的系统。

**核心原则：** 停止一切新功能的开发和上层业务的重构，集中所有资源优先完成以下基础架构的修复工作。

---

## 2. 第一阶段：地基加固 (Phase 1: Foundation Stabilization) - **最高优先级**

此阶段的目标是修复核心仓储层和管理器，使其达到可用、安全、完整的状态。

### **任务 1.1: 彻底修复 `RepositoryManager`**

-   **问题**: `RepositoryManager` 当前是一个充满 `nil` 和设计冲突的空壳。
-   **行动项**:
    1.  **[ ] 解决事务类型冲突**: 统一事务模型。在 `NewTransactionalUserRepository` 及所有其他事务性仓储的构造函数中，必须正确处理 `*sqlx.Tx` 类型，而不是 `*sql.Tx`。确保 `manager.go` 中被注释掉的 `NewTransactionalUserRepository` 调用被修复并启用。
    2.  **[ ] 实现所有缺失的构造函数**: 移除所有返回 `nil` 的占位符实现。必须为 `GuildRepository`, `TimedTaskRepository`, `LeaderboardRepository` 等所有接口提供具体的、可工作的非事务性实现。
    3.  **[ ] 实现所有事务性构造函数**: 必须为 `PostRepository`, `PunishmentRepository` 等所有需要支持事务的仓储提供可工作的 `NewTransactional...` 版本实现。

### **任务 1.2: 修复并完善 `PostRepository`**

-   **问题**: 存在严重SQL注入漏洞，且功能不完整。
-   **行动项**:
    1.  **[ ] 消除SQL注入漏洞**:
        *   **严禁**使用 `fmt.Sprintf` 将表名拼接到SQL语句中。
        *   **必须**在执行任何数据库操作前，通过 `GetTableNames` 获取当前服务器所有合法的表名，将传入的 `tableName` 与此白名单进行比对。如果不在白名单内，应立即返回错误，而不是执行查询。
    2.  **[ ] 实现缺失的接口方法**:
        *   必须实现 `PostRepository` 接口中定义的 `CreateTable(guildID, tableName string) error` 方法。
        *   必须实现 `PostRepository` 接口中定义的 `DropTable(guildID, tableName string) error` 方法。

### **任务 1.3: 完成 `UserRepository` 的实现**

-   **问题**: `UserRepository` 几乎完全未实现。
-   **行动项**:
    1.  **[ ] 实现所有接口方法**: 必须为 `UserRepository` 接口中定义的所有方法（`GetByID`, `Create`, `Update`, `Delete`, `GetUserStats` 等）提供完整、正确的实现。不能再返回 `"not implemented"` 错误。

---

## 3. 第二阶段：业务层重构 (Phase 2: Business Logic Refactoring)

在地基稳固后，可以开始将业务逻辑迁移到新的仓储层。

### **任务 2.1: 重构 `rollcard` 模块**

-   **问题**: 仍在使用旧的、不安全的数据库调用。
-   **行动项**:
    1.  **[ ] 移除旧的数据库依赖**: 删除 `handlers/rollcard/handler.go` 中对 `utils/database` 的所有直接或间接调用。
    2.  **[ ] 全面采用 `UserRepository`**: 使用在第一阶段完成的 `UserRepository` 来处理所有与用户相关的操作，例如获取用户偏好设置。

### **任务 2.2: 优化 `PunishmentRepository`**

-   **问题**: 实现质量不高，存在性能和安全隐患。
-   **行动项**:
    1.  **[ ] 提升性能**: 修改 `punishmentRepository` 结构，在初始化时创建并持有一个 `*sqlx.DB` 实例，避免在每个方法中重复创建。
    2.  **[ ] 移除硬编码**: 将 `"sqlite3"` 驱动名作为配置项传入，而不是在代码中硬编码。
    3.  **[ ] 优化辅助函数**: 重构 `getPunishmentsByField` 和 `countByField`，使用白名单验证 `field` 参数，避免潜在的SQL注入风险。

---

## 4. 第三阶段：审查与验收 (Phase 3: Review and Acceptance)

-   **行动项**:
    1.  **[ ] 代码审查**: 对所有修改过的代码进行严格的同行评审，确保符合安全和质量标准。
    2.  **[ ] 功能回归测试**: 验证所有相关指令（`/punish`, `/punish_admin`, `/roll`）在重构后功能正常。
    3.  **[ ] 更新状态报告**: 提交一份**诚实的、准确的**重构状态报告，反映所有已完成的工作和已解决的问题。