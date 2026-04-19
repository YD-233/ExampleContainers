# 基于 Mythic 框架的渗透测试远控系统设计与实现（初稿）

## 摘要

随着网络攻防对抗研究的不断发展，命令与控制（Command and Control，C2）系统已成为渗透测试、攻防演练和安全验证中的关键基础设施。传统远控系统往往存在平台耦合度高、扩展困难、功能模块分散等问题。Mythic 作为一套模块化、容器化的现代 C2 框架，具备良好的可扩展性和任务编排能力，适合进行二次开发与工程化研究。

本文基于 Mythic 框架，设计并实现了一套面向实验环境的渗透测试远控系统原型。系统以 Go 语言开发跨平台 Agent，以自定义 Payload Type 与自定义 C2 Profile 为基础，构建了完整的 `Beacon + Session` 双模通信体系。其中，Beacon 通道负责低频轮询、命令执行、文件管理和系统状态收集，Session 通道基于 Push C2 架构实现交互式会话、SOCKS5 代理和反向端口转发等高频双向通信能力。为增强通信伪装效果，本文还设计并实现了 `masked_https` 自定义 C2 Profile，使 Agent 流量能够通过 HTTPS/WSS 通道进行传输，并支持路径、Host、User-Agent 以及附加请求头的伪装配置。

在系统功能层面，本文完成了命令执行、文件上传下载、目录管理、系统信息收集、进程枚举、安全产品识别、辅助提权探测、交互式 PTY、SOCKS5 代理、反向端口转发等多项能力，并实现了与 Mythic 平台任务系统、消息队列和回调管理机制的整体对接。全文覆盖了 Agent 开发、Payload Type 扩展、自定义 C2 Profile 设计、Push Session 会话通道、文件管理模型适配、代理转发能力接入、跨平台构建与实机测试等多个模块，形成了一套功能覆盖较为完整、组成层次较为丰富的远控系统原型。测试结果表明，系统已能够稳定完成 Payload 构建、Beacon 上线、Session 派生、交互式会话、SOCKS/反向端口转发等关键功能验证，满足毕业设计阶段对系统完整性、可演示性和可验证性的要求。

关键词：Mythic；渗透测试；远控系统；Beacon；Push C2；SOCKS5；反向端口转发

---

## 第1章 绪论

### 1.1 研究背景

在网络安全攻防场景中，远控系统承担着任务下发、结果回传、资产管理和横向支撑等关键功能。随着 C2 平台逐步向模块化、容器化和多语言扩展方向演进，现代渗透测试系统已经不再是单一的“木马程序”，而是由控制端、通信组件、Agent 及配套插件共同构成的复杂系统。

Mythic 是近年来较为典型的现代 C2 框架之一。其整体架构围绕 PostgreSQL、RabbitMQ、主服务、Payload Type、C2 Profile、Webhook、Translation Container 等组件构建，具有明显的微服务特征。相比传统“单体式”远控框架，Mythic 更强调功能解耦、接口标准化和动态扩展能力。因此，围绕 Mythic 开展 Agent 与自定义 C2 Profile 开发，具有较强的工程实践价值。

### 1.2 研究意义

本课题的意义主要体现在以下几个方面：

1. 通过对 Mythic 扩展机制的实际开发，深入理解现代 C2 系统的任务生命周期、通信流程与模块组织方式。
2. 通过实现自定义 Agent、Payload Type 与 C2 Profile，掌握基于 Go 的跨平台 Agent 开发方法及其与 Mythic 的集成方式。
3. 通过实现 `Beacon + Session` 双模通信结构，验证低频任务轮询与高频实时会话在同一平台下的协同模式。
4. 通过实现文件管理、SOCKS5、PTY、反向端口转发等功能，构建一套具备完整演示价值的原型系统，为后续攻防研究提供实验基础。
5. 通过完成 Payload Type、C2 Profile、Mythic 微服务配置与容器部署的联动开发，积累跨模块系统集成经验，体现现代安全工具开发中“协议实现 + 平台接入 + 工程运维”并重的实践价值。

### 1.3 研究内容

本文围绕 Mythic 扩展开发，重点完成以下内容：

1. 设计并实现 Go 语言编写的 `my_agent` Agent。
2. 设计并实现自定义 `masked_https` C2 Profile。
3. 完成 Agent 与 Mythic 的任务通信、文件管理、系统信息收集和结果回传能力。
4. 构建 `Beacon + Push Session` 双模架构，并实现基于 Push Session 的 PTY、SOCKS5 与反向端口转发。
5. 对系统进行完整联调与测试，验证其功能完整性和通信可用性。
6. 完成 Payload Type、C2 Profile、Push Session、文件管理、代理转发、跨平台测试等多个子模块的集成与验证，形成较完整的系统原型。

### 1.4 论文结构

本文后续内容安排如下：

- 第2章：系统需求分析；
- 第3章：相关技术介绍；
- 第4章：系统总体设计；
- 第5章：系统实现；
- 第6章：系统测试与结果分析；
- 第7章：总结与展望。

---

## 第2章 需求分析

### 2.1 功能需求分析

#### 2.1.1 Beacon 通信需求

Agent 需要支持周期性心跳通信，即以一定轮询间隔向服务端发起请求，完成初始上线、任务拉取和结果回传。该模式适合命令执行、系统信息查询、文件传输等离散型任务。

#### 2.1.2 Session 通信需求

除低频 Beacon 外，系统还需要支持高频双向流量能力，以满足交互式会话、SOCKS5 代理和反向端口转发等功能。此类能力要求通信链路具备低延迟和持续性，因此必须构建独立于 Beacon 的 Session 通道。

#### 2.1.3 命令执行需求

系统需要提供基础命令执行能力，支持操作系统命令调用、标准输出回传与错误信息处理，并与 Mythic 的 tasking 流程保持一致。

#### 2.1.4 文件管理需求

系统需要支持：

1. 目录浏览；
2. 文件上传；
3. 文件下载；
4. 文件删除；
5. 与 Mythic 文件浏览器模型兼容的结构化回传格式。

#### 2.1.5 信息收集需求

系统应支持基础资产与环境信息收集，包括：

1. 系统基本信息；
2. 进程枚举；
3. 安全产品识别；
4. 权限状态与辅助提权环境探测。

#### 2.1.6 代理转发需求

系统应支持：

1. SOCKS5 代理；
2. 反向端口转发（RPFWD）。

这两类功能均要求数据能够在 Agent、C2 与 Mythic 之间稳定双向转发。

### 2.2 非功能需求分析

#### 2.2.1 可扩展性

系统应支持通过 Payload Type 命令定义扩展新的任务功能，并支持通过 C2 Profile 增加新的通信方式。

#### 2.2.2 跨平台性

Agent 需支持在不同操作系统上构建与运行，至少能够覆盖 macOS、Linux 和 Windows。

#### 2.2.3 稳定性

系统应具备基本的错误处理、超时控制与资源清理能力，避免因高频任务或长连接导致资源泄漏。

#### 2.2.4 可验证性

系统设计应便于进行端到端联调和测试，能够通过日志、数据库、前端界面等多种方式验证各功能点执行状态。

---

## 第3章 相关技术

### 3.1 Mythic 框架

Mythic 是一套模块化的命令与控制平台，其核心特征在于：

1. 使用 PostgreSQL 存储任务、文件、回调和操作信息；
2. 使用 RabbitMQ 进行服务间消息通信；
3. 以 Payload Type 和 C2 Profile 作为扩展入口；
4. 支持通过 RPC 方式实现组件间协作。

在本文中，Mythic 主要承担以下角色：

1. 构建 Payload；
2. 保存任务和回调状态；
3. 通过 GraphQL/UI 提供操作界面；
4. 通过 Push C2 机制支持 Session 直连投递。

### 3.2 Payload Type

Payload Type 用于描述某一类 Agent 的构建方式、支持命令和任务处理逻辑。本文实现的 `my_agent_service` 即属于自定义 Payload Type 服务。它主要负责：

1. 注册 Agent 基本信息；
2. 定义命令与参数；
3. 构建二进制样本；
4. 处理 `process_response`、`OnNewCallback` 等服务端钩子。

### 3.3 C2 Profile

C2 Profile 用于定义 Agent 与 Mythic 之间的通信方式。本文实现的 `masked_https` 同时承载 Beacon 与 Session 两种通信：

1. Beacon：HTTPS `POST`；
2. Session：WSS + Push C2 gRPC stream。

与官方 `http` / `websocket` Profile 相比，`masked_https` 统一了两条链路的伪装参数，使二者在 Host、User-Agent、路径和 Header 上保持一致。

### 3.4 Push C2

Push C2 是 Mythic 中用于支持长连接和实时通信的机制。其核心思路是：

1. Agent 与 C2 Profile 保持长连接；
2. C2 Profile 与 Mythic 保持 gRPC 流；
3. Mythic 可直接将任务推送到在线 Session Callback；
4. Agent 可持续回传 SOCKS、RPFWD、interactive 数据。

Push C2 是本文实现真 Session 的关键技术基础。

### 3.5 Go 语言

Go 语言具有跨平台编译、标准库丰富、并发模型清晰等优点，适合用于 Agent 开发。本文中，Go 被用于：

1. Agent 本体实现；
2. 自定义 C2 Profile 服务端实现；
3. Payload Type 构建逻辑。

---

## 第4章 系统设计

### 4.1 总体架构设计

本文系统整体由三部分组成：

1. Mythic 控制端；
2. 自定义 `masked_https` C2 Profile；
3. Go 语言编写的 `my_agent`。

其总体结构可概括为：

```text
Operator -> Mythic UI/GraphQL -> Mythic Server
                                -> RabbitMQ / PostgreSQL
                                -> Payload Type: my_agent_service
                                -> C2 Profile: masked_https
masked_https <-> my_agent (Beacon / Session)
```

### 4.2 Beacon + Session 双模设计

#### 4.2.1 Beacon

Beacon 通道负责：

1. 初始上线（checkin）；
2. `get_tasking` 拉取任务；
3. `post_response` 提交结果；
4. 命令执行、文件管理、系统信息收集等离散任务。

#### 4.2.2 Session

Session 通道负责：

1. 派生独立 session callback；
2. 通过 WSS 与 C2 Profile 保持长连接；
3. 通过 Push C2 gRPC stream 与 Mythic 保持实时双向流；
4. 承载 `pty`、`socks`、`rpfwd` 等高频通信功能。

该设计使系统同时具备低频任务执行能力和高频实时交互能力，能够在同一套平台内覆盖离散任务与持续流量两类典型使用场景，从而提升系统的功能完整性与展示效果。

### 4.3 Agent 模块设计

Agent 主要由以下模块组成：

1. 配置加载模块；
2. Beacon 控制器；
3. Session 控制器；
4. 命令分发模块；
5. 文件管理模块；
6. 信息收集模块；
7. PTY 模块；
8. SOCKS 模块；
9. RPFWD 模块。

### 4.4 自定义 `masked_https` 设计

`masked_https` Profile 的设计目标是统一 Beacon 与 Session 的外部通信特征。

其主要设计点包括：

1. HTTPS Beacon 路径与 WSS Session 路径可分离；
2. 支持 `Host`、`User-Agent`、`extra_headers`、`query_string` 配置；
3. 对非法路径返回 decoy 响应；
4. Session 路径内部通过 Push C2 gRPC stream 与 Mythic 对接。

### 4.5 命令与能力边界

为保证系统结构清晰，本文将命令按 callback 角色进行划分：

#### 4.5.1 Beacon Callback

允许执行：

- `echo`
- `shell`
- `ls`
- `upload`
- `download`
- `remove`
- `report`
- `sysinfo`
- `ps`
- `avscan`
- `privesc_check`
- `privesc_tool`
- `session_start`
- `session_status`
- `exit`

#### 4.5.2 Session Callback

允许执行：

- `pty`
- `socks`
- `socks_stop`
- `rpfwd`
- `rpfwd_stop`
- `session_stop`
- `session_status`
- `exit`

---

## 第5章 系统实现

### 5.1 Payload Type 实现

本文实现的 `my_agent_service` 作为自定义 Payload Type，主要包括以下内容：

1. Builder：负责根据命令选择和 C2 参数构建样本；
2. 命令定义：定义参数、帮助信息与 tasking 行为；
3. 服务端辅助逻辑：如 `OnNewCallback`、结构化结果回写等。

Builder 已实现：

1. 根据 `Select Commands to Include in the Payload` 决定命令集；
2. 根据不同 C2 Profile 参数生成嵌入配置；
3. 支持 `masked_https` 单 Profile 模式；
4. 支持 `interactive_shell`、`insecure_skip_verify` 等必要构建参数。

通过该实现，系统能够根据 `Select Commands to Include in the Payload` 灵活生成不同能力组合的样本，便于在实验中针对命令执行、文件管理、信息收集、交互会话和代理转发等不同场景进行裁剪与验证，也进一步体现了系统在功能组织和样本生成方面的完整性。

### 5.2 Agent 通信实现

#### 5.2.1 Beacon 通信

Beacon 控制器负责：

1. 使用 Payload UUID 发起初始 `checkin`；
2. 获取 callback UUID；
3. 以 callback UUID 为身份进行后续通信；
4. 处理任务、响应、文件、结构化结果等普通任务流。

#### 5.2.2 Session 通信

Session 由 `session_start` 触发创建，随后：

1. Agent 建立 WSS 长连接；
2. `masked_https` 为该连接建立独立的 Push C2 gRPC stream；
3. Mythic 将 session callback 直接注册为在线 Push client；
4. Session 控制器通过 reader/writer pump 持续收发 `interactive/socks/rpfwd` 数据。

在 Session 通道实现后，系统能够将高频交互任务与普通 Beacon 任务分层处理，使交互式会话、代理转发和持续数据收发具备独立承载通道。这样既丰富了系统能力边界，也使整套原型在功能上更接近完整的现代远控系统形态。

### 5.3 命令执行与文件管理实现

系统已实现以下基础能力：

1. `shell`：执行系统命令；
2. `ls`：目录浏览；
3. `upload`：文件上传；
4. `download`：文件下载；
5. `remove`：文件删除；
6. `report`：结构化结果上报。

其中，`upload` 已修复为优先恢复原始文件名，不再默认落为 `file_id`。

文件管理模块构成了系统中最基础且最常用的一组能力。通过对 `upload`、`download`、`ls` 和 `remove` 的实现，系统已经覆盖了远控场景中常见的文件浏览、文件投递、结果回收和目标清理等核心需求，并与 Mythic 文件浏览器模型保持一致。

### 5.4 信息收集与辅助提权实现

#### 5.4.1 `sysinfo`

用于收集：

1. 主机名；
2. 用户名；
3. 操作系统；
4. 架构；
5. PID；
6. 路径等基础信息。

#### 5.4.2 `ps`

用于进行基础进程枚举。该命令在 Linux 环境下已验证可用，在 macOS 环境下受系统执行权限限制。

#### 5.4.3 `avscan`

基于进程名特征库进行安全产品识别。本文对特征集进行了整理，并实现为 Windows 优先功能。

#### 5.4.4 `privesc_check` 与 `privesc_tool`

本文未实现真实 exploit，而是实现“辅助提权模块”：

1. `privesc_check`：探测权限状态、sudo、SUID 等环境信息；
2. `privesc_tool`：允许调用外部工具或脚本，并回传结果。

### 5.5 PTY、SOCKS 与 RPFWD 实现

#### 5.5.1 PTY

PTY 通过 Session callback 执行，启动后可进入交互式 shell，会话结束时能正确回收资源。实现中需要处理不同平台的终端行为差异、Session 生命周期、控制命令与资源回收问题，避免退出后留下残留子进程或会话句柄。

#### 5.5.2 SOCKS5

SOCKS 模块实现了 Mythic 任务下发、Session 下推和实际 TCP 转发。通过该模块，系统已具备将 Agent 主机作为代理节点接入外部 HTTP/HTTPS 访问的能力，进一步扩展了原型系统在网络访问与流量中转方面的功能边界。

#### 5.5.3 RPFWD

RPFWD 模块用于在 Agent 主机监听某个端口，并将进入该端口的连接通过 Mythic 转发到指定远端地址。通过该模块，系统进一步覆盖了内网访问和端口转发场景，使整套原型在代理与转发能力方面形成了较完整的功能组合。本文在测试中进一步确认：

1. `remote_ip` 需要从 Mythic 容器的网络视角可达；
2. 正确配置后，RPFWD 已可实现真实端口转发。

---

## 第6章 系统测试与结果分析

### 6.1 测试环境

#### 6.1.1 控制端

- Mythic UI：`https://127.0.0.1:7443`
- 自定义 C2：`masked_https`
- Beacon 地址：`https://127.0.0.1:8443/cdn-cgi/submit`
- Session 地址：`wss://127.0.0.1:8443/cdn-cgi/ws`

#### 6.1.2 测试平台

- macOS 本机；
- Linux 远端服务器；
- Mythic Docker 容器环境。

### 6.2 核心测试结果

#### 6.2.1 Beacon 主链

已验证：

1. Payload 构建；
2. 下载与运行；
3. Beacon 上线；
4. `sysinfo`；
5. `shell`；
6. 文件上传、下载、删除；
7. `privesc_check`、`privesc_tool`。

#### 6.2.2 真 Push Session

已验证：

1. `session_start` 派生 session callback；
2. Mythic 将 session callback 注册为在线 Push client；
3. `session_status` 成功；
4. `pty` 成功；
5. `session_stop` 成功。

#### 6.2.3 SOCKS

已验证：

1. `socks` 启动；
2. `socks_stop` 停止；
3. 外部 HTTP/HTTPS 请求通过代理成功返回 `200`。

#### 6.2.4 RPFWD

已验证：

1. `rpfwd` 启动；
2. `rpfwd_stop` 停止；
3. 经真实本地 HTTP 服务验证，反向端口转发成功返回 `200`。

### 6.3 测试结果分析

从测试结果看，当前系统已具备以下特点：

1. 构建链路完整，Payload Type 与 C2 Profile 已能够协同工作；
2. Beacon 能稳定承载离散任务；
3. 真 Push Session 已成功支撑高频交互与代理转发；
4. `pty`、`socks`、`rpfwd` 三类高交互功能均已打通；
5. 系统已具备较完整的原型系统形态。

同时，从系统实现范围看，本文完成内容并不局限于若干单点命令，而是覆盖了以下多个层面：

1. 自定义 Payload Type 的设计、注册、构建与动态命令装载；
2. 自定义 `masked_https` C2 Profile 的 HTTPS/WSS 双通道设计；
3. `Beacon + Push Session` 双模通信体系的构建；
4. `pty`、`socks`、`rpfwd` 等高交互功能的接入与验证；
5. Mythic CLI、Docker Compose、InstalledServices、容器构建与服务纳管；
6. macOS 与 Linux 两类平台的实机验证。

因此，本文完成的并非单一 Agent 功能堆叠，而是一套跨越 Agent、Payload Type、C2 Profile、Mythic 平台和容器运行环境的系统级实现，整体功能覆盖面较广，模块组成较完整，测试验证也较为充分。

同时，也存在一些可继续优化的点：

1. macOS 下 `ps` 仍受系统环境限制；
2. 可进一步丰富 Linux/Windows 环境下的信息收集细节；
3. 可继续增强 `masked_https` 的伪装策略，如更复杂的 decoy 页面与更细粒度 Header 伪装。

---

## 第7章 总结与展望

### 7.1 总结

本文基于 Mythic 框架，设计并实现了一套面向实验环境的渗透测试远控系统原型。系统采用自定义 Payload Type 与自定义 `masked_https` C2 Profile，构建了 `Beacon + Push Session` 双模通信结构，并实现了命令执行、文件管理、系统状态收集、辅助提权探测、PTY、SOCKS5 和反向端口转发等核心功能。全文完成内容覆盖 Agent 开发、Payload Type 扩展、C2 Profile 设计、Push Session 通信、文件管理、信息收集、代理转发、跨平台构建和实机测试等多个部分，整体模块较为齐全。

测试结果表明，系统已完成从 Payload 构建、样本运行、Beacon 上线、Session 派生，到交互式会话、SOCKS 代理与反向端口转发的真实闭环验证，能够满足毕业设计阶段对系统完整性、可演示性和可验证性的要求。整体来看，本文形成了一套功能覆盖面较广、模块组成较完整、联调测试较充分的远控系统原型，能够较好体现毕业设计阶段的软件实现工作量与系统集成成果。

### 7.2 展望

后续可以从以下方向继续完善：

1. 补充更丰富的 Windows/Linux 信息收集能力；
2. 增加更细粒度的流量伪装与 decoy 页面设计；
3. 优化日志、告警与状态可观测性；
4. 对 `masked_https` Profile 增加更多协议适配能力；
5. 进一步完善图形化操作与自动化测试流程。

---

## 参考内容提示

后续正式写论文时，可继续补充以下内容：

1. Mythic 微服务架构图；
2. Beacon 与 Session 数据流图；
3. `masked_https` Push C2 结构图；
4. 构建页面、回调页面、任务执行结果截图；
5. 关键测试命令与返回结果截图；
6. 代码结构说明与模块关系图。
