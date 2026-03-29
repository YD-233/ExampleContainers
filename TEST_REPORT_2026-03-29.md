# Mythic `my_agent` 全链路测试报告

## 1. 测试范围

本次测试针对当前仓库中的以下组件进行联调验证：

- Payload Type：`my_agent_service`
- 自定义 C2 Profile：`masked_https`
- Agent：`my_agent`
- 传输链路：`HTTPS Beacon + WSS Session`

测试目标包括：

- 在 Mythic 中启动 C2 Profile
- 构建 Payload
- 下载 Payload
- 在本地 macOS 与远端 Linux 上运行样本
- 验证命令执行、文件管理、信息收集、辅助提权、Session、PTY、SOCKS 等能力

## 2. 测试环境

### 2.1 控制端

- Mythic UI：`https://127.0.0.1:7443`
- 自定义 C2：`masked_https`
- Beacon 地址：`https://127.0.0.1:8443/cdn-cgi/submit`
- Session 地址：`wss://127.0.0.1:8443/cdn-cgi/ws`

### 2.2 测试主机

- macOS：
  - 样本文件：`/tmp/my_agent_macos_full_e2e.bin`
  - 本轮新 callback：`C-58`
- Linux：
  - 样本文件：`/root/my_agent_linux_full_e2e.bin`
  - callback：`C-55`

## 3. 浏览器侧流程

本次已完成的浏览器真实操作：

- 登录 Mythic UI
- 在 Services 页面启动 `masked_https`
- 在 Payloads 页面下载构建后的 macOS / Linux 样本

说明：

- 浏览器侧“登录 / 启动 profile / 下载样本”已真实点击验证。
- 后续命令测试主要通过 Mythic 后端同一套任务接口执行，以便稳定批量验证功能点。

## 4. macOS 全链路测试结果

### 4.1 上线

- Payload：`e221703f-60ee-48fe-9749-0e9924c61737`
- 新 callback：`C-58`
- 上线成功

### 4.2 功能测试结果

| 功能 | 任务/说明 | 结果 |
|---|---|---|
| `sysinfo` | `T-150` | 成功 |
| `shell whoami` | `T-151` | 成功，返回 `zhujiayi` |
| `ps` | `T-156` | 失败，`/bin/ps` 在当前 macOS 环境被系统拒绝执行 |
| `privesc_check` | `T-155` | 成功，返回“未找到匹配的提权辅助探测结果” |
| `privesc_tool` | `T-157` | 成功，`/bin/echo hello-mac-privesc-v2` |
| `report` | `T-161` | 成功，结构化登记 artifact/credential |
| `session_start` | `T-160` | 成功，派生 session callback `C-59` |
| `session_status` | `T-170` | 成功 |
| `pty /bin/sh` | `T-171` | 成功，进入交互会话 |
| `socks 7000` | `T-173` | 任务成功 |
| SOCKS 实流量 | `curl --socks5-hostname 127.0.0.1:7000 -I https://example.com` | 失败，代理连接被关闭 |
| `socks_stop 7000` | `T-175` | 成功 |
| `session_stop` | `T-178` | 成功 |
| `exit` | `T-177` | 成功，Agent 优雅退出 |

### 4.3 文件管理测试

本轮文件上传下载链路已验证成功，但发现一个实现细节：

- `upload` 最终写入路径不是“目录 + 原始文件名”
- 当前实际落地为“目录 + `file_id`”

本次验证结果：

| 功能 | 任务/说明 | 结果 |
|---|---|---|
| `upload` | `T-163` / `T-164` | 成功 |
| 上传结果验证 | `T-166`，读取 `/tmp/2ef79703-5d8f-49f2-872a-ca3b998e9413` | 成功，内容为 `mac upload v2 content` |
| `download` | `T-167`，下载 `/tmp/2ef79703-5d8f-49f2-872a-ca3b998e9413` | 成功 |
| `remove` | `T-168`，删除 `/tmp/2ef79703-5d8f-49f2-872a-ca3b998e9413` | 成功 |

结论：

- 文件上传、下载、删除主链路可用
- 但上传目标文件名当前不是原始文件名，这属于实现上的路径处理问题，后续应修正

## 5. Linux 测试结果

### 5.1 已成功完成的项目

基于 callback `C-55`，以下项目已经真实执行成功：

| 功能 | 任务 | 结果 |
|---|---|---|
| `sysinfo` | `T-132` | 成功 |
| `ps` | `T-133` | 成功 |
| `shell whoami` | `T-134` | 成功，返回 `root` |
| `privesc_check` | `T-135` | 成功 |
| `privesc_tool` | `T-136` | 成功，`/bin/echo hello-linux-privesc` |
| `session_start` | `T-142` | 成功，派生 session `C-57` |
| `session_status` | `T-143` | 成功 |
| `pty /bin/sh` | `T-144` | 成功 |
| `session_stop` | `T-145` | 成功 |

### 5.2 文件链路结果

Linux 文件链路本轮未完全补齐，原因如下：

- 早期 `upload` 参数传法不对，目标路径被解析成目录，导致后续 `cat/download` 对原始路径验证失败
- 后续补测阶段，Linux 样本已不再继续轮询新任务
- 同时远端 SSH 重新登录未成功，导致无法把 Linux 样本重新拉起做第二轮文件补测

已有结果：

| 功能 | 任务 | 结果 |
|---|---|---|
| `upload` | `T-137` | 成功，但目标路径被解析为目录风格 |
| 上传后读取 | `T-138` | 失败，读取了目录路径 |
| `download` | `T-139` | 失败，路径是目录 |
| `remove` | `T-140` | 成功 |
| 删除后验证 | `T-141` | 成功，返回 `removed` |

结论：

- Linux 文件功能代码路径已跑到 Agent 执行阶段
- 但本轮 Linux 文件下载验证不算完整通过

## 6. 本轮测试中定位到的真实问题

### 6.1 `ps` 在 macOS 上失败

现象：

- `ps` 命令在 macOS callback `C-58` 上失败

原因：

- 当前实现直接调用 `/bin/ps`
- 在当前测试环境下被系统拒绝，报 `operation not permitted`

结论：

- 这是目标机执行环境限制
- 不是 Mythic / C2 / tasking 流程故障

### 6.2 `upload` 路径处理不符合直觉

现象：

- 指定目录 `/tmp/` 后，实际文件落地为 `/tmp/<file_id>`
- 而不是 `/tmp/<原始文件名>`

影响：

- 文件链路本身能跑通
- 但验证和使用体验不符合预期

### 6.3 SOCKS 任务成功，但真实代理流量失败

现象：

- `socks 7000` 任务成功
- `curl --socks5-hostname 127.0.0.1:7000 -I https://example.com` 返回：
  - `curl: (97) connection to proxy closed`

结论：

- 当前 SOCKS 的“任务层 / 管理层”是通的
- 但这轮在 macOS 新 callback 上，真实转发流量没有通过
- 需要后续单独排查 `socks_manager` 的连接泵或 session 生命周期

### 6.4 Linux 二次补测受远端登录问题影响

现象：

- 远端样本首次测试时已成功运行并完成多项任务
- 本轮补测时，SSH 重新登录 root 失败，无法重新启动样本

影响：

- Linux 基础功能和 session 已验证
- 但 Linux 文件链路未能在“修正参数后”再跑一轮完整闭环

## 7. 本轮总体结论

### 7.1 已确认成功的核心能力

- 自定义 `masked_https` C2 启动成功
- Beacon `HTTPS` 链路可用
- Session `WSS` 派生可用
- macOS 样本：
  - 上线成功
  - `sysinfo`
  - `shell`
  - `privesc_check`
  - `privesc_tool`
  - `report`
  - `upload`
  - `download`
  - `remove`
  - `session_start`
  - `session_status`
  - `pty`
  - `socks/socks_stop`
  - `session_stop`
  - `exit`
- Linux 样本：
  - 上线成功
  - `sysinfo`
  - `ps`
  - `shell`
  - `privesc_check`
  - `privesc_tool`
  - `session_start`
  - `session_status`
  - `pty`
  - `session_stop`

### 7.2 本轮未完全通过的项目

- macOS：`ps`
- macOS：SOCKS 真实代理流量
- Linux：修正参数后的文件上传/下载闭环

### 7.3 当前系统状态判断

从“毕业设计原型系统”的角度看，当前已经可以认为：

- 主链路已跑通
- 关键功能已有较完整的真实测试依据
- 但仍存在若干需要在后续继续收口的细节问题：
  - `upload` 目标文件名逻辑
  - `ps` 的跨平台实现
  - SOCKS 真实流量转发稳定性
  - Linux 远端补测环境恢复

## 8. 追加回归修复（2026-03-29）

### 8.1 `upload` 目标文件名逻辑已修复

修复点：

- Agent 现在会识别 Mythic 在 `upload` 任务里下发的 `目录/file_id` 形式路径。
- 若任务参数中同时带有 `filename`，则会自动恢复为 `目录/原始文件名`。

macOS 实测：

- callback：`C-60`
- 上传任务：`T-184`
- 上传参数：`/tmp/`
- 实际落地文件：
  - `/tmp/e2e_upload_mac_fix.txt`
- 服务端响应：
  - `文件已写入: /tmp/e2e_upload_mac_fix.txt (19 bytes)`
- 随后通过 `shell cat /tmp/e2e_upload_mac_fix.txt` 读回成功：
  - `upload fix content`
- `remove /tmp/e2e_upload_mac_fix.txt` 删除成功

结论：

- `upload` 不再错误落成 `file_id` 文件名
- 文件上传、读回、删除闭环已完成

### 8.2 `masked_https` 已升级为真正 Push C2

本轮关键修复：

- 重新编译并替换了真正包含 `push_session.go` 的 `masked_https_server`
- WSS Session 不再走“单条消息转 `/agent_message`”的伪流式路径
- 现在的 Session 链路是：
  - `Agent WSS <-> masked_https <-> Mythic gRPC PushC2Streaming`

重新验证后，Mythic 侧调试日志已经出现 Push 注册成功的完整证据：

- `grpc.push_stream.registration_candidate`
- `grpc.push_stream.lookup_ok`
- `grpc.addNewPushC2Client`
- `grpc.push_stream.add_client_ok`
- `grpc.GetPushC2ClientInfo.hit`

说明：

- session callback 已真正注册进 Mythic 的 PushC2 内存连接表
- 后续下发给 session callback 的任务，已经可以 direct push 到在线 session

### 8.3 真 Push Session 回归验证

使用新的 macOS payload：

- payload UUID：`1fe09b42-16d9-45c8-b429-a7b78c050dae`
- beacon callback：`C-93`
- session callback：`C-94`

实测结果：

| 功能 | 任务 | 结果 |
|---|---|---|
| `session_start` | `T-247` | 成功，派生 `C-94` |
| `session_status` | `T-248` | 成功 |
| `pty /bin/sh` | `T-249` | 成功，状态为 `interactive session started` |
| `socks {"local_port":7009}` | `T-250` | 成功 |
| `socks_stop {"local_port":7009}` | `T-251` | 成功 |
| `session_stop` | `T-252` | 成功 |
| `exit` | `T-253` | 成功 |

补充证据：

- `pty` 下发后，本机实际拉起了 `/bin/sh`
- Agent 日志出现持续的 `[session] push 消息`
- `masked_https` 日志出现持续的 `push session 收到 mythic 下推`

结论：

- 现在的 Session 已不再是“伪 session callback”
- 而是带有真正 Push 流式通道的 Session callback

### 8.4 SOCKS 实流量验证已通过

基于 session callback `C-94` 与本地代理端口 `7009`，本轮已完成真实外部流量测试：

| 测试 | 命令 | 结果 |
|---|---|---|
| 外部 HTTPS | `curl --socks5-hostname 127.0.0.1:7009 -I https://example.com --max-time 20` | 成功，返回 `HTTP/2 200` |
| 外部 HTTP | `curl --socks5-hostname 127.0.0.1:7009 -I http://example.com --max-time 20` | 成功，返回 `HTTP/1.1 200 OK` |

额外说明：

- 对 `http://127.0.0.1:18081` 的 SOCKS 测试失败是符合预期的
- 因为 `127.0.0.1` 会在 Agent 自身的回环环境中解析，而不是操作端本机

工程结论：

- `socks` 在真正 Push Session 上已经完成真实外部 HTTP/HTTPS 转发验证
- 之前的 `wrong address type` 与 `connection to proxy closed` 问题已被消除

### 8.5 测试残留清理

本轮结束后已完成收尾：

- `socks_stop` 已执行成功
- `session_stop` 已执行成功
- beacon `exit` 已执行成功
- 本地测试用 `callbackport` 历史记录已清理：
  - `delete from callbackport;`
  - 当前表内记录数：`0`

### 8.6 `rpfwd` 回归结果

本轮另外补测了基于真 Push Session 的 `rpfwd`，最终已验证通过。

测试载荷：

- payload UUID：`149b81f8-c14e-4790-a6b2-2df062eeb3e8`
- 第一轮：
  - beacon callback：`C-95`
  - session callback：`C-96`
- 第二轮复测：
  - beacon callback：`C-97`
  - session callback：`C-98`

关键结论：

- `rpfwd` 代码路径本身没有结构性错误
- 第一轮失败的根因是测试参数使用了：
  - `remote_ip=127.0.0.1`
- 对 `rpfwd` 来说，这个地址是 **Mythic 容器视角** 的 `127.0.0.1`，不是宿主机的回环地址
- 因此返回 `503` 是预期结果，不是 Agent 逻辑错误

修正后的测试方式：

1. 操作端本机启动临时 HTTP 服务：
   - `python3 -m http.server 18082 --bind 0.0.0.0`
2. 从 Mythic 容器验证宿主机可达地址：
   - `10.128.53.182:18082`
3. 在 session callback `C-98` 上下发：
   - `rpfwd {"local_port":18083,"remote_ip":"10.128.53.182","remote_port":18082}`
4. 操作端本机直接访问：
   - `curl -I http://127.0.0.1:18083 --max-time 15`

任务结果：

| 功能 | 任务 | 结果 |
|---|---|---|
| 第一轮 `session_status` | `T-255` | 成功 |
| 第一轮 `rpfwd` | `T-256` | 成功 |
| 第二轮 `session_status` | `T-261` | 成功 |
| 第二轮 `rpfwd` | `T-262` | 成功 |
| 第二轮 `rpfwd_stop` | `T-263` | 成功 |
| 第二轮 `session_stop` | `T-264` | 成功 |
| 第二轮 `exit` | `T-265` | 成功 |

实流量结果：

- `curl -I http://127.0.0.1:18083 --max-time 15`
  - 成功，返回：`HTTP/1.0 200 OK`
  - `Server: SimpleHTTP/0.6 Python/3.13.3`

临时 HTTP 服务日志：

- `10.128.53.182 - - [29/Mar/2026 13:49:26] "GET / HTTP/1.1" 200 -`
- `127.0.0.1 - - [29/Mar/2026 13:50:34] "HEAD / HTTP/1.1" 200 -`

结论：

- `rpfwd` 已完成：
  - 命令注册
  - 任务创建
  - 真 Push Session 下推
  - 启停命令回归
  - 真实端口转发验证

工程判断：

- `Beacon`
- `真 Push Session`
- `pty`
- `socks`
- `rpfwd`

这五条主链现已全部打通。
