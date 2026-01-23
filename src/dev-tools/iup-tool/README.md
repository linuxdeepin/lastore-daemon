# iup-tool

内网更新平台（Intranet Update Platform，简称 **IUP**）的命令行调试/联调工具。

该工具封装了与 IUP 的 HTTP 接口交互、日志/状态上报、升级结果上报等能力，主要用于：

- 快速校验当前机器是否能正常访问更新平台
- 查询版本、更新策略与更新日志
- 查询当前基线的软件包清单
- 查询 CVE 漏洞信息
- 在升级流程中向 IUP 上报过程状态、过程事件和最终结果

> 本工具定位为 **开发/测试/运维联调辅助工具**，并不面向普通用户。

---

## 构建与运行环境

### 依赖条件

- 执行 `apt-config dump`命令，输出配置中包含`Acquire::SmartMirrors::Token`，例如：
- `Acquire::SmartMirrors::Token "a=Deepin;b=Desktop;...";`
- 本机可访问内网更新平台。

### 从源码构建

在仓库根目录下执行：

```bash
make bin/iup-tool
```

---

## 全局用法

```bash
./iup-tool [全局参数] <子命令> [子命令参数]
```

### 全局参数

- `--timeout <秒>`：HTTP 请求超时时间，默认 **40** 秒。
- `--debug`：启用调试日志，打印更详细的请求/响应信息（包括请求头等）。

示例：

```bash
./iup-tool --debug get_version
```

---

## 子命令总览

- **版本与策略查询相关**
  - `get_version`：获取版本信息与更新策略
  - `get_update_log`：获取指定基线的系统更新日志
  - `get_current_packages`：获取指定基线的预装软件包和脚本清单
  - `get_cve_info`：获取 CVE 漏洞信息
- **过程状态与结果上报相关**
  - `post_process`：上报过程状态消息，或上传压缩后的日志文件
  - `post_process_event`：上报升级过程事件
  - `post_result`：上报最终升级结果

下文分别介绍各子命令及使用示例。

---

## get_version —— 获取版本与更新策略

对应源码：`cmd_get_version.go`

### 功能

向 IUP 发送 `GET /api/v1/version` 请求，携带：

- `X-Repo-Token`：从 `apt-config` 中读取的 Token（Base64 编码后放入头部）；
- `X-Packages`：客户端包信息（比如：`client=lastore-daemon&version=6.2.45`的 base64 编码）。

平台返回的数据会解析为内部的 `updateMessage` 结构，并在日志中以较详细的形式打印。

### 使用示例

```bash
./iup-tool --debug get_version
```

常用场景：联调 IUP 版本策略接口、检查当前机器是否能正常获取更新策略。

---

## get_update_log —— 获取更新日志

对应源码：`cmd_get_update_log.go`

### 功能

向 IUP 发送 `GET /api/v1/systemupdatelogs` 请求，根据基线号和是否为不稳定版本获取更新日志列表。

### 参数

- `-b, --baseline <baseline>`：**必选**，目标基线号。
- `-u, --is-unstable <1|2>`：是否为不稳定版本：
  - `1`：发行版（release）
  - `2`：不稳定版（unstable）

### 使用示例

```bash
./iup-tool --debug get_update_log --baseline pro-20-std-4-s-260105143125
```

可结合 `--debug` 查看详细响应内容，用于确认指定基线的更新日志是否正确下发。

---

## get_current_packages —— 获取当前基线的软件包清单

对应源码：`cmd_get_current_packages.go`

### 功能

向 IUP 发送 `GET /api/v1/package` 请求，带上当前基线号，获取该基线的预装软件包元数据。

### 参数

- `-b, --baseline <baseline>`：**必选**，当前基线号。

### 使用示例

```bash
./iup-tool --debug get_current_packages --baseline pro-20-std-4-s-260105143125
```

---

## get_cve_info —— 获取 CVE 漏洞信息

对应源码：`cmd_get_cve_info.go`

### 功能

向 IUP 发送 `GET /api/v1/cve/sync` 请求，获取 CVE 相关信息，解析为 `CVEMeta` 结构：

- `DataTime`：数据时间；
- `CVEs`：CVE ID → CVE 信息（描述、严重等级等）；
- `PkgCVEs`：包名 → 关联 CVE 列表。

命令会在日志中打印整体数量，并展示前若干条 CVE 的概要信息。

### 参数

- `-s, --sync-time <YYYY-MM-DD>`：
  - 可选；用于指定同步时间，通常用于增量同步/过滤。
  - 为空时由平台决定返回策略（一般可视为全量或最新）。

### 使用示例

```bash
./iup-tool --debug get_cve_info
```

---

## post_process —— 上报过程状态或日志

对应源码：`cmd_post_process.go`

该命令有两种典型用法：

1. **上报过程状态消息（JSON 或命令行参数构造）**；
2. **上传若干日志文件（打包 tar 再 xz 压缩后上报）**。

### 1. 上报过程状态消息

#### 参数

- `-m, --message-type <info|warning|error>`：消息类型，默认 `info`。
- `-u, --update-type <string>`：更新类型，自定义字符串，用于区分不同类别的更新。
- `-j, --job-desc <string>`：任务描述。
- `-d, --detail <string>`：详细描述内容。
- `-f, --data-file <path>`：可选，包含 `StatusMessage` JSON 的文件，字段格式：
  - `type`：`info` / `warning` / `error`
  - `updateType`
  - `jobDescription`
  - `detail`
- `-c, --current-baseline <baseline>`：当前基线，用于 HTTP 头 `X-CurrentBaseline`。
- `-t, --target-baseline <baseline>`：目标基线，用于 HTTP 头 `X-Baseline`。

如果指定 `-f/--data-file`，则直接使用文件中的 JSON；否则根据命令行参数构造 `StatusMessage` 并上报。

#### 使用示例

```bash
./iup-tool --debug post_process \
  -d 'msg detail' \
  -j 'jobupdate' \
  -m info \
  -u 'updateType' \
  -c pro-20-std-4-s-260105143125 \
  -t pro-20-std-5-s-260105143125
```

### 2. 上传日志文件

#### 参数

- `-l, --log-files <file1,file2,...>`：要上传的日志文件列表（逗号分隔）。
- `-c, --current-baseline <baseline>`：当前基线（可选，参与上报头部）。
- `-t, --target-baseline <baseline>`：目标基线（可选，参与上报头部）。

命令会：

1. 校验所有文件是否存在；
2. 打包为 tar 文件：`/tmp/update_logs_<时间>.tar`；
3. 调用 `genPostProcessResponse` 将 tar 再通过 `xz` 压缩后上传；
4. 在非 `--debug` 模式下自动删除中间生成的 tar/xz 文件。

#### 使用示例

```bash
./iup-tool --debug post_process \
  -l /var/log/apt/history.log \
  -c pro-20-std-4-s-260105143125 \
  -t pro-20-std-5-s-260105143125
```

---

## post_process_event —— 上报升级过程事件

对应源码：`cmd_post_process_event.go`

### 功能

上报升级过程中的关键事件（检查环境、获取更新、开始/完成下载、开始/完成备份、开始安装等）。

事件最终会被封装为 `ProcessEvent` 结构并通过 AES-CBC 加密、Base64 编码后发送到 `POST /api/v1/process/events`。

### 事件类型枚举

`ProcessEventType` 对应关系如下：

- `1`：`CheckEnv` —— 检查升级环境
- `2`：`GetUpdateEvent` —— 获取更新任务
- `3`：`StartDownload` —— 开始下载
- `4`：`DownloadComplete` —— 下载完成
- `5`：`StartBackUp` —— 开始备份
- `6`：`BackUpComplete` —— 备份完成
- `7`：`StartInstall` —— 开始安装

命令内部会校验事件类型是否在 `[1, 7]` 范围内，超出则退出并打印可用类型列表。

### 参数

- `-T, --task-id <int>`：任务 ID。
- `-e, --event-type <int>`：事件类型，见上表。
- `-s, --status <true|false>`：事件是否成功；
  - `true` 表示成功，`false` 表示失败。
- `-m, --message <string>`：事件内容/说明文本；命令会将长度裁剪到最多 950 个字符。
- `-c, --current-baseline <baseline>`：当前基线；
- `-t, --target-baseline <baseline>`：目标基线。
- `-f, --data-file <path>`：包含 `ProcessEvent` JSON 的文件，字段格式：
  - `taskID`
  - `eventType`
  - `eventStatus`
  - `eventContent`

如果提供 `-f/--data-file`，则优先从文件中读取事件对象；否则根据命令行参数构造。

### 使用示例

```bash
./iup-tool --debug post_process_event \
  -e 1 \
  -s true \
  -m 'process event content' \
  -c pro-20-std-4-s-260105143125 \
  -t pro-20-std-5-s-260105143125 \
  -T 7
```

---

## post_result —— 上报最终升级结果

对应源码：`cmd_post_result.go`

### 功能

在升级流程结束时，将最终结果上报给 IUP，对应接口 `POST /api/v1/update/status`。

命令会构造或读取 `UpgradePostMsg` 结构，使用 AES-CBC 加密并 Base64 编码后发送。

### 参数

- `-T, --task-id <int>`：任务 ID；
- `-s, --status <0|1|2>`：升级结果：
  - `0`：`UpgradeSucceed` 升级成功
  - `1`：`UpgradeFailed` 升级失败
  - `2`：`CheckFailed` 检查失败（未进入真正的升级过程）
- `-m, --message <string>`：失败时的错误信息。
- `-c, --current-baseline <baseline>`：当前（升级前）基线；
- `-t, --target-baseline <baseline>`：目标（升级后）基线。
- `-f, --data-file <path>`：包含 `UpgradePostMsg` JSON 的文件；若指定则所有信息都以文件为准。

当未提供 `-f/--data-file` 时，命令会自动填充部分字段，例如：

- `MachineID`：从 Token 中解析得到；
- `TimeStamp`、`UpgradeStartTime`、`UpgradeEndTime`：使用当前时间生成；
- `Uuid`：通过 `utils.GenUuid()` 生成一个新的 UUID；

### 使用示例

```bash
./iup-tool --debug post_result \
  -c pro-20-std-4-s-260105143125 \
  -t pro-20-std-5-s-260105143125 \
  -T 7 \
  -s 0
```

---

## 关于 Token 与 Machine ID

- 工具启动时会从 `apt-config` 中执行：

  ```bash
  apt-config dump Acquire::SmartMirrors::Token
  ```

- 解析形式类似：`Acquire::SmartMirrors::Token "a=xxx;b=yyy;i=<machineID>;...";`
- 其中 `i=<machineID>` 字段会被提取为 Machine ID，用于：
  - 上报时填充 `X-MachineID` 头；
  - 部分结果上报结构体中的 `MachineID` 字段。

---

## 调试建议

- **推荐总是加上 `--debug`**，这样可以看到：
  - 实际请求 URL 和请求方法；
  - 所有请求头（包括 `X-Repo-Token`、`X-MachineID`、基线信息等）；
  - 平台返回的原始 JSON 数据。
- 当命令失败时：
  - 先检查 Token 是否正确配置；
  - 再检查平台 URL（`platform-url`）是否可访问；
