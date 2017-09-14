/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

//#include "display.h"
#include <QDebug>

#include "lastore-daemon.h"

inline QDebug operator<<(QDebug d, const QDBusObjectPath& p)
{
    QList<QVariant> argumentList;
    argumentList << QVariant::fromValue(p);
    d << p.path();
    return d;
}

using namespace dbus::common;
using namespace dbus::objects;

extern void body();
int main(int argc, char *argv[])
{
    QCoreApplication app(argc, argv);
    body();
    app.exec();
}
