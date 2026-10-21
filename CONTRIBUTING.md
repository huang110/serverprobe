# 贡献指南

感谢您考虑为 ServerProbe 做出贡献！

## 如何贡献

### 报告 Bug

1. 在 [Issues](https://github.com/huang110/serverprobe/issues) 中搜索是否已有相同问题。
2. 如果没有，创建新 Issue，包含：
   - 问题描述
   - 复现步骤
   - 预期行为与实际行为
   - 操作系统、Go/Node 版本
   - 日志或截图

### 提交功能建议

1. 在 Issues 中创建 Feature Request。
2. 描述使用场景和预期价值。
3. 等待维护者反馈。

### 提交代码

```bash
git clone https://github.com/huang110/serverprobe.git
cd serverprobe
git checkout -b feature/your-feature-name
```

编写代码时请遵循现有风格，添加必要测试，并更新相关文档。

#### 运行测试

```bash
# Agent
cd agent && go test -count=1 ./...

# Server
cd server && go test -count=1 ./...

# 端到端集成测试
cd tests && go test -v -count=1 -timeout 10m ./...

# 前端类型检查与构建
cd frontend && npm ci && npm run type-check && npm run build
```

#### 代码规范

**Go**：

```bash
gofmt -w .
go vet ./...
```

**TypeScript / Vue**：

```bash
npm run type-check
```

#### 提交消息

使用语义化提交前缀：

- `feat:` 新功能
- `fix:` Bug 修复
- `docs:` 文档
- `style:` 格式
- `refactor:` 重构
- `test:` 测试
- `chore:` 构建/工具

示例：

```bash
git commit -m "feat: add offline agent detection"
```

#### 提交 Pull Request

请说明：

- 改动内容
- 解决的问题
- 测试结果
- 是否包含破坏性变更

提交前请确保：

- [ ] `gofmt` / `go vet` 通过
- [ ] 单元测试通过
- [ ] 集成测试通过
- [ ] 前端构建通过
- [ ] 文档已更新

## 已知可贡献方向

- 自定义脚本指标采集
- Agent 离线检测
- Token 轮换幂等缓存
- 邮件 / Telegram 告警通道
- 更多图表与聚合维度

## 问题与讨论

- Issues：https://github.com/huang110/serverprobe/issues
- Discussions：https://github.com/huang110/serverprobe/discussions

感谢您的贡献！
