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
using namespace dbus::common;
using namespace dbus::objects;
using namespace dbus::objects::com::deepin::lastore;

Job* j = 0;
void printStatus() {
    qDebug() << j->progress().Value<0>() << j->status().Value<0>() << j->description().Value<0>();
}

void install_package(const char* package)
{
  Manager* m = new Manager(QString("system"));

  R<QDBusObjectPath> rpath = m->InstallPackage(package, "mainland");
  if (rpath.hasError()) {
    qDebug () << "Found Error When install " << package << ": " << rpath.Error();
    return;
  }

  j = new Job(QString("system"), "org.deepin.dde.Lastore1", rpath.Value<0>().path());
  j->connect(j, &Job::progressChanged, printStatus);
  R<void> r =  m->StartJob(j->id().Value<0>());
 
  if (r.hasError()) {
      qDebug () << "Found Error when start job " << r.Error();
      return;
  }
}

void remove_package(const char* package)
{
   Manager* m = new Manager(QString("system"));

  R<QDBusObjectPath> rpath = m->RemovePackage(package);
  if (rpath.hasError()) {
    qDebug () << "Found Error When removing  " << package << ": " << rpath.Error();
    return;
  }

  j = new Job(QString("system"), "org.deepin.dde.Lastore1", rpath.Value<0>().path());
  j->connect(j, &Job::progressChanged, printStatus);
  R<void> r =  m->StartJob(j->id().Value<0>());
  

  if (r.hasError()) {
      qDebug () << "Found Error when start job " << r.Error();
      return;
  }
}

void body()
{
  install_package("deepin-movie");
  remove_package("deepin-movie");
}
