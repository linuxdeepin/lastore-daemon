#!/bin/bash
if [ -e "/tmp/update_has_run" ] ;then
    echo "not first run gen_upgrade_check_config.sh"
    exit 0
fi

if [ -e "/etc/deepin/deepin_update_option.json" ] && [ ! -e "/tmp/deepin_update_option.json" ] ;  then
    # 如果文件存在，则创建软连接
    ln -s "/etc/deepin/deepin_update_option.json" "/tmp/deepin_update_option.json"
    systemctl start lastore-daemon.service > /dev/null || true &
else
    echo "deepin_update_option.json not exist or don't need create link"
fi

touch "/tmp/update_has_run"