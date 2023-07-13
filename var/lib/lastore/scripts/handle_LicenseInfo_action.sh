#!/bin/bash

dbus-send --system --print-reply --dest=com.deepin.lastore /com/deepin/lastore com.deepin.lastore.Manager.HandleSystemEvent string:"OsVersionChanged"