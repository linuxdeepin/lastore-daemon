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
    Manager* m = new Manager(0, QString("/org/deepin/lastore"), QString("system"));
    R<bool >r = m->CheckPackageExists("deepin-movie");
    qDebug() << r.Value<0>() << r.Error();

    QList<QString> ps;
    ps << "deepin-movie";
    R<QDBusObjectPath> rpath = m->RemovePackages(ps);
    j = new Job(0, rpath.Value<0>().path(), QString("system"));
    m->StartJob(j->id().Value<0>());

    j->connect(j, &Job::progressChanged, printStatus);
}
