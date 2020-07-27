#include "ip.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/ioctl.h>
#include <net/if.h>
#include <arpa/inet.h>

#define IPv4_LEN 15

char *
get_iface_ip(const char *iface)
{
    if (!iface) {
        return NULL;
    }

    int fd;
    struct ifreq ifr;

    fd = socket(AF_INET, SOCK_DGRAM, 0);
    if (fd < 0) {
        fprintf(stderr, "Failed to open socket: %s\n", strerror(errno));
        return NULL;
    }

    memset(&ifr, 0, sizeof(ifr));
    ifr.ifr_addr.sa_family = AF_INET;
    strncpy(ifr.ifr_name, iface, IFNAMSIZ -1);
    ioctl(fd, SIOCGIFADDR, &ifr);
    close(fd);

    char *ip = inet_ntoa(((struct sockaddr_in*)&(ifr.ifr_addr))->sin_addr);
    char *ret = calloc(IPv4_LEN+1, sizeof(char));
    if (!ret) {
        fprintf(stderr, "Failed to alloc memory: %s\n", strerror(errno));
        return NULL;
    }

    memcpy(ret, ip, IPv4_LEN);
    return ret;
}
