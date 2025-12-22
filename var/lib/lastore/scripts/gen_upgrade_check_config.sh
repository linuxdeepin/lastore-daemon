#!/bin/bash

# Constants
readonly UPDATE_RUN_FLAG="/tmp/update_has_run"

# Function to start lastore-daemon service using busctl
# Triggers the service by querying JobList property via DBus
# Ignores any errors if the service is not available
start_lastore_daemon() {
    local dbus_path="org.deepin.dde.Lastore1"
    local object_path="/org/deepin/dde/Lastore1"
    local interface="org.deepin.dde.Lastore1.Manager"
    local property="JobList"

    # Execute busctl command to trigger lastore-daemon service
    # Ignore any errors if service is not available
    busctl --system get-property "$dbus_path" "$object_path" "$interface" "$property" || true
}

if [ -e "$UPDATE_RUN_FLAG" ] ;then
    echo "not first run gen_upgrade_check_config.sh"
    exit 0
fi

if [ -e "/etc/deepin/deepin_update_option.json" ] && [ ! -e "/tmp/deepin_update_option.json" ] ;  then
    # 如果文件存在，则创建软连接
    ln -s "/etc/deepin/deepin_update_option.json" "/tmp/deepin_update_option.json"
    start_lastore_daemon
else
    echo "deepin_update_option.json not exist or don't need create link"
fi

touch "$UPDATE_RUN_FLAG"
