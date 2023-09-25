#ifndef __LOG_H__
#define __LOG_H__

#include <stdio.h>
#include <syslog.h>

#define LOG(level, format, ...)                                                \
  syslog(level, "%s:%d " format "\n", __FILE__, __LINE__, ##__VA_ARGS__)
#endif