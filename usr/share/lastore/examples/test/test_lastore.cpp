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
void body()
{
  Manager* m = new Manager(QString("system"));
  m = new Manager(QString("system"));
  R<bool >r = m->PackageExists("deepin-movie");
  qDebug() << r.Value<0>() << r.Error();

  R<QDBusObjectPath> rpath = m->RemovePackage("deepin-movie");
  j = new Job(QString("system"), "org.deepin.lastore", rpath.Value<0>().path());
  m->StartJob(j->id().Value<0>());

  j->connect(j, &Job::progressChanged, printStatus);
}
