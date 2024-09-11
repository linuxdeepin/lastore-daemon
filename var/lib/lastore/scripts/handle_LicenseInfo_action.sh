#!/bin/bash

dbus-send --system --print-reply --dest=org.deepin.dde.Lastore1 /org/deepin/dde/Lastore1 org.deepin.dde.Lastore1.Manager.HandleSystemEvent string:"OsVersionChanged"
