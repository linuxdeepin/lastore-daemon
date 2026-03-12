## 更新环境、过程检查或修复结果：
| 事件名称 | 变量 | 公网 API | 内网 API |
|----|--- | ----| ----|
|检查更新前 | preUpdateCheck |   ✗ | /api/v1/process/events |
|检查更新后|postUpdateCheck  |  ✗ | /api/v1/process/events |
|下载更新前|preDownloadCheck |  ✗ | /api/v1/process/events |
|下载更新后|postDownloadCheck|  ✗ | /api/v1/process/events |
|备份前|preBackupCheck       |  ✗ | /api/v1/process/events |
|备份后|postBackupCheck       |  ✗ | /api/v1/process/events |
|系统升级前|preCheck          |  ✗ | /api/v1/process/events |
|系统升级后|midCheck          |   ✗ | /api/v1/process/events |
|系统升级完成重启后|postCheck  |   ✗ | /api/v1/process/events |

## 更新事件上报：
|描述 | 公网 API | 内网 API |
|----|---- | ----|
|检查到更新  | /api/v1/process | /api/v1/process/events |
|检查更新失败 | /api/v1/process | /api/v1/process/events |
|开始下载 |           ✗          | /api/v1/process/events |
|下载完成 | /api/v1/process | /api/v1/process/events |
|下载失败 | /api/v1/process | /api/v1/process/events |
|开始备份 |        ✗        | /api/v1/process/events |
|备份成功 | /api/v1/process | /api/v1/process/events |
|备份失败 |   ✗             | /api/v1/process/events |
|开始安装 | /api/v1/process | /api/v1/process/events |
|安装失败 | /api/v1/process | /api/v1/process/events |

> 上述结果只代表当前代码逻辑状态