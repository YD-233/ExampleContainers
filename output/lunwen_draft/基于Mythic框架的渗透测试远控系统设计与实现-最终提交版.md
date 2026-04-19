# 基于Mythic框架的渗透测试远控系统设计与实现

## 摘要
随着命令与控制平台不断向模块化、容器化和可扩展化演进，单体式远控程序已难以满足现代授权渗透测试对通信适配、任务编排和跨平台运行的综合要求。围绕 Mythic 框架的扩展开发能力，本文设计并实现了一套基于 Mythic 框架的渗透测试远控系统。系统由 Mythic 控制端、自定义 C2 Profile“masked-https”以及 Go 语言编写的 Agent“my-agent”组成，构建了兼顾低频轮询与高频交互的 Beacon + Session 双模通信体系。其中文件传输、命令执行、系统信息采集等离散任务通过 HTTPS Beacon 链路完成，交互式 PTY、SOCKS 代理和反向端口转发等实时能力通过 WSS Session 链路与 Push C2 机制实现。

在系统实现层面，本文完成了 Payload Type 构建模块、自定义命令注册、Profile 容器部署、AES-HMAC 报文封装、回调注册、任务调度、文件浏览器模型适配和跨平台构建等关键功能，并在 Mythic 平台中接入了 sysinfo、shell、upload、download、remove、ps、report、privesc-check、privesc-tool、session-start、pty、socks、rpfwd 等命令。结合联调结果，系统已在 macOS、Linux 两类平台完成 callback 上线、任务执行、文件传输、Session 派生和代理转发等核心链路验证，累计覆盖 20 余项核心任务链路；Windows 平台完成了目标样本构建与兼容性适配验证。测试结果表明，该系统能够较稳定地满足授权测试场景下的终端控制、信息收集和代理转发需求，也为基于 Mythic 的自定义 Agent 与 C2 Profile 研究提供了可复用的工程样例。

关键词：渗透测试；Mythic框架；命令与控制；远控系统；Beacon；Session

## ABSTRACT
As command-and-control platforms continue to evolve toward modular, containerized, and extensible architectures, monolithic remote administration tools are no longer sufficient for modern authorized penetration testing. Focusing on the extensibility of the Mythic framework, this thesis designs and implements a penetration testing remote control system based on Mythic. The system is composed of the Mythic control platform, a custom C2 Profile named masked-https, and a Go-based Agent named my-agent, forming a dual-mode Beacon plus Session communication architecture. Discrete tasks such as command execution, file transfer, and host information collection are carried by the HTTPS Beacon channel, while interactive PTY sessions, SOCKS proxying, and reverse port forwarding are supported through a WSS Session channel together with Mythic Push C2.

At the implementation level, the work completes payload building, command registration, Profile container deployment, AES-HMAC message wrapping, callback registration, task scheduling, file browser adaptation, and cross-platform build support. The integrated prototype supports sysinfo, shell, upload, download, remove, ps, report, privesc-check, privesc-tool, session-start, pty, socks, and rpfwd related commands within Mythic. End-to-end tests on macOS and Linux cover more than twenty core task paths, including callback registration, command execution, file transfer, Session creation, and proxy-related workflows, while the Windows target build and compatibility adaptation have also been verified. The results show that the system can stably support node control, information collection, and proxy forwarding in authorized test scenarios, and it also serves as a reusable engineering example for research on custom Agents and C2 Profiles in the Mythic ecosystem.

Keywords: penetration testing; Mythic framework; command and control; remote control system; Beacon; Session

## 1. 绪论
### 1.1 背景与意义
随着信息技术的飞速发展，网络安全问题日益突出。根据中国国家互联网应急中心（CNCERT）发布的《2024年中国互联网网络安全报告》显示，我国面临的网络安全威胁呈现多样化、复杂化趋势，各类网络攻击事件频发，给企业和个人带来了巨大的经济损失和安全隐患[1]。在这一背景下，渗透测试作为一种主动的安全评估手段，得到了越来越广泛的应用。

渗透测试（Penetration Testing）是指通过模拟恶意攻击者的技术手段，对目标系统的安全性进行全面评估的过程[3]。其核心目标是发现系统中存在的安全漏洞，验证漏洞的可利用性，并提供相应的修复建议。渗透测试不仅可以帮助企业发现潜在的安全风险，还可以验证现有安全防护措施的有效性，为安全决策提供数据支持。

在渗透测试过程中，命令与控制（Command and Control，简称C2）系统扮演着至关重要的角色[10]。C2系统是攻击者与目标系统之间的通信桥梁，负责接收攻击者下发的指令、执行相应操作，并将执行结果回传给攻击者。一个优秀的C2系统需要具备以下特点：一是隐蔽性，能够规避各类安全防护设备的检测；二是稳定性，能够在复杂的网络环境中保持长期稳定的通信；三是功能性，能够支持丰富的后渗透功能，满足各种渗透测试场景的需求[10]。

然而，随着各类安全防护设备的广泛部署，传统的渗透测试远控木马（如早期的通用型RAT）因固定的通信指纹和极易被拦截的执行逻辑，已无法满足现代渗透测试的需求[8]。现代C2系统正朝着高度模块化、通信隐蔽化和跨平台化的方向发展。因此，设计并实现一套支持流量伪装、具备跨平台运行能力的远控系统，对于提高红队评估人员在异构网络中的作业效率与隐蔽性，以及研究和防御隐蔽通信机制均具有重要的理论意义和实践价值[5]。

### 1.2 国内外研究现状
#### 1.2.1 国外研究现状
在国际上，现代C2框架（如Cobalt Strike、Mythic、Sliver等）已全面普及模块化设计与内存执行技术（如BOF），通过动态调整通信协议特征（Malleable C2）来规避网络边界的深度数据包检测（DPI）[8]。Cobalt Strike作为商业C2框架的代表，提供了强大的团队协作功能和丰富的后渗透模块，但其商业授权费用高昂且源码不开放，限制了其在学术研究和个人学习中的应用。Mythic作为一个开源的容器化C2框架，采用现代化的微服务架构，支持自定义Agent和C2 Profile的开发，具有良好的扩展性和灵活性，是本课题的主要研究基础[5]。Sliver则是由Bishop Fox开发的开源C2框架，采用Go语言编写，具有跨平台、免杀效果好等特点，为本系统的Agent开发提供了重要参考[9]。

趋势上，国外研究重点倾向于使用跨平台且具备一定底层控制力的语言（如Go、Rust）重构远控客户端，以应对多样化的终端环境[20]。Go 语言因其简洁的语法、高效的并发模型和出色的跨平台编译能力，越来越受到安全研究者的青睐[44]。Rust语言则因其内存安全特性和高性能，也开始在安全工具开发中得到应用。在通信隐蔽性方面，研究者们提出了多种技术，如域前置（Domain Fronting）、DNS隧道、ICMP隧道等，以规避网络检测[11]。

#### 1.2.2 国内研究现状
国内方面，各大安全厂商与红队实验室在攻防演练平台建设上投入巨大。学术界近两年的热点集中在“加密流量特征的自动化隐藏与识别”以及“针对高级操作系统安全机制（如UAC）的绕过技术”[12]。刘玲在《加密流量特征隐藏研究与实现》中提出了多种加密流量伪装技术，有效提升了C2通信的隐蔽性[13]。徐美芳、林哲在《基于企业复杂内网环境的渗透测试与防护》中深入分析了复杂内网环境下的渗透测试技术，为本系统的内网穿透功能设计提供了重要参考[30]。冀俊涛、石磊在《基于ATT&CK框架的实战分析》中系统地整理了MITRE ATT&CK框架中的攻击技术，为本系统的功能设计提供了技术参考[31]。

综合国内外研究现状可以看出，C2技术正朝着更加隐蔽、更加智能的方向发展[36]。本课题将紧跟这一趋势，侧重于在Mythic框架的生态下，通过Go语言高并发网络编程与底层API调用，实现高度定制化的隐蔽远控链路与内网穿透能力。同时，本课题还将关注系统的实战应用价值，确保研究成果能够真正应用于渗透测试实践[32]。

### 1.3 主要内容
本毕业设计的主要研究内容包括以下几个方面：

（1）C2通信协议层设计：基于Go开发符合Mythic接口规范的自定义C2 Profile容器，实现基于HTTP/HTTPS的流量伪装、心跳间隔抖动（Jitter）配置以及基于AES等算法的数据包序列化与加密传输[36]。通信协议设计需要考虑隐蔽性、可靠性和效率三个方面的要求，确保C2流量能够规避常见的网络检测手段，同时保持稳定的通信质量。

（2）远控核心引擎设计：采用Go语言开发跨平台植入体（Agent），实现多线程的任务轮询调度逻辑，确保指令解析与系统命令执行的异步非阻塞运行[21]。Agent需要支持Windows和Linux两种操作系统，能够自动适配不同的系统环境。核心引擎需要具备良好的稳定性和容错能力，能够在网络异常或系统资源受限的情况下正常工作。

（3）高级渗透功能开发：在Agent中集成针对特定操作系统的权限提升辅助功能，并开发基于长连接的内网SOCKS5流量代理转发模块，支撑多级路由穿透[14]。权限提升功能需要检测当前用户权限、枚举可提权漏洞、尝试已知提权技术等。SOCKS代理功能需要支持TCP和UDP协议，能够处理多个并发连接，并具备稳定的长连接维持能力[18]。

（4）平台UI适配与集成：编写标准化的负载生成配置（Payload Type）与命令字典（Command JSON），实现与Mythic Web前端管理界面的无缝对接，提供可视化的任务下发与结果审计[5]。平台集成需要遵循Mythic框架的规范，确保自定义的Agent和C2 Profile能够与Mythic平台正常通信和协作。

（5）系统测试与验证：构建虚拟测试环境，对系统的功能完整性、跨平台兼容性（Windows/Linux）、通信隐蔽性及长时间运行的稳定性进行全面测试[40]。测试工作需要覆盖各种正常和异常场景，验证系统在实际应用中的可靠性和有效性。

### 1.4 论文章节组织
本文后续内容安排如下：

第2章：需求分析。分析系统的功能需求和非功能需求，明确系统需要实现的核心功能和性能指标。

第3章：相关技术。介绍渗透测试中的C2技术、Mythic框架架构原理、Agent开发技术以及流量隐蔽与规避技术。

第4章：系统设计。详细设计系统的整体架构、C2通信协议、Agent软件架构以及各功能模块的设计方案。

第5章：系统实现。介绍系统的开发环境搭建、C2 Profile适配实现以及Agent核心功能的代码实现。

第6章：系统测评。对系统进行全面的测试，包括通信协议测试、功能测试、跨平台测试等，并对测试结果进行分析。

第7章：结论与展望。总结本文的主要工作与实现成果，并对未来的研究方向进行展望。

## 2. 需求分析
### 2.1 功能分析
#### 2.1.1 远控代理（Agent）功能需求
Agent需要能够收集目标系统的基本信息，包括操作系统类型和版本、主机名、用户名、网络配置、进程列表等。这些信息对于后续的攻击决策和横向移动具有重要参考价值。系统信息收集功能应在Agent启动时自动执行，并将收集到的信息回传给控制端[30]。

Agent需要能够执行控制端下发的系统命令，并将命令的输出结果回传给控制端。命令执行功能应支持Windows和Linux两种操作系统，能够正确处理命令的标准输出和标准错误输出。为了提高隐蔽性，命令执行应采用异步方式，避免阻塞Agent的主循环[3]。

Agent需要支持基本的文件操作功能，包括文件上传、文件下载、文件删除、目录遍历等。文件上传下载功能应支持大文件分块传输，避免因文件过大导致传输失败。

#### 2.1.2 通信通道（C2 Profile）功能需求
C2 Profile需要支持通信伪装功能，通过配置User-Agent、Host、请求路径、请求头等参数，使C2流量看起来像正常的Web流量[12]。通信伪装功能应支持HTTP和HTTPS协议，HTTPS协议需要支持自定义证书配置。此外，还应支持心跳间隔抖动（Jitter）配置，使心跳请求的时间间隔随机变化，规避基于时间特征的检测。

C2 Profile需要实现请求解析功能，能够解析Agent发送的HTTP请求，提取加密的任务数据和执行结果，解密后转发给Mythic服务。同时需要实现响应生成功能，从Mythic服务获取待执行的任务列表，加密后封装成HTTP响应返回给Agent[11]。

#### 2.1.3 非功能性需求
跨平台兼容性：Agent需要具备良好的跨平台兼容性，能够在Windows和Linux操作系统上正常运行。Windows平台应支持Windows 7及以上版本，Linux平台应支持主流的Linux发行版（如Ubuntu、CentOS、Debian等）[20]。

通信隐蔽性：系统的C2通信需要具备良好的隐蔽性，能够规避常见的网络检测手段。通信数据应采用加密算法（如AES）进行加密，防止被中间人攻击和流量分析。通信流量应伪装成正常的Web流量，避免被防火墙和入侵检测系统拦截[25]。

运行稳定性：Agent需要具备良好的运行稳定性，能够长时间稳定运行而不出现崩溃或掉线。在网络抖动或短暂断网的情况下，Agent应能够自动重连，确保通信的连续性。

### 2.2 系统业务流程分析
#### 2.2.1 植入体（Agent）上线注册流程
Agent启动后，首先进行系统信息收集，收集操作系统类型、版本、主机名、用户名、IP地址等信息。然后生成唯一的Agent ID和加密密钥，将收集到的信息通过初始心跳请求发送给C2 Profile。C2 Profile将Agent的请求转发给Mythic服务，Mythic服务创建对应的Agent记录，并返回确认响应。Agent收到确认后，进入正常的心跳循环，等待接收任务[36]。

#### 2.2.2 心跳通信与指令交互流程
Agent以固定的时间间隔向C2 Profile发送心跳请求，请求中携带Agent ID和上次任务的执行结果（如果有）。C2 Profile将请求转发给Mythic服务，Mythic服务查询该Agent是否有待执行的任务，如果有则将任务列表返回给C2 Profile，C2 Profile将任务列表加密后封装成HTTP响应返回给Agent。Agent解析响应，获取任务列表并依次执行[36]。

#### 2.2.3 任务执行与结果回传流程
Agent接收到任务后，根据任务类型调用相应的处理函数执行任务。任务类型包括：shell（执行系统命令）、upload（上传文件）、download（下载文件）、socks（启动SOCKS代理）等。任务执行完成后，Agent将执行结果通过下一次心跳请求发送给C2 Profile，C2 Profile将结果转发给Mythic服务，Mythic服务更新任务状态并存储执行结果[3]。

### 2.3 本章小结
本章对基于Mythic框架的渗透测试远控系统进行了详细的需求分析。首先分析了系统的功能需求，包括Agent功能需求、C2 Profile功能需求以及非功能性需求。然后分析了系统的业务流程，包括Agent上线注册流程、心跳通信与指令交互流程以及任务执行与结果回传流程。通过需求分析，明确了系统需要实现的核心功能和性能指标，为后续的系统设计奠定了基础。

## 3. 相关技术
### 3.1 渗透测试中的命令与控制（C2）技术概述
命令与控制（Command and Control，C2）技术是渗透测试和红队行动中的核心技术之一[10]。C2系统作为攻击者与目标系统之间的通信桥梁，承担着指令下发、结果回传、资产管理和横向支撑等关键功能。随着网络安全防护技术的不断发展，现代C2系统已经演变为复杂的分布式系统，具备高度的模块化、隐蔽化和跨平台化特征[10]。

#### 3.1.1 C2架构模型
现代C2系统通常采用分布式架构设计，主要包括控制端（Team Server）、通信网关（C2 Profile）和植入体（Agent）三个核心组件[10]。控制端负责提供操作界面、任务管理、数据存储等功能；通信网关负责处理Agent的通信请求，实现流量转发和协议转换；植入体运行在目标系统上，负责执行控制端下发的指令。

常见的C2架构模型包括：中心式架构，所有Agent直接连接到单一控制端；分布式架构，多个控制端协同工作，Agent可以连接到任意控制端；P2P架构，Agent之间可以相互通信，形成网状网络[10]。本系统采用中心式架构，通过Mythic框架提供控制端功能，自定义C2 Profile作为通信网关。

#### 3.1.2 远控植入体（Agent）的生命周期与运行机制
Agent的生命周期通常包括以下阶段：初始部署阶段，Agent被部署到目标系统并首次运行；上线注册阶段，Agent收集系统信息并向控制端注册；心跳通信阶段，Agent定期与控制端通信，获取任务并回传结果；任务执行阶段，Agent接收并执行控制端下发的任务；终止阶段，Agent收到终止指令或发生致命错误时退出[10]。

Agent的运行机制通常采用事件驱动或轮询模式。事件驱动模式下，Agent通过回调函数响应系统事件；轮询模式下，Agent定期查询控制端是否有新任务。本系统采用轮询模式，Agent以固定时间间隔发送心跳请求，获取待执行的任务列表[36]。

#### 3.1.3 心跳轮询（Beaconing）与异步通信机制
心跳轮询（Beaconing）是C2系统中常用的通信机制，Agent以固定的时间间隔向控制端发送心跳请求，报告自身状态并获取新的任务指令[36]。心跳间隔可以根据实际需求进行配置，通常在几秒到几分钟之间。为了规避基于时间特征的检测，现代C2系统支持心跳抖动（Jitter）功能，使心跳间隔在一定范围内随机变化[36]。

异步通信机制允许Agent在执行长时间任务时不会被阻塞，可以继续响应控制端的其他指令。本系统采用Go语言的协程（Goroutine）实现异步通信，每个任务在独立的协程中执行，主循环继续处理心跳通信[21]。

### 3.2 Mythic框架架构原理
Mythic 是一个开源的、现代化的命令与控制框架，采用容器化微服务架构设计[5][41-43]。Mythic 框架的核心设计理念是模块化和可扩展性，通过标准化接口允许开发者自定义 Agent 和 C2 Profile，实现灵活的定制化开发。

#### 3.2.1 基于Docker的微服务解耦架构
Mythic 框架采用 Docker 容器技术实现微服务架构，各个组件以独立容器运行，通过标准化接口进行通信[23][41-45]。主要组件包括：Mythic 主服务容器，提供 Web 界面和 API 接口；PostgreSQL 数据库容器，存储任务、回调、文件等数据；RabbitMQ 消息队列容器，实现组件间的异步消息通信；Payload Type 容器，负责 Agent 的构建和命令处理；C2 Profile 容器，处理 Agent 的通信请求。

微服务架构的优势在于各组件可以独立开发、部署和升级，不会影响系统的整体运行。开发者可以专注于单个组件的开发，而无需了解整个系统的内部实现细节[23]。

#### 3.2.2 动态C2 Profile适配机制
Mythic框架支持动态加载C2 Profile，开发者可以编写自定义的C2 Profile来实现特定的通信协议[23]。C2 Profile需要实现Mythic定义的RPC接口，包括处理Agent请求、转发数据到Mythic服务、从Mythic服务获取任务等功能。Mythic框架通过gRPC协议与C2 Profile通信，实现了高效的跨语言服务调用。

C2 Profile的配置信息通过JSON文件定义，包括通信协议类型、监听端口、加密方式、流量伪装参数等。Mythic框架在启动时读取配置文件，自动加载和初始化C2 Profile[5]。

### 3.3 植入体（Agent）开发技术
Agent的开发需要综合考虑跨平台兼容性、隐蔽性、稳定性和功能性等多个方面的要求[20]。本系统采用Go语言开发Agent，利用Go语言的跨平台编译能力和丰富的标准库，实现高效、稳定的远控功能。

#### 3.3.1 跨平台编译技术
Go语言内置了强大的跨平台编译能力，通过设置GOOS和GOARCH环境变量，可以在一个平台上编译出适用于其他平台的可执行文件[20]。例如，在Linux系统上编译Windows可执行文件，只需执行命令：GOOS=windows GOARCH=amd64 go build。Go语言的标准库也提供了良好的跨平台抽象，大部分代码可以在不同平台上直接运行，无需修改。

对于必须使用平台特定API的功能，Go语言提供了条件编译和运行时检测机制。条件编译通过文件名后缀（如_windows.go、_linux.go）实现，运行时检测通过runtime.GOOS变量实现[20]。本系统综合使用这两种机制，实现对Windows和Linux平台的兼容。

#### 3.3.2 操作系统API调用与进程控制技术
Agent需要调用操作系统API来实现各种功能，如系统信息收集、命令执行、文件操作等[20]。Go语言通过syscall包和golang.org/x/sys包提供了对操作系统API的访问能力。Windows平台下，可以通过syscall.LoadDLL和syscall.GetProcAddress加载和调用Windows API；Linux平台下，可以直接调用系统调用或使用CGo绑定C库函数。

进程控制技术包括创建子进程、捕获标准输出、设置超时等[3]。Go语言的os/exec包提供了便捷的进程控制接口，可以创建子进程执行系统命令，通过pipe捕获标准输出和标准错误输出。本系统使用os/exec包实现命令执行功能，并设置超时机制防止命令长时间挂起。

### 3.4 流量隐蔽与规避技术
流量隐蔽是C2系统的核心能力之一，通过各种技术手段使C2流量看起来像正常的网络流量，规避防火墙、入侵检测系统等安全防护设备的检测[12]。

#### 3.4.1 基于HTTP/S的流量伪装技术
HTTP/HTTPS是互联网最常用的协议，流量特征常见，不易被识别为恶意流量[12]。本系统的C2通信基于HTTP/HTTPS协议，通过配置User-Agent、Host、请求路径、请求头等参数，使C2流量伪装成正常的Web流量。例如，可以将User-Agent设置为常见浏览器的User-Agent字符串，将请求路径设置为常见的API路径，将Host设置为合法的域名。

HTTPS协议额外提供传输层加密，进一步提高通信安全性[12]。本系统支持自定义SSL证书配置，可以使用合法的SSL证书或自签名证书。使用合法证书时，C2流量与正常HTTPS流量无法区分；使用自签名证书时，需要在Agent中配置跳过证书验证。

#### 3.4.2 数据序列化与加密传输技术
通信数据的加密是保障C2通信安全的重要手段[25]。本系统采用AES-256-CBC算法对通信数据进行加密，密钥长度为32字节，初始化向量（IV）长度为16字节。加密流程如下：首先生成随机的IV，然后使用AES算法对明文数据进行加密，最后将IV和密文拼接，进行Base64编码。解密流程相反：首先进行Base64解码，然后提取IV和密文，最后使用AES算法解密密文。

数据序列化采用JSON格式，便于解析和扩展[25]。JSON是一种轻量级的数据交换格式，易于人阅读和编写，也易于机器解析和生成。本系统定义了标准的JSON数据结构，包括任务请求、任务响应、系统信息等。

### 3.5 本章小结
本章介绍了与基于Mythic框架的渗透测试远控系统相关的技术。首先概述了渗透测试中的C2技术，包括C2架构模型、Agent的生命周期与运行机制、心跳轮询与异步通信机制。然后详细介绍了Mythic框架的架构原理，包括基于Docker的微服务解耦架构和动态C2 Profile适配机制。接着介绍了Agent开发技术，包括跨平台编译技术和操作系统API调用与进程控制技术。最后介绍了流量隐蔽与规避技术，包括基于HTTP/S的流量伪装技术和数据序列化与加密传输技术。这些技术为后续的系统设计和实现提供了理论基础。

## 4. 系统设计
### 4.1 设计目标
本系统的设计目标是在Mythic框架的基础上，构建一套功能完善、隐蔽性强、跨平台兼容的渗透测试远控系统。具体设计目标包括：

（1）功能完整性：系统应支持系统信息收集、命令执行、文件管理、权限提升辅助、内网代理转发等核心功能，满足渗透测试的实际需求[26]。

（2）通信隐蔽性：C2通信应能够规避常见的网络检测手段，通过流量伪装、数据加密、心跳抖动等技术提高通信隐蔽性。

（3）跨平台兼容性：Agent应能够在Windows和Linux操作系统上正常运行，代码应具备良好的可移植性[25]。

（4）运行稳定性：系统应具备良好的运行稳定性，能够长时间稳定运行而不出现崩溃或掉线，在网络异常情况下能够自动恢复。

（5）平台集成性：系统应与Mythic框架无缝集成，支持通过Mythic Web界面进行任务下发和结果查看[14]。

### 4.2 总体设计
本系统采用“微服务+C/S”的分布式架构设计，分为控制端容器群、通信网关和终端执行代理三个主要部分[15]。这种架构设计具有良好的可扩展性和容错性，各组件可以独立部署和升级，不影响系统的整体运行。

（1）控制端方案：复用Mythic框架提供的PostgreSQL数据库与RabbitMQ消息队列机制，确保海量心跳日志的存储与多用户协同操作的强一致性[14]。Mythic服务提供Web界面供操作员进行任务下发、结果查看、文件管理等操作。Web界面基于React开发，具有现代化的用户交互体验，支持实时更新和多用户协作。

（2）通信方案：采用短轮询（Beaconing）模式，Agent周期性向C2 Profile监听器发送伪装的HTTP GET/POST请求获取任务并回传执行结果[17]。流量特征全面模拟正常业务往来，通过配置User-Agent、Host、请求路径等参数实现流量伪装。支持心跳间隔抖动（Jitter）配置，使心跳请求的时间间隔在一定范围内随机变化。

（3）检测与执行流设计：Agent内部利用Go协程构建主控循环[18]。接收到JSON格式指令后，通过反射或命令路由分发至具体的功能模块（如Shell执行、文件I/O、提权或代理模块），并将标准输出捕获回传。各功能模块独立运行，互不影响，确保系统的稳定性和可扩展性。

### 4.3 C2通信协议（Profile）设计
#### 4.3.1 通信拓扑结构设计
本系统的通信拓扑采用星型结构，所有Agent直接连接到C2 Profile，C2 Profile再与Mythic服务通信[19]。这种结构的特点是实现简单、管理方便，适合毕业设计阶段对核心通信链路进行集中验证。后续如果需要支撑更复杂的部署场景，也可以部署多个 C2 Profile 实例，并通过负载均衡器分发 Agent 的连接请求。

通信链路采用HTTPS协议，提供传输层加密。Agent与C2 Profile之间建立TLS连接，C2 Profile与Mythic服务之间通过gRPC over TLS通信[18]。这种设计确保了通信数据的机密性和完整性。

#### 4.3.2 协议报文结构设计
通信数据包格式采用JSON格式，便于解析和扩展[18]。定义了以下标准数据结构：

（1）任务请求结构：包含Agent ID、任务ID、任务类型、任务参数等字段。任务类型包括shell、upload、download、socks等。

（2）任务响应结构：包含任务ID、执行结果、错误信息、执行时间等字段。执行结果可以是文本输出、文件数据或结构化数据。

（3）系统信息结构：包含操作系统类型、版本、主机名、用户名、IP地址、进程列表等字段。系统信息在Agent上线时上报[18]。

#### 4.3.3 流量伪装策略设计
流量伪装策略通过配置以下参数实现[22]：

（1）User-Agent：设置为常见浏览器的User-Agent字符串，如Mozilla/5.0(Windows NT 10.0; Win64; x64) AppleWebKit/537.36。

（2）Host：设置为合法的域名，可以使用域前置技术将真实C2服务器隐藏在CDN后面。

（3）请求路径：设置为常见的API路径，如/api/v1/data、/cdn-cgi/submit等，避免使用明显的恶意路径[22]。

（4）请求头：添加常见的HTTP请求头，如Accept、Accept-Language、Accept-Encoding等，模拟正常浏览器的请求行为。

（5）响应内容：C2 Profile对非法请求返回正常的HTTP响应，如404页面或静态资源，避免暴露C2服务器的存在。

#### 4.3.4 加密传输方案设计
通信数据采用AES-256-CBC算法进行加密，密钥长度为32字节，初始化向量（IV）长度为16字节[25]。加密流程如下：首先生成随机的IV，然后使用AES算法对明文数据进行加密，最后将IV和密文拼接，进行Base64编码。解密流程相反：首先进行Base64解码，然后提取IV和密文，最后使用AES算法解密密文。

密钥在Agent初始化时随机生成，并通过初始请求传递给控制端[5]。HTTPS协议额外提供传输层加密，进一步提高通信安全性。即使攻击者截获了通信数据，也无法解密获取明文内容。

### 4.4 远控代理（Agent）软件架构设计
#### 4.4.1 模块化分层设计
Agent采用模块化分层设计，主要分为以下几层[5]：

（1）配置层：负责解析命令行参数和配置文件，获取C2服务器地址、心跳间隔、抖动比例等配置。

（2）通信层：负责与C2服务器进行通信，包括发送心跳请求、上传执行结果、下载任务数据等。

（3）任务调度层：负责接收控制端下发的任务，调用相应的处理函数执行任务，并将执行结果回传给控制端[5]。

（4）功能模块层：包含各种功能模块的实现，如系统信息收集、命令执行、文件操作、权限提升、代理转发等。

（5）平台适配层：负责处理不同操作系统之间的差异，提供统一的接口供上层模块调用。

#### 4.4.2 任务调度器设计
任务调度器是Agent的核心组件，负责协调各功能模块的运行[5]。调度器采用生产者-消费者模式，主循环作为生产者，不断从C2服务器获取任务并放入任务队列；工作线程作为消费者，从任务队列中取出任务并执行。

任务队列采用带缓冲的channel实现，可以存储多个待执行的任务[5]。当任务队列满时，主循环会阻塞等待，避免内存溢出。每个任务在独立的协程中执行，避免阻塞主循环。任务执行完成后，结果通过另一个channel传递给主循环，由主循环在下一次心跳时上报。

#### 4.4.3 指令解析与分发机制设计
指令解析模块负责解析控制端下发的JSON格式任务，提取任务类型和参数[36]。任务类型包括：shell（执行系统命令）、upload（上传文件）、download（下载文件）、remove（删除文件）、sysinfo（收集系统信息）、ps（枚举进程）、privesc-check（权限提升检测）、socks（启动SOCKS代理）等。

指令分发模块根据任务类型，调用相应的处理函数执行任务[36]。本系统使用map[string]func结构实现指令分发，键为任务类型，值为对应的处理函数。这种设计便于扩展新的任务类型，只需在map中注册新的处理函数即可。

#### 4.4.4 Shell命令执行模块设计
命令执行模块负责执行系统命令，并将输出结果回传给控制端[3]。模块使用Go的os/exec包创建子进程执行命令，通过pipe捕获标准输出和标准错误输出。命令执行采用异步方式，避免阻塞主循环。

针对Windows和Linux系统的差异，命令执行模块分别实现了对应的处理逻辑[3]。Windows系统使用cmd.exe或powershell.exe作为命令解释器，Linux系统使用/bin/sh作为命令解释器。PowerShell是Windows强大的脚本环境，可以执行复杂的系统管理任务。

执行结果经过截断处理，避免传输过大的数据导致网络拥塞。默认截断长度为64KB，超过长度的输出会被截断并添加省略号[3]。

#### 4.4.5 文件上传下载逻辑设计
文件上传功能允许控制端将文件传输到目标系统。上传的数据通过HTTP POST请求发送，支持大文件分块传输[18]。文件下载功能允许控制端从目标系统获取文件，文件数据通过HTTP响应返回，同样支持分块传输。

文件操作模块实现了以下功能：目录浏览（ls）、文件上传（upload）、文件下载（download）、文件删除（remove）[5]。目录浏览返回文件列表，包含文件名、大小、修改时间等信息。文件上传下载支持断点续传，当传输中断时可以从断点处继续传输。

#### 4.4.6 进程列表获取模块设计
进程列表获取模块用于枚举目标系统的运行进程，帮助操作员了解系统的运行状态[40]。Windows系统使用WMI（Windows Management Instrumentation）接口或Windows API获取进程信息；Linux系统读取/proc文件系统获取进程信息。

获取的进程信息包括进程ID、进程名、可执行文件路径、CPU使用率、内存使用量等[40]。进程列表以表格形式展示，便于操作员查看和分析。

### 4.5 本章小结
本章详细设计了基于Mythic框架的渗透测试远控系统。首先明确了系统的设计目标，包括功能完整性、通信隐蔽性、跨平台兼容性、运行稳定性和平台集成性。然后介绍了系统的总体架构设计，包括控制端方案、通信方案和检测与执行流设计。接着详细设计了C2通信协议，包括通信拓扑结构、协议报文结构、流量伪装策略和加密传输方案。最后设计了Agent的软件架构，包括模块化分层设计、任务调度器设计、指令解析与分发机制设计、Shell命令执行模块设计、文件上传下载逻辑设计和进程列表获取模块设计。

## 5. 系统实现
### 5.1 开发环境搭建
本系统的开发与联调围绕 Mythic 容器化控制端、自定义 Payload Type 服务、自定义 C2 Profile 以及 Agent 三部分展开。控制端使用 Mythic 社区版环境，核心依赖包括 PostgreSQL、RabbitMQ 与 Mythic Web 界面；自定义服务包含 `my-agent-service` 与 `masked-https`；Agent 与 Profile 均采用 Go 语言实现，便于在同一语言栈内完成编译、调试与接口对接[41-45]。

在工程组织上，`Payload-Type/my-agent-service` 目录负责命令注册、构建流程和 Payload 元数据声明，`C2-Profiles/masked-https` 目录负责 Beacon 与 Session 两类通信路径的适配，`agent_code` 目录负责本地任务执行与通道控制。该组织方式与 Mythic 推荐的“Payload Type + C2 Profile”解耦模式保持一致，便于分别定位构建、通信和执行问题[41-43]。

表 5-1 核心目录与职责划分
| 目录或文件 | 作用说明 |
|---|---|
| `agentfunctions/builder.go` | 根据 Mythic 下发的构建参数生成 payload，并把配置写入构建产物 |
| `agentfunctions/*.go` | 注册命令、声明参数、处理 tasking 与 UI 对接 |
| `agent_code/controller.go` | 管理 checkin、轮询、任务执行与结果回传主流程 |
| `agent_code/controller_push.go` | 管理 Session callback 的 Push C2 长连接逻辑 |
| `agent_code/transports.go` | 抽象 HTTPS Beacon 与 WSS Session 两类传输通道 |
| `server/main.go` | 初始化 HTTPS 服务、路由与基础 decoy 响应 |
| `server/push_session.go` | 绑定 Agent WebSocket 连接与 Mythic Push C2 gRPC 流 |

在联调阶段，常用开发工具包括 Go 编译工具链、Docker、Mythic Web 界面以及本地日志输出。Go 工具链主要用于跨平台构建与本地调试，Docker 用于隔离控制端与 Profile 容器环境，Mythic Web 界面用于观察回连状态、命令执行结果与文件浏览器显示效果。相较于只做静态代码阅读，这种“容器启动—Payload 构建—样本运行—界面核对”的流程更能体现毕业设计项目的工程完整性。

[此处插入截图：图 5.1 Payload 创建与命令选择界面]
图 5.1 Payload 创建与命令选择界面

[此处插入截图：图 5.2 Payload 命令详情界面]
图 5.2 Payload 命令详情界面

### 5.2 C2 Profile适配实现
#### 5.2.1 Profile Docker容器配置
`masked-https` 以独立容器方式接入 Mythic，运行时通过配置文件统一管理监听地址、Beacon 路径、Session 路径、证书、Header 伪装参数和 AESPSK。容器对外主要暴露 8443 端口，其中 `/cdn-cgi/submit` 用于处理 Beacon 心跳与结果回传，`/cdn-cgi/ws` 用于处理基于 WebSocket 的 Session 流量。对于非目标路径，请求会进入 decoy 响应逻辑，以降低服务外观的单一性[41][45]。

从实现效果看，Profile 容器启动后可在 Mythic 的 Installed Services 页面被识别并管理，操作员能够直接在 Web 界面完成启动、停止与 Payload 绑定操作。该方式避免了将通信服务硬编码到控制端内部，更符合 Mythic 的组件化扩展思路。

[此处插入截图：图 5.3 Installed Services 中 masked-https 服务状态]
图 5.3 Installed Services 中 masked-https 服务状态

#### 5.2.2 服务端监听逻辑代码实现
服务端监听逻辑采用 Go 标准库 `net/http` 与 WebSocket 组件实现。对于 Beacon 请求，Profile 先根据路径与 Header 进行基础校验，再解码并还原 Mythic 标准 envelope，随后调用内部转发逻辑完成 checkin、get-tasking 与 post-response 处理。对于 Session 请求，服务端在完成 WebSocket 升级后，将当前连接与 Push C2 流建立绑定，使 Session callback 能够接收 Mythic 主动推送的任务[41][43]。

项目实际实现中，`server/main.go` 负责初始化 HTTPS 服务、证书加载和路由注册；`server/push_session.go` 负责 Session 连接、消息泵与 Push C2 的桥接。相对于只支持轮询的基础 Profile，这一设计能够进一步支撑 PTY、SOCKS 与反向端口转发等需要持续双向流量的功能。

表 5-2 `masked-https` 关键路由与处理逻辑
| 路由 | 通道类型 | 主要处理内容 |
|---|---|---|
| `/cdn-cgi/submit` | HTTPS Beacon | 处理 checkin、拉取任务、提交执行结果 |
| `/cdn-cgi/ws` | WSS Session | 建立 WebSocket 长连接并对接 Push C2 stream |
| 非匹配路径 | Decoy | 返回伪装响应，避免暴露单一路由用途 |

从代码结构上看，服务端并没有直接承担任务解释与业务决策，而是主要完成“接收、解码、转发、回写”四步。这种边界划分有两个好处：一是通信层逻辑足够集中，便于独立调试；二是当 Agent 后续新增命令时，Profile 只需要继续做协议搬运，不必同步增加命令处理分支。

#### 5.2.3 流量转发与解密实现
流量转发模块的关键在于完成 Mythic envelope 与本地 JSON 结构之间的映射。Agent 侧先将任务请求或回传结果序列化为 JSON，再按 `UUID + EncBlob` 结构封装；若启用了 AESPSK，则进一步执行 AES-256-CBC 加密并附加 HMAC 校验值。Profile 收到请求后先做 Base64 解码，再依据相同规则执行解密、验签和字段解析[46-48]。

当前工程中，Agent 侧的 `marshalOutboundEnvelope`、`decodeInboundEnvelope` 和 `aes256Encrypt/aes256Decrypt` 共同完成消息封装；Profile 侧则在收到请求后完成对应逆过程，并把结果转交给 Mythic。该设计保证了 Beacon 与 Session 两类链路在消息格式上的一致性，减少了不同通道之间的实现分叉。

表 5-3 报文处理主要步骤
| 阶段 | Agent 侧处理 | Profile 侧处理 |
|---|---|---|
| 序列化 | 将请求或结果编码为 JSON | 还原 JSON 结构并分发给 Mythic |
| 封装 | 按 `UUID + EncBlob` 组合 envelope | 解析 UUID 并分离密文主体 |
| 加解密 | AES-256-CBC + HMAC | 按相同密钥做验签与解密 |
| 传输 | 通过 HTTPS/WSS 发送 | 经 gRPC/消息接口转发给 Mythic |

需要说明的是，本文将“基础加密保护”和“外观层伪装”作为两个相互独立的目标：前者用于保证报文主体不以明文形式出现，后者用于让链路在形式上更接近常规 Web 流量。对毕业设计原型而言，这种分层已经足以支撑通信链路分析和后续扩展。

### 5.3 远控代理（Agent）核心实现
#### 5.3.1 主循环（Main Loop）与心跳机制代码实现
Agent 主循环由 `AgentController` 驱动，其典型执行路径为：启动后先执行 checkin，上线成功后保存 Mythic 返回的 callback UUID，随后进入 `get-tasking -> executeTask -> flushTaskResponses -> sleep` 的循环。对于 Beacon callback，轮询间隔主要由 sleep 与 jitter 控制；对于 Session callback，当存在交互会话、SOCKS 或 rpfwd 流量时，控制器会切换到更短的 active wait，以提升实时性。

在实现细节上，`checkin()` 负责收集主机名、用户名、IP、体系结构、进程名和完整性级别等信息，并用 payload UUID 发起初始注册；注册成功后，控制器再切换为使用 Mythic 返回的 callback UUID 持续通信。这样既符合 Mythic 的 checkin 流程要求，也保证了后续任务与结果能够准确归属到对应 callback[41][43]。

与传统“固定间隔 + 单线程处理”的实现相比，当前主循环在任务执行与异步结果缓冲之间做了分层，能够在保持轮询稳定的同时处理文件回传、交互式输出和代理数据等多类消息。这也是系统能够同时支撑 Beacon 与 Session 的关键。

#### 5.3.2 任务接收与JSON解析实现
任务接收模块以 JSON 为统一交换格式。Profile 返回的数据首先在 Agent 侧被还原为 `MythicMessageResponse`，随后经过任务合法性过滤，交由命令分发层执行。各命令统一输出 `TaskResponse`，再由结果缓冲区分类合并为普通响应、交互式消息、SOCKS 数据或 RPFWD 数据[46]。

在工程实现上，系统没有把所有消息都压成同一种“字符串结果”，而是区分了普通任务响应、interactive 消息、SOCKS 数据块和 rpfwd 数据块四类对象。这样做的直接收益是：PTY 输出不需要伪装成普通任务结果，SOCKS 与反向端口转发也可以按数据流方式回传，而不会与常规命令输出混淆。

表 5-4 异步结果缓冲区中的消息类型
| 消息类型 | 典型来源 | 主要用途 |
|---|---|---|
| `responses` | shell、sysinfo、ps 等 | 回传普通任务输出 |
| `interactive` | pty / 交互式 shell | 回传 stdout、stderr、exit 等流式消息 |
| `socks` | socks manager | 在 Agent 与 Mythic 之间转发代理流量 |
| `rpfwd` | rpfwd manager | 转发本地监听端口与远端连接数据 |

这种实现方式的优点在于：一是便于直接对照 Mythic 的任务结构进行调试；二是降低了新增命令时对通信层的影响；三是可以在后续继续扩展更复杂的消息类型，而不必重写主循环。对毕业设计项目而言，这种分层已经能够较清晰地体现任务解析、分发和回传的完整链路。

#### 5.3.3 系统API调用实现
系统 API 调用模块围绕实际已接入的命令展开。基础信息采集由 `sysinfo` 汇总主机名、用户名、网络地址、系统类型和体系结构；`ps`、`avscan`、`report` 等命令分别用于进程枚举、安全产品识别和结构化结果上报；`privesc-check` 与 `privesc-tool` 用于辅助提权探测和调用本地工具；`session-start` 负责从 Beacon callback 派生 Session callback，再由 `pty`、`socks` 与 `rpfwd` 等命令承载后续实时能力。

在文件管理方面，系统已完成与 Mythic File Browser 模型的对接，能够回传目录项、文件大小、修改时间和类型信息，支持 upload、download、remove 等基本操作。结合前端界面，操作员可直接在 Mythic 中浏览节点文件、执行上传下载并查看 sysinfo 输出。

从代码实现看，交互式会话模块依赖 `github.com/creack/pty` 创建 PTY，并通过独立 reader 协程持续采集输出；SOCKS 模块负责在本地代理端口与 Mythic 之间转发数据；RPFWD 模块则在 Agent 侧维护监听器、连接表和连接生命周期。这三个模块虽然面向的流量形态不同，但都共享“异步缓存 -> Push/轮询回传”的总框架，因此系统在扩展新的实时能力时不需要推翻既有通信结构。

表 5-5 主要命令与实现模块对应关系
| 命令或能力 | 主要实现模块 | 功能说明 |
|---|---|---|
| `sysinfo` / `report` | `collection.go` 等 | 主机信息采集与结构化结果上报 |
| `shell` | `task_execution.go` | 执行系统命令并返回输出 |
| `upload` / `download` / `remove` | `task_execution.go` | 文件上传、下载和删除 |
| `session-start` / `session-stop` | `session_supervisor.go` | Session callback 生命周期管理 |
| `pty` | `interactive_sessions.go` | 启动交互式 shell 或 PTY |
| `socks` | `socks_manager.go` | 启动本地代理通道 |
| `rpfwd` | `rpfwd_manager.go` | 维护反向端口转发监听与连接 |

[此处插入截图：图 5.4 文件管理功能界面]
图 5.4 文件管理功能界面

[此处插入截图：图 5.5 sysinfo 结果展示]
图 5.5 sysinfo 结果展示

[此处插入截图：图 5.6 Session 控制台中的命令列表与 SOCKS 启动结果]
图 5.6 Session 控制台中的命令列表与 SOCKS 启动结果

[此处插入截图：图 5.7 Session 与 Proxy 相关界面展示]
图 5.7 Session 与 Proxy 相关界面展示

### 5.4 本章小结
本章从开发环境、Profile 适配和 Agent 实现三个层面介绍了系统的关键实现过程。与仅展示界面效果的项目不同，本文明确给出了 Payload 构建模块、HTTPS/WSS 双通道、Push C2 桥接、主循环调度、异步结果缓冲和 PTY/SOCKS/RPFWD 等核心实现路径，使“基于 Mythic 框架”的题目要求能够落到具体代码结构与运行机制上。整体来看，系统已经形成较完整的控制端、通信层与执行层协同实现，为后续测试和结论分析提供了直接依据。

## 6. 系统测评
### 6.1 测试目的与环境
#### 6.1.1 测试目的
系统测试的目的是验证本系统的功能完整性、跨平台兼容性、通信隐蔽性和运行稳定性[8]。通过全面的测试，确保系统能够满足渗透测试的实际需求，为后续的实际应用提供可靠保障。

#### 6.1.2 测试环境
本课题的测试以本地 Mythic 联调环境为核心，重点验证自定义 Payload Type、C2 Profile 与 Agent 三者之间的端到端协同效果。相较于只做静态代码编译检查，本文更强调“真实回连、真实任务、真实界面回显”的验证方式。实际测试环境如表 6-1 所示。

表 6-1 主要测试环境
| 组件 | 说明 |
|---|---|
| Mythic UI | `https://127.0.0.1:7443` |
| 自定义 C2 Profile | `masked-https` |
| Beacon 地址 | `https://127.0.0.1:8443/cdn-cgi/submit` |
| Session 地址 | `wss://127.0.0.1:8443/cdn-cgi/ws` |
| macOS 样本 | `/tmp/my-agent_macos_full_e2e.bin` |
| Linux 样本 | `/root/my-agent_linux_full_e2e.bin` |
| 主要验证功能 | checkin、sysinfo、shell、upload/download、session-start、pty、socks、session-stop 等 |

测试过程中，浏览器侧已完成 Mythic 登录、启动 `masked-https` 服务、构建 Payload 以及下载样本等操作；后续命令验证则结合 Mythic 任务接口与 Web 前端回显结果进行核对，从而保证测试证据具有较好的可追踪性。

### 6.2 通信协议测试
#### 6.2.1 连通性测试
连通性测试重点验证 Beacon 与 Session 两条链路是否能够在 Mythic 环境中形成完整闭环。实际联调过程中，操作员先在 Mythic Web 界面启动 `masked-https` 服务，再分别构建 macOS 与 Linux 样本。样本运行后，两类平台都能够完成初始 checkin，并在 Mythic 中生成新的 callback 记录；其中 macOS Beacon callback 还可进一步通过 `session-start` 派生新的 Session callback。

从任务执行结果看，checkin、任务拉取、结果回传和 Session 派生四个关键环节均已被真实触发。结合界面回显和日志输出，可以确认 `/cdn-cgi/submit` 与 `/cdn-cgi/ws` 两条路径在当前实验环境下均处于可用状态。需要说明的是，本文没有单独搭建丢包仿真环境对链路进行压力测试，因此不对“指令丢失率”“抖动下吞吐量”等指标给出额外结论。

#### 6.2.2 流量伪装效果测试
流量伪装效果测试主要从“配置能力”和“协议外观”两个层面进行验证。当前 Profile 支持自定义 Host、User-Agent、附加 Header、Beacon 路径、Session 路径和 TLS 证书参数；Beacon 默认使用 `/cdn-cgi/submit`，Session 默认使用 `/cdn-cgi/ws`，对于无效路径会返回 decoy 响应。

结合抓包与请求内容观察可以看到，链路外观已接近常规 HTTPS/WSS 请求，报文主体则以加密后的 Base64 数据形式承载。换言之，系统已经实现了基础的 Web 协议型流量伪装能力[49]。但由于本文没有引入企业级 WAF、IDS 或代理审计设备开展对照测试，因此只能得出“具备基础伪装能力”的结论，而不宜进一步宣称其对实际防御设备具有确定规避效果。

### 6.3 远控代理功能测试
#### 6.3.1 基础指令执行测试
基础命令测试主要围绕已接入的核心命令展开，包括 `sysinfo`、`shell`、`ps`、`privesc-check`、`privesc-tool`、`report` 以及 Session 生命周期控制命令。测试重点放在“控制端下发任务—Agent 本地执行—结果回传 Mythic 前端”这一完整链路。实际结果如表 6-2 所示。

表 6-2 基础命令执行结果
| 平台 | 命令 | 结果 | 说明 |
|---|---|---|---|
| macOS | `sysinfo` | 通过 | 返回主机基础系统信息 |
| macOS | `shell whoami` | 通过 | 返回当前用户信息 |
| macOS | `privesc-check` | 通过 | 权限辅助探测逻辑可正常返回 |
| macOS | `privesc-tool` | 通过 | 可完成本地辅助命令调用 |
| macOS | `report` | 通过 | 可结构化登记 artifact/credential |
| Linux | `sysinfo` | 通过 | 返回主机基础系统信息 |
| Linux | `ps` | 通过 | 进程列表可正常回传 |
| Linux | `shell whoami` | 通过 | 返回当前用户信息 |
| Linux | `privesc-check` | 通过 | 基础探测逻辑可用 |
| Linux | `privesc-tool` | 通过 | 可完成本地辅助命令调用 |

从结果看，基础命令主链路已经打通，命令分发、执行和回传过程整体稳定，能够满足毕业设计阶段对远控代理基础任务执行能力的验证要求。

#### 6.3.2 文件管理功能测试
文件管理测试围绕 `upload`、`download`、`remove` 和 File Browser 展开，重点验证文件链路是否能够在 Mythic 前端与 Agent 本地之间形成闭环。实际测试结果如表 6-3 所示。

表 6-3 文件管理测试结果
| 平台 | 功能 | 结果 | 说明 |
|---|---|---|---|
| macOS | `upload` | 通过 | 文件可写入目标目录 |
| macOS | `download` | 通过 | 文件可被 Mythic 回收 |
| macOS | `remove` | 通过 | 目标文件删除正常 |
| macOS | 目录浏览 | 通过 | 可回显文件名、大小与时间 |
| Linux | 目录浏览 | 通过 | 可返回目标目录项信息 |
| Linux | `remove` | 通过 | 删除逻辑正常 |

测试结果表明，系统能够完成文件浏览、上传、下载和删除等基础文件管理操作，File Browser 前端显示结果与 Agent 端执行结果能够形成对应关系，说明文件管理模块与 Mythic 平台的数据模型适配有效。

#### 6.3.3 长时间运行稳定性测试
稳定性测试主要关注多轮上线、反复任务执行、Session 创建/停止、SOCKS 启停与文件传输后的整体状态观察。当前联调结果表明，Agent 在多轮 checkin、任务轮询和 Session 派生过程中运行稳定，未出现影响主流程的整体崩溃；退出命令能够触发资源回收，Session 相关状态也能够在 Mythic 前端中得到同步更新。

综合观察结果可以看出，系统在毕业设计测试范围内能够保持稳定运行，HTTPS Beacon 与 WSS Session 两类链路均可承载对应任务，满足论文中对远控代理连续运行和任务调度能力的验证要求。

## 6.4 跨平台运行测试
#### 6.4.1 Windows环境测试结果
Windows 平台测试主要围绕目标样本构建、基础命令接口适配和跨平台编译链路展开。结合 `builder.go` 中的 GOOS/GOARCH 处理逻辑以及命令实现的跨平台分支设计，系统能够生成 Windows x86-64 目标样本。本文在本地环境中执行 `GOOS=windows GOARCH=amd64 go build` 后，成功生成 PE32+ x86-64 控制台程序，说明 Agent 代码在 Windows 目标平台上的构建链路已经通过验证。

在功能设计上，Windows 平台复用了任务接收、JSON 解析、结果回传和文件管理等通用逻辑，平台相关差异主要由系统 API 封装层处理。由此可见，系统已具备 Windows 平台部署和运行的基础条件，满足毕业设计中关于跨平台构建与兼容性适配的测试要求。

#### 6.4.2 Linux/macOS环境测试结果
Linux 与 macOS 是本次论文功能联调的主要运行平台。Linux 平台完成了 checkin、sysinfo、ps、shell、privesc-check、privesc-tool、session-start、session-status、pty 和 session-stop 等验证；macOS 平台完成了 checkin、sysinfo、shell、upload、download、remove、report、socks、socks-stop、exit 等任务验证。

总体来看，系统在 Linux 与 macOS 平台上的运行结果能够证明其跨平台设计具备可用性。两类平台均可完成 Agent 上线、任务接收、命令执行与结果回传；文件管理、Session 派生和代理相关功能也能够在测试环境中完成验收，说明系统的核心执行框架和通信框架具有较好的平台适配能力。

## 6.5 测试结果分析
综合第 6 章各项测试结果，可以将当前系统状态概括为“主链路完整、核心功能可用、跨平台适配有效”。从控制链路看，自定义 `masked-https` Profile 能够承载 HTTPS Beacon 与 WSS Session 两类流量；从功能层看，命令执行、文件管理、基础信息收集、Session 派生、PTY 与代理相关能力均已完成验收；从平台层看，Linux、macOS 的运行测试和 Windows 目标样本构建验证共同支撑了论文中关于跨平台设计的主要论点。

测试结果表明，本系统能够在 Mythic 框架下完成自定义 C2 Profile、自定义 Payload Type 与 Agent 端功能模块之间的协同工作，整体实现效果符合需求分析阶段提出的功能目标和非功能目标。对于本科毕业设计而言，该系统已经具备较完整的架构层次、较清晰的工程实现和可复现的测试证据，能够支撑后续答辩展示与进一步扩展研究。

## 6.6 本章小结
本章对基于 Mythic 框架的渗透测试远控系统进行了完整测试。首先介绍了测试目的和测试环境，然后进行了通信协议测试、远控代理功能测试和跨平台运行测试，最后对测试结果进行了分析。综合测试结果可以认为，本系统具备较好的功能完整性、跨平台适配能力和通信可用性，能够满足毕业设计阶段的系统验收需求。

## 7. 结论与展望
### 7.1 结论
本文围绕“基于Mythic框架的渗透测试远控系统设计与实现”这一主题，完成了一套以 Mythic 为控制核心、以 `masked-https` 为通信适配层、以 `my-agent` 为执行主体的远控系统设计与实现。相较于单独编写一个能回连的 Agent，本课题更关注控制端、通信层与执行层三者之间的协同关系，因此在实现中同时覆盖了 Payload Type、C2 Profile 和 Agent 三个部分。

本文的主要工作可以概括为以下五点。第一，围绕授权测试场景，对远控系统在 callback 上线、任务调度、文件管理、系统信息采集、Session 派生和代理转发等方面的需求进行了分析。第二，基于 Mythic 的模块化机制，设计了由 HTTPS Beacon 与 WSS Session 组成的双通道通信结构，并给出了相应的消息组织与加密封装方案。第三，完成了自定义 Payload Type 与自定义 C2 Profile 的接入，实现了命令注册、构建参数写入、服务端监听、Push C2 桥接和 decoy 响应等功能。第四，完成了 Agent 主循环、异步任务执行、文件浏览器适配、PTY、SOCKS 与反向端口转发等模块开发。第五，完成了 macOS、Linux 平台功能联调以及 Windows 目标样本构建验证，验证了 checkin、命令执行、文件传输、Session 派生和代理相关能力的可用性。

从毕业设计成果角度看，本文实现的系统具备较完整的架构层次、较清晰的代码组织方式和较充分的测试证据，能够体现“基于 Mythic 框架”与“渗透测试远控系统设计与实现”这两个题目核心点。系统测试结果表明，自定义 C2 Profile、自定义 Payload Type 与 Agent 端功能模块能够协同工作，整体达到了需求分析阶段提出的设计目标。

## 7.2 展望
本系统已经完成毕业设计阶段所需的核心链路。后续若继续扩展，可从以下方向进一步提升系统能力。

第一，扩展 Session 通道的应用场景。现有系统已经支持 PTY、SOCKS 与反向端口转发等交互式能力，后续可围绕更复杂的多连接场景和更细粒度的连接状态展示继续增强。

第二，扩展文件管理模块的工程化能力。现有系统已经支持文件浏览、上传、下载和删除等基本操作，后续可继续加入更完整的批量操作、断点续传和传输进度展示，以提升大文件和多文件场景下的使用体验。

第三，丰富 Windows 平台适配能力。当前系统已经完成 Windows 目标样本构建与兼容性适配验证，后续可在更多 Windows 版本上补充运行场景，进一步完善进程枚举、文件操作和 Session 派生等功能的细节表现。

第四，继续提高通信层的研究价值。本文已经实现了基于 HTTP/S 与 WSS 的双通道结构，后续可继续引入更丰富的 Header 模板、路径模板和协议变体，并结合靶场审计环境开展更系统的通信特征分析实验。

第五，完善工程交付形态。后续可继续补充自动化测试、运行态指标统计和部署脚本，使系统在教学演示、授权测试和二次开发场景中具备更好的复用性。

## 参考文献
[1] Gardiner J, Cova M, Nagaraja S. Command & Control: Understanding,Denying and Detecting-A review of malware C2 techniques, detection and defences[J]. arXiv preprint arXiv:1408.1136, 2014.

[2] 傅钰. 网络安全等级保护2.0下的安全体系建设[J].网络安全技术与应用, 2018(10): 8-11.

[3] Tonguino J K, Rahman N A A, Al-Rashdan M T. ZeroTrace—A Tailored Command-and-Control (C2) Server for Penetration Testing[C]//International Conference on Computer Science and Information Technology. Springer, 2024: 687-702.

[4] MITRE Corporation. MITRE ATT&CK Framework[EB/OL].(2024-12-01)[2026-04-08]. https://attack.mitre.org/.

[5] Schmidt L. Enhancing Command & Control Capabilities: Integrating Cobalt Strike's Plugin System into a Mythic-based Beacon Developed at cirosec[D]. Offenburg: Hochschule Offenburg, 2025.

[6] Chatzoglou E, Kambourakis G. C3: Leveraging the Native Messaging Application Programming Interface for Covert Command and Control[J].Future Internet, 2025, 17(4): 172.

[7] Rao S S, Mishra S, Seshadri S. Post-Exploitation Insights: Dynamic Analysis of C2 Frameworks[C]//2024 4th International Conference on Intelligent Technologies (CONIT). IEEE, 2024: 1-6.

[8] Cyber Centaurs. Cobalt Strike - The C2 Framework[EB/OL].(2025-08-24)[2026-04-08].https://cybercentaurs.com/topics/cobalt-strike-the-c2-framework/.

[9] González Castaño M. LONS-C2: Desarrollo de una herramienta de Command and Control para pentesting[D]. Barcelona: Universitat Autònoma de Barcelona, 2023.

[10] Eisenberg D A, Alderson D L, Kitsak M, et al. Network foundation for command and control (C2) systems: Literature review[J]. IEEE Access, 2018, 6: 70345-70365.

[11] Heinz C, Mazurczyk W, Caviglione L. Covert channels in transport layer security[C]//ACM Workshop on Information Hiding and Multimedia Security. 2020: 83-94.

[12] Heinz C, Zuppelli M, Caviglione L. Covert Channels in Transport Layer Security: Performance and Security Assessment[J]. Journal of Wireless Mobile Networks, Ubiquitous Computing, and Dependable Applications, 2021, 12(3): 45-58.

[13] Husák M, Čermák M, Jirsík T, et al. HTTPS traffic analysis and client identification using passive SSL/TLS fingerprinting[J]. EAI Endorsed Transactions on Information Security, 2016, 2(1): e1.

[14] Ahmed A. Privilege Escalation Techniques: Learn the art of exploiting Windows and Linux systems[M]. Birmingham: Packt Publishing,2021.

[15] O'Leary M. Privilege Escalation in Linux[M]//Cyber Operations:Building, Defending, and Attacking Modern Computer Networks. Berkeley:Apress, 2019: 243-268.

[16] Qiang W, Yang J, Jin H, et al. Privguard: Protecting sensitive kernel data from privilege escalation attacks[J]. IEEE Access, 2018,6: 46924-46937.

[17] Provos N, Friedl M, Honeyman P. Preventing privilege escalation[C]//12th USENIX Security Symposium. 2003: 231-242.

[18] Zhang Y, Chen J, Chen K, et al. Network traffic identification of several open source secure proxy protocols[J]. International Journal of Network Management, 2021, 31(6): e2090.

[19] Smith D, Miller M, Saint-Andre P, et al. XEP-0065: SOCKS5 Bytestreams[S]. XMPP Standards Foundation, 2004.

[20] Donovan A A A, Kernighan B W. The Go Programming Language[M].New York: Addison-Wesley Professional, 2015.

[21] Togashi N, Klyuev V. Concurrency in Go and Java: performance analysis[C]//2014 4th IEEE International Conference on Information Science and Technology. IEEE, 2014: 369-372.

[22] Sultan S, Ahmad I, Dimitriou T. Container security: Issues,challenges, and the road ahead[J]. IEEE Access, 2019, 7: 52976-52996.

[23] Brady K, Moon S, Nguyen T, et al. Docker container security in cloud computing[C]//2020 10th Annual Computing and Communication Workshop and Conference (CCWC). IEEE, 2020: 0109-0114.

[24] Yarygina T, Bagge A H. Overcoming security challenges in microservice architectures[C]//2018 IEEE Symposium on Service-Oriented System Engineering (SOSE). IEEE, 2018: 11-20.

[25] Mahajan P. A study of encryption algorithms AES, DES and RSA for security[J]. Global Journal of Computer Science and Technology, 2013,13(15): 1-6.

[26] D'souza F J, Panchal D. Advanced encryption standard (AES)security enhancement using hybrid approach[C]//2017 International Conference on Computing, Communication and Automation (ICCCA). IEEE,2017: 416-421.

[27] 刘奇旭, 王君楠, 尹捷, 等.对抗机器学习在网络入侵检测领域的应用[J]. 通信学报, 2024, 45(1): 1-18.

[28] 杨宇, 闫钰, 申芳, 等. 基于机器和深度学习的入侵检测综述[J].科学技术与工程, 2023, 23(9): 3618-3630.

[29] 张昊, 张小雨, 张振友, 等. 基于深度学习的入侵检测模型综述[J].计算机工程与应用, 2022, 58(15): 1-14.

[30] 颜星晨, 张鑫, 周志洪, 等.基于等保2.0验证测试与ATT&CK攻击矩阵的融合实践[J]. 网络安全与数据治理,2025, 44(9): 15-23.

[31] 网络安全知识图谱构建与应用研究综述[J]. 网络空间安全科学学报,2024, 4(3): 1-15.

[32] 周勇, 陈玺名, 程度, 等.基于服务器主动安全的自动化红队测试技术研究[J]. 微电子学与计算机, 2024,41(12): 1-8.

[33] 刘刚, 杨轶杰. 基于等级保护2.0的铁路网络安全技术防护体系研究[J].铁路计算机应用, 2020, 29(8): 1-6.

[34] 朱岩, 张艺, 王迪, 等. 网络安全等级保护下的区块链评估方法[J].工程科学学报, 2020, 42(12): 1543-1553.

[35] 王振东, 张林, 李大海. 基于机器学习的物联网入侵检测系统综述[J].计算机工程与应用, 2021, 57(15): 1-12.

[36] 李俊, 夏松竹, 兰海燕, 等. 基于GRU-RNN的网络入侵检测方法[J].哈尔滨工程大学学报, 2021, 42(5): 601-608.

[37] Mehmood M, Amin R, Muslam M M A, et al. Privilege escalation attack detection and mitigation in cloud using machine learning[J].IEEE Access, 2023, 11: 48938-48953.

[38] Combe T, Martin A, Di Pietro R. To docker or not to docker: A security perspective[J]. IEEE Cloud Computing, 2016, 3(5): 54-62.

[39] 张茜, 王晓菲, 王亚洲, 等.基于机器学习的网络入侵检测技术综述[J]. 网络安全与数据治理, 2024,43(5): 1-10.

[40] Happe A, Kaplan A, Cito J. Llms as hackers: Autonomous linux privilege escalation attacks[J]. Empirical Software Engineering, 2026,31(2): 1-37.

[41] Mythic Documentation. C2 Profiles Overview[EB/OL]. [2026-04-12]. https://docs.mythic-c2.net/operational-pieces/c2-profiles.

[42] Mythic Documentation. Payload Type Definition[EB/OL]. [2026-04-12]. https://docs.mythic-c2.net/customizing/payload-type-development/payload-type-info.

[43] its-a-feature. Mythic: A collaborative, multi-platform, red teaming framework[EB/OL]. [2026-04-12]. https://github.com/its-a-feature/Mythic.

[44] The Go Project. Go Wiki: Building Windows Go programs on Linux[EB/OL]. [2026-04-12]. https://go.dev/wiki/WindowsCrossCompiling.

[45] Docker Inc. Docker Docs: Getting Started with Docker[EB/OL]. [2026-04-12]. https://docs.docker.com/guides/lab-container-getting-started/.

[46] Bray T. RFC 8259: The JavaScript Object Notation (JSON) Data Interchange Format[S/OL]. 2017[2026-04-12]. https://www.rfc-editor.org/rfc/rfc8259.

[47] National Institute of Standards and Technology. Advanced Encryption Standard (AES)[S/OL]. 2023[2026-04-12]. https://doi.org/10.6028/NIST.FIPS.197-upd1.

[48] Rescorla E. RFC 8446: The Transport Layer Security (TLS) Protocol Version 1.3[S/OL]. 2018[2026-04-12]. https://www.rfc-editor.org/rfc/rfc8446.html.

[49] MITRE ATT&CK. T1071.001 Application Layer Protocol: Web Protocols[EB/OL]. [2026-04-12]. https://attack.mitre.org/techniques/T1071/001.

## 符号说明
C2：Command and Control，命令与控制

RAT：Remote Access Trojan，远程访问木马

Agent：植入体，运行在目标系统上的远控程序

Profile：通信配置文件，定义C2通信协议

Beacon：心跳，Agent定期向控制端发送的通信请求

Jitter：抖动，心跳间隔的随机变化比例

SOCKS：Socket Secure，一种代理协议

AES：Advanced Encryption Standard，高级加密标准

HTTP：HyperText Transfer Protocol，超文本传输协议

HTTPS：HTTP Secure，安全的超文本传输协议

WMI：Windows Management Instrumentation，Windows管理规范

UAC：User Account Control，用户账户控制

DPI：Deep Packet Inspection，深度包检测

TTP：Tactics, Techniques, and Procedures，战术、技术和程序

BOF：Beacon Object File，Beacon对象文件

IV：Initialization Vector，初始化向量

NAT：Network Address Translation，网络地址转换

EDR：Endpoint Detection and Response，端点检测与响应

XDR：Extended Detection and Response，扩展检测与响应

gRPC：Google Remote Procedure Call，Google远程过程调用

## 附录
### 附录A 核心代码片段
#### A.1 Agent主控循环代码
以下代码节选自 `controller.go`，展示了 Agent 完成 checkin 后进入轮询、任务执行与结果回传的主控逻辑：

```go
if err := c.checkin(); err != nil {
    log.Printf("[%s] checkin 失败: %v\n", c.name, err)
    time.Sleep(c.currentPollInterval())
    return
}

for {
    select {
    case <-c.stopCh:
        return
    default:
    }

    tasks, err := c.getTasking()
    if err != nil {
        log.Printf("[%s] 获取任务失败: %v\n", c.name, err)
        time.Sleep(c.currentPollInterval())
        continue
    }

    if len(tasks) > 0 {
        responses := make([]TaskResponse, 0, len(tasks))
        for _, task := range tasks {
            resp := c.executeTask(task)
            responses = append(responses, resp)
        }
        if err := c.flushTaskResponses(responses); err != nil {
            log.Printf("[%s] 提交响应失败: %v\n", c.name, err)
        }
    }
    time.Sleep(c.currentPollInterval())
}
```

#### A.2 AES加密模块代码
以下代码节选自 `agent_utils.go`，展示了 Agent 在有 AESPSK 配置时对报文主体执行加密、验签与解密的核心逻辑：

```go
func aes256Encrypt(plaintext []byte, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    plaintext = pkcs7Pad(plaintext, aes.BlockSize)
    iv := make([]byte, aes.BlockSize)
    if _, err := io.ReadFull(rand.Reader, iv); err != nil {
        return nil, err
    }
    mode := cipher.NewCBCEncrypter(block, iv)
    ciphertext := make([]byte, len(plaintext))
    mode.CryptBlocks(ciphertext, plaintext)
    h := hmac.New(sha256.New, key)
    h.Write(iv)
    h.Write(ciphertext)
    mac := h.Sum(nil)
    result := append(iv, ciphertext...)
    result = append(result, mac...)
    return result, nil
}

func aes256Decrypt(data []byte, key []byte) ([]byte, error) {
    if len(data) < aes.BlockSize+sha256.Size {
        return nil, fmt.Errorf("数据太短")
    }
    iv := data[:aes.BlockSize]
    mac := data[len(data)-sha256.Size:]
    ciphertext := data[aes.BlockSize : len(data)-sha256.Size]
    h := hmac.New(sha256.New, key)
    h.Write(iv)
    h.Write(ciphertext)
    if !hmac.Equal(mac, h.Sum(nil)) {
        return nil, fmt.Errorf("HMAC 验证失败")
    }
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    mode := cipher.NewCBCDecrypter(block, iv)
    plaintext := make([]byte, len(ciphertext))
    mode.CryptBlocks(plaintext, ciphertext)
    return pkcs7Unpad(plaintext)
}
```

## 致  谢
本论文完成之际，我首先感谢指导教师彭许红老师。自选题、开题到论文撰写和系统联调，彭老师都给予了持续的指导。无论是论文结构、技术路线，还是实验内容取舍，老师都提出了明确而细致的修改意见，使我能够逐步把项目从一个可运行的原型整理为较完整的毕业设计成果。

感谢计算机学院及网络空间安全专业的各位老师。四年的课程学习为我完成本课题提供了必要的理论基础和实践能力，也让我对渗透测试、系统实现和工程规范之间的关系有了更具体的认识。

感谢在毕业设计期间给予我帮助的同学和朋友。项目调试、环境搭建和问题排查过程中，大家提供了不少讨论和建议，这些交流对我完善系统功能和整理测试结果都很有帮助。

最后，感谢家人一直以来的理解与支持。正是这些稳定的支持，使我能够较为专注地完成本次毕业设计。

感谢各位老师在论文评阅和答辩过程中给予指导。

朱家逸

2026年4月

