# my_agent 接手文档

## 1. 当前目标

`my_agent` 现在已经从“单一 HTTP beacon”演进到“Beacon + Session”双模结构，设计目标对齐 Sliver 的思路：

- `beacon`：低频轮询，负责普通任务、文件操作、会话拉起
- `session`：显式通过 `session_start` 拉起，负责 `pty`、`socks`、`rpfwd`
- `session` 不原地升级旧 callback，而是派生一个新的 websocket callback

当前代码已经可以编译通过，但是否真正端到端跑通，取决于你本地 Mythic / RabbitMQ / `http` / `websocket` profile 是否在线。

## 2. 关键目录

### Agent 运行时

- [main.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/main.go)
  - 启动入口
  - 读取嵌入配置
  - 创建 beacon controller
- [controller.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/controller.go)
  - 通用控制器
  - 封装 checkin / 轮询 / 提交响应
- [session_supervisor.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/session_supervisor.go)
  - 进程级 session 管理
  - `session_start / stop / status`
- [transports.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/transports.go)
  - HTTP transport
  - WebSocket transport
- [task_execution.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/task_execution.go)
  - 普通命令执行
  - 角色校验
  - download/upload/remove/report 等
- [interactive_sessions.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/interactive_sessions.go)
  - PTY / interactive session
- [socks_manager.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/socks_manager.go)
  - SOCKS registry
- [rpfwd_manager.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/rpfwd_manager.go)
  - RPFWD registry
- [embedded_config.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/embedded_config.go)
  - builder 注入的统一配置格式

### Payload Type 容器侧

- [builder.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agentfunctions/builder.go)
  - 构建入口
  - 支持 `http + websocket`
  - 生成嵌入配置并注入 linker flags
- [session_start.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agentfunctions/session_start.go)
  - 创建 `session_link_id`
  - 写 Agent Storage
- [session_stop.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agentfunctions/session_stop.go)
- [session_status.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agentfunctions/session_status.go)
- [on_new_callback.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agentfunctions/on_new_callback.go)
  - 新 callback 出现后，关联回父 beacon/task
  - 更新 description
  - 向父 task 回写 “session callback ready”

## 3. 当前数据流

### 3.1 Beacon 启动

1. Agent 启动后读取嵌入配置
2. `main.go` 创建 beacon controller
3. beacon controller 用 `http` profile 做初始 `checkin`
4. 后续走 `get_tasking/post_response`

### 3.2 派生 Session

1. Operator 在 beacon callback 上执行 `session_start`
2. Payload Type 容器生成 `session_link_id`
3. 容器把 `session_link_id -> parent_callback_id / parent_task_id` 写入 Agent Storage
4. Agent 收到 `session_start` 后，在本地创建 websocket transport
5. Session controller 用相同 payload UUID 重新 `checkin`
6. 新 callback 的 `process_name` 带 `[session:<id>]`
7. `OnNewCallback` 检测到标记后：
   - 查 Agent Storage
   - 更新子 callback description
   - 向父 task 回写 ready 消息
   - 删除 Agent Storage 记录

### 3.3 角色边界

- beacon 允许：
  - `echo`
  - `shell`
  - `ls`
  - `download`
  - `upload`
  - `remove`
  - `report`
  - `session_start`
  - `session_status`
- session 允许：
  - `pty`
  - `socks`
  - `socks_stop`
  - `rpfwd`
  - `rpfwd_stop`
  - `session_stop`
  - `session_status`

如果在错误 callback 上执行，会由 Agent 侧直接返回错误。

## 4. 本地调试方式

### 4.1 启动 Payload Type 服务

在 [my_agent_service](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service)：

```bash
make run_local_service
```

要求：

- 本地 RabbitMQ 在线
- Mythic 主服务在线

### 4.2 本地跑 Agent

```bash
make run_agent_local
```

默认环境变量在 [Makefile](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/Makefile)：

- `MY_AGENT_CALLBACK_HOST`
- `MY_AGENT_CALLBACK_PORT`
- `MY_AGENT_AESPSK`
- `MY_AGENT_WS_CALLBACK_HOST`
- `MY_AGENT_WS_CALLBACK_PORT`
- `MY_AGENT_WS_AESPSK`
- `MY_AGENT_ENABLE_SESSION_MODE`
- `MY_AGENT_INSECURE_SKIP_VERIFY`
- `MY_AGENT_INTERACTIVE_SHELL`

这些变量只用于 `run_agent_local` 的源码调试兜底。
正式在 Mythic 里构建 payload 时，功能是否包含主要取决于
`Select Commands to Include in the Payload` 中勾选了哪些命令。

## 5. 构建要求

如果在命令勾选阶段包含了 session 相关命令，例如：

- `session_start`
- `session_stop`
- `session_status`
- `pty`
- `socks`
- `socks_stop`
- `rpfwd`
- `rpfwd_stop`

那么 builder 会自动把当前 payload 识别成启用了 session mode。

此时：

- 构建时必须同时选择 `http` 和 `websocket` C2 profiles
- 缺少任意一个，builder 会直接失败

当前 builder 会把所有运行参数打成一个 Base64 JSON，注入：

- `main.BuildEmbeddedConfigB64`

旧的散落变量只保留了最小的本地调试兜底项，正式 payload 不再依赖
`MY_AGENT_ENABLE_*` 这类能力开关。

## 6. 已知限制

### 6.1 WebSocket transport 还是简化版

[transports.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agent_code/transports.go) 里的 `websocketTransport.Send` 目前采用的是“发一条、收一条”模型。

这意味着：

- 它适合当前先验证 session callback 是否能建立
- 但还没有完全进化成“独立 reader/writer pump 的真正流式 Push C2”

如果你后续要把体验继续拉向 Sliver：

1. 拆出独立 websocket reader goroutine
2. 把 inbound 顶层消息改成长期消费
3. 把 outbound 走单独发送队列
4. 让 session 真正脱离轮询式 `Send -> Receive`

### 6.2 端到端联调依赖环境

当前代码已经 `go build ./...` 通过，但如果你本地：

- `127.0.0.1:5672` 不通
- `127.0.0.1:9999` 不通

那 `run_local_service` / `run_agent_local` 仍然会失败，这不是代码编译问题，而是本地服务没起来。

## 7. 建议接手顺序

### 第一阶段：先做联调

1. 启动 Mythic / RabbitMQ
2. 确认 `http` profile 在线
3. 安装并启动 `websocket` profile
4. 重新 build payload
5. 验证：
   - beacon checkin
   - `session_start`
   - 新 session callback 出现
   - `pty` 在 session callback 上成功

### 第二阶段：补强 Push Session

1. 把 websocket transport 改成真正的流式 pump
2. 将 session controller 改成事件驱动
3. 再做 `socks` / `rpfwd` 大流量稳定性测试

### 第三阶段：做 UI/可观测性优化

1. 给 session callback 增加更明显的 description/tag
2. 给 `session_status` 加更多运行态信息
3. 给 `socks/rpfwd` 增加连接计数与错误回显

## 8. 本轮我建议你优先验证的命令

在新的 beacon callback 上：

```text
session_status
session_start
```

等新的 session callback 出现后，再在 session callback 上测试：

```text
pty /bin/sh
session_status
```

如果这条链路通了，再继续测：

```text
socks 1080
rpfwd {"local_port":8080,"remote_ip":"127.0.0.1","remote_port":80}
```

## 9. 快速判断故障点

### Payload 构建失败

优先看：

- [builder.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agentfunctions/builder.go)
- Mythic build step 输出

### session_start 成功但没有新 callback

优先看：

- websocket profile 是否在线
- Agent 端 websocket 是否能连上
- `OnNewCallback` 是否被触发

### 新 callback 出来了但父 task 没收到 ready

优先看：

- [on_new_callback.go](/Users/zhujiayi/Documents/allMyCode/01_Work/ExampleContainers/Payload_Type/my_agent_service/my_agent/agentfunctions/on_new_callback.go)
- Agent Storage 里 `session_link:<uuid>` 是否存在

### pty/socks/rpfwd 一直卡

优先看：

- 当前是否真的在 session callback 上执行
- websocket transport 是否只做了同步收发，导致实时性不够
- `interactive/socks/rpfwd` 顶层消息是否成功回传
