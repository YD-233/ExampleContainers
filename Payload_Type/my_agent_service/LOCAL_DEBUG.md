# 本地调试模式

这个目录支持两种运行方式：

1. 容器模式
   由 Mythic 通过 `mythic-cli add/build/start` 拉起，连接 `mythic_rabbitmq`、`mythic_server`。

2. 本地调试模式
   直接在宿主机运行 Go 进程，连接 `127.0.0.1:5672` 和 `127.0.0.1:17444`。

## 结论

如果你是在宿主机直接运行这个 Payload Type 服务，不要使用 Docker 内部主机名。

## 启动命令

在当前目录执行：

```bash
make run_local_service
```

这会显式注入本地调试所需的关键环境变量：

- `RABBITMQ_HOST=127.0.0.1`
- `RABBITMQ_PORT=5672`
- `MYTHIC_SERVER_HOST=127.0.0.1`
- `MYTHIC_SERVER_PORT=17443`
- `MYTHIC_SERVER_GRPC_PORT=17444`

## 配套检查

先确认 Mythic 主服务已经启动：

```bash
/Users/zhujiayi/Documents/allMyCode/01_Work/Mythic/mythic-cli status
```

查看 Mythic 给远程/本地服务的连接参数：

```bash
/Users/zhujiayi/Documents/allMyCode/01_Work/Mythic/mythic-cli config service
```

如果需要看日志：

```bash
/Users/zhujiayi/Documents/allMyCode/01_Work/Mythic/mythic-cli logs mythic_server -f
/Users/zhujiayi/Documents/allMyCode/01_Work/Mythic/mythic-cli logs mythic_rabbitmq -f
```

## 现在这个项目的正确使用方式

- 本地调试 Payload Type 服务：`make run_local_service`
- 本地调试生成出来的 Agent：`make run_agent_local`

二者不要和 Docker 内部主机名混用。
