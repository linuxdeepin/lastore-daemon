// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#include "agent.h"
#include "log.h"
#include <string.h>
#include <syslog.h>
#include <systemd/sd-bus.h>

#define PROG_NAME "lastore-agent"

int main(void) {
  // 初始化日志系统，指定程序名称和选项
  openlog(PROG_NAME, LOG_PID, LOG_USER);

  // 初始化lastore
  lastore_agent *agent = agent_init();
  if (agent == NULL) {
    LOG(LOG_ERR, "Init %s err", PROG_NAME);
    return -1;
  }

  agent_loop(agent);
  return 0;
}