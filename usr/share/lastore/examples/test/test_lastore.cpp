//#include "display.h"
#include <QDebug>

#include "lastore-daemon.h"
using namespace dbus::common;
using namespace dbus::objects;
using namespace dbus::objects::org::deepin::lastore;

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

  j = new Job(QString("system"), "org.deepin.lastore", rpath.Value<0>().path());
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

  j = new Job(QString("system"), "org.deepin.lastore", rpath.Value<0>().path());
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
