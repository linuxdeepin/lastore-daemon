// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#ifndef __SD_BUS_METHOD__
#define __SD_BUS_METHOD__

#include "agent.h"
#include "log.h"
#include <glib.h>
#include <stdarg.h>
#include <stdbool.h>

#define BUS_SYSLASTORE_NAME "org.deepin.dde.Lastore1"
#define BUS_SYSLASTORE_PATH "/org/deepin/dde/Lastore1"
#define BUS_SYSLASTORE_IF_NAME "org.deepin.dde.Lastore1.Manager"

#define BUS_FREEDESKTOP_BUS_NAME "org.freedesktop.DBus"
#define BUS_FREEDESKTOP_BUS_PATH "/org/freedesktop/DBus"
#define BUS_FREEDESKTOP_BUS_IF_NAME "org.freedesktop.DBus"

#define BUS_DAEMON_EVENTLOG_NAME "org.deepin.dde.daemon.EventLog"
#define BUS_DAEMON_EVENTLOG_PATH "/org/deepin/dde/daemon/EventLog"
#define BUS_DAEMON_EVENTLOG_IF_NAME "org.deepin.dde.daemon.EventLog"

#define BUS_OSD_NOTIFICATION_NAME "org.freedesktop.Notifications"
#define BUS_OSD_NOTIFICATION_PATH "/org/freedesktop/Notifications"
#define BUS_OSD_NOTIFICATION_IF_NAME "org.freedesktop.Notifications"

#define BUS_DAEMON_NETWORK_NAME "org.deepin.dde.daemon.Network"
#define BUS_DAEMON_NETWORK_PATH "/org/deepin/dde/daemon/Network"
#define BUS_DAEMON_NETWORK_IF_NAME "org.deepin.dde.daemon.Network"

#define BUS_DAEMON_WM_NAME "com.deepin.daemon.KWayland"
#define BUS_DAEMON_WM_PATH "/com/deepin/daemon/KWayland/WindowManager"
#define BUS_DAEMON_WM_IF_NAME "com.deepin.daemon.KWayland.WindowManager"
#define BUS_DAEMON_WM_WININFO_PATH "/com/deepin/daemon/KWayland/PlasmaWindow"
#define BUS_DAEMON_WM_WININFO_IF_NAME "com.deepin.daemon.KWayland.PlasmaWindow"

#define BUS_CONTROL_CENTER_NAME "org.deepin.dde.ControlCenter1"
#define BUS_CONTROL_CENTER_PATH "/org/deepin/dde/ControlCenter1"
#define BUS_CONTROL_CENTER_IF_NAME "org.deepin.dde.ControlCenter1"

#define _cleanup_(f) __attribute__((cleanup(f)))

typedef struct
{
  char type;
  char *contents;
} type_info;
struct sd_bus_method
{
  uint32_t id;
  char *bus_name;
  char *bus_path;
  char *if_name;
  char *method_name;
  type_info **in_args;
};

typedef struct sd_bus_method sd_bus_method;

// dbus函数枚举，需要在bus_methods添加dbus具体信息。
enum BUS_METHOD
{
  BUS_METHOD_LOG_REPORT,
  BUS_METHOD_NOTIFY_CLOSE,
  BUS_METHOD_GET_CONNECTION_USER,
  BUS_METHOD_NETWORK_GET_PROXYMETHOD,
  BUS_METHOD_NETWORK_GET_PROXY,
  BUS_METHOD_NETWORK_GET_PROXY_AUTH,
  BUS_METHOD_WM_ACTIVEWINDOW,
  BUS_METHOD_NOTIFY_NOTIFY,
  BUS_METHOD_MAX,
};

extern sd_bus_method bus_methods[BUS_METHOD_MAX];

// 对应org.deepin.dde.daemon.Network.GetProxy方法的key值
#define PROXY_TYPE_HTTP "http"
#define PROXY_TYPE_HTTPS "https"
#define PROXY_TYPE_FTP "ftp"
#define PROXY_TYPE_SOCKS "socks"

// 对应系统代理环境变量
#define PROXY_ENV_HTTP "http_proxy"
#define PROXY_ENV_HTTPS "https_proxy"
#define PROXY_ENV_FTP "ftp_proxy"
#define PROXY_ENV_ALL "all_proxy"

#define UPDATE_NOTIFY_SHOW \
  "dde-control-center" // 无论控制中心状态，都需要发送的通知
#define UPDATE_NOTIFY_SHOW_OPTIONAL \
  "dde-control-center-optional" // 根据控制中心更新模块焦点状态,选择性的发通知(由dde-session-daemon的lastore
                                // agent判断后控制)

// system lastore RegisterAgent接口
int bus_syslastore_register_agent(lastore_agent *agent, char *path);

// 校验是否是系统调用
int check_caller_auth(sd_bus_message *m, lastore_agent *agent);
int sd_bus_message_get_datav(sd_bus_message *msg, va_list ap);
int sd_bus_message_get_data(sd_bus_message *msg, ...);
int sd_bus_read_dict(sd_bus_message *msg, GHashTable **map);

int sd_bus_set_data(sd_bus_message *msg, sd_bus_method *bus_method, ...);
int sd_bus_set_datav(sd_bus_message *msg, sd_bus_method *bus_method,
                     va_list ap);
int sd_bus_set_dict(sd_bus_message *msg, char *contents, GHashTable *map);

int bus_call_method(sd_bus *bus, sd_bus_method *bus_method,
                    sd_bus_message **reply, ...);
// sd-bus接口
int CloseNotification(sd_bus_message *m, void *userdata,
                      sd_bus_error *ret_error);
int GetManualProxy(sd_bus_message *m, void *userdata, sd_bus_error *ret_error);
int ReportLog(sd_bus_message *m, void *userdata, sd_bus_error *ret_error);
int SendNotify(sd_bus_message *m, void *userdata, sd_bus_error *ret_error);

#endif