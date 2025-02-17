// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#ifndef __LASTORE_AGENT_H__
#define __LASTORE_AGENT_H__

#include "log.h"
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <syslog.h>
#include <systemd/sd-bus.h>

struct lastore_agent
{
  sd_bus *session_bus;
  sd_bus *sys_bus;
  sd_bus_slot *slot;
  bool is_wayland_session;
};

typedef struct lastore_agent lastore_agent;

#define OBJECT_PATH "/org/deepin/dde/Lastore1/Agent"
#define INTERFACE_NAME "org.deepin.dde.Lastore1.Agent"

lastore_agent *agent_init();
void agent_loop(lastore_agent *agent);
void agent_close(lastore_agent *agent);
#endif