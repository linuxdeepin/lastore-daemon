/*This file is automatically generated by pkg.deepin.io/dbus-generator. Don't edit it*/
package login1

import "pkg.deepin.io/lib/dbus"
import "pkg.deepin.io/lib/dbus/property"
import "reflect"
import "sync"
import "runtime"
import "fmt"
import "errors"

/*prevent compile error*/
var _ = fmt.Println
var _ = runtime.SetFinalizer
var _ = sync.NewCond
var _ = reflect.TypeOf
var _ = property.BaseObserver{}

type Session struct {
	Path     dbus.ObjectPath
	DestName string
	core     *dbus.Object

	signals       map[<-chan *dbus.Signal]struct{}
	signalsLocker sync.Mutex

	Id                     *dbusPropertySessionId
	User                   *dbusPropertySessionUser
	Name                   *dbusPropertySessionName
	Timestamp              *dbusPropertySessionTimestamp
	TimestampMonotonic     *dbusPropertySessionTimestampMonotonic
	DefaultControlGroup    *dbusPropertySessionDefaultControlGroup
	VTNr                   *dbusPropertySessionVTNr
	Seat                   *dbusPropertySessionSeat
	TTY                    *dbusPropertySessionTTY
	Display                *dbusPropertySessionDisplay
	Remote                 *dbusPropertySessionRemote
	RemoteHost             *dbusPropertySessionRemoteHost
	RemoteUser             *dbusPropertySessionRemoteUser
	Service                *dbusPropertySessionService
	Leader                 *dbusPropertySessionLeader
	Audit                  *dbusPropertySessionAudit
	Type                   *dbusPropertySessionType
	Class                  *dbusPropertySessionClass
	Active                 *dbusPropertySessionActive
	State                  *dbusPropertySessionState
	Controllers            *dbusPropertySessionControllers
	ResetControllers       *dbusPropertySessionResetControllers
	KillProcesses          *dbusPropertySessionKillProcesses
	IdleHint               *dbusPropertySessionIdleHint
	IdleSinceHint          *dbusPropertySessionIdleSinceHint
	IdleSinceHintMonotonic *dbusPropertySessionIdleSinceHintMonotonic
}

func (obj *Session) _createSignalChan() <-chan *dbus.Signal {
	obj.signalsLocker.Lock()
	ch := getBus().Signal()
	obj.signals[ch] = struct{}{}
	obj.signalsLocker.Unlock()
	return ch
}
func (obj *Session) _deleteSignalChan(ch <-chan *dbus.Signal) {
	obj.signalsLocker.Lock()
	delete(obj.signals, ch)
	getBus().DetachSignal(ch)
	obj.signalsLocker.Unlock()
}
func DestroySession(obj *Session) {
	obj.signalsLocker.Lock()
	for ch, _ := range obj.signals {
		getBus().DetachSignal(ch)
	}
	obj.signalsLocker.Unlock()

	obj.Id.Reset()
	obj.User.Reset()
	obj.Name.Reset()
	obj.Timestamp.Reset()
	obj.TimestampMonotonic.Reset()
	obj.DefaultControlGroup.Reset()
	obj.VTNr.Reset()
	obj.Seat.Reset()
	obj.TTY.Reset()
	obj.Display.Reset()
	obj.Remote.Reset()
	obj.RemoteHost.Reset()
	obj.RemoteUser.Reset()
	obj.Service.Reset()
	obj.Leader.Reset()
	obj.Audit.Reset()
	obj.Type.Reset()
	obj.Class.Reset()
	obj.Active.Reset()
	obj.State.Reset()
	obj.Controllers.Reset()
	obj.ResetControllers.Reset()
	obj.KillProcesses.Reset()
	obj.IdleHint.Reset()
	obj.IdleSinceHint.Reset()
	obj.IdleSinceHintMonotonic.Reset()
}

func (obj *Session) Terminate() (_err error) {
	_err = obj.core.Call("org.freedesktop.login1.Session.Terminate", 0).Store()
	if _err != nil {
		fmt.Println(_err)
	}
	return
}

func (obj *Session) Activate() (_err error) {
	_err = obj.core.Call("org.freedesktop.login1.Session.Activate", 0).Store()
	if _err != nil {
		fmt.Println(_err)
	}
	return
}

func (obj *Session) Lock() (_err error) {
	_err = obj.core.Call("org.freedesktop.login1.Session.Lock", 0).Store()
	if _err != nil {
		fmt.Println(_err)
	}
	return
}

func (obj *Session) Unlock() (_err error) {
	_err = obj.core.Call("org.freedesktop.login1.Session.Unlock", 0).Store()
	if _err != nil {
		fmt.Println(_err)
	}
	return
}

func (obj *Session) SetIdleHint(b bool) (_err error) {
	_err = obj.core.Call("org.freedesktop.login1.Session.SetIdleHint", 0, b).Store()
	if _err != nil {
		fmt.Println(_err)
	}
	return
}

func (obj *Session) Kill(who string, signal string) (_err error) {
	_err = obj.core.Call("org.freedesktop.login1.Session.Kill", 0, who, signal).Store()
	if _err != nil {
		fmt.Println(_err)
	}
	return
}

func (obj *Session) ConnectLock(callback func()) func() {
	__conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',path='"+string(obj.Path)+"', interface='org.freedesktop.login1.Session',sender='"+obj.DestName+"',member='Lock'")
	sigChan := obj._createSignalChan()
	go func() {
		for v := range sigChan {
			if v.Path != obj.Path || v.Name != "org.freedesktop.login1.Session.Lock" || 0 != len(v.Body) {
				continue
			}

			callback()
		}
	}()
	return func() {
		obj._deleteSignalChan(sigChan)
	}
}

func (obj *Session) ConnectUnlock(callback func()) func() {
	__conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',path='"+string(obj.Path)+"', interface='org.freedesktop.login1.Session',sender='"+obj.DestName+"',member='Unlock'")
	sigChan := obj._createSignalChan()
	go func() {
		for v := range sigChan {
			if v.Path != obj.Path || v.Name != "org.freedesktop.login1.Session.Unlock" || 0 != len(v.Body) {
				continue
			}

			callback()
		}
	}()
	return func() {
		obj._deleteSignalChan(sigChan)
	}
}

type dbusPropertySessionId struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionId) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Id is not writable")
}

func (this *dbusPropertySessionId) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionId) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Id").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:Id error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionId) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionUser struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionUser) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.User is not writable")
}

func (this *dbusPropertySessionUser) Get() []interface{} {
	return this.GetValue().([]interface{})
}
func (this *dbusPropertySessionUser) GetValue() interface{} /*[]interface {}*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "User").Store(&r)
	if err == nil && r.Signature().String() == "(uo)" {
		return r.Value().([]interface{})
	} else {
		fmt.Println("dbusProperty:User error:", err, "at org.freedesktop.login1.Session")
		return *new([]interface{})
	}
}
func (this *dbusPropertySessionUser) GetType() reflect.Type {
	return reflect.TypeOf((*[]interface{})(nil)).Elem()
}

type dbusPropertySessionName struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionName) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Name is not writable")
}

func (this *dbusPropertySessionName) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionName) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Name").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:Name error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionName) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionTimestamp struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionTimestamp) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Timestamp is not writable")
}

func (this *dbusPropertySessionTimestamp) Get() uint64 {
	return this.GetValue().(uint64)
}
func (this *dbusPropertySessionTimestamp) GetValue() interface{} /*uint64*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Timestamp").Store(&r)
	if err == nil && r.Signature().String() == "t" {
		return r.Value().(uint64)
	} else {
		fmt.Println("dbusProperty:Timestamp error:", err, "at org.freedesktop.login1.Session")
		return *new(uint64)
	}
}
func (this *dbusPropertySessionTimestamp) GetType() reflect.Type {
	return reflect.TypeOf((*uint64)(nil)).Elem()
}

type dbusPropertySessionTimestampMonotonic struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionTimestampMonotonic) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.TimestampMonotonic is not writable")
}

func (this *dbusPropertySessionTimestampMonotonic) Get() uint64 {
	return this.GetValue().(uint64)
}
func (this *dbusPropertySessionTimestampMonotonic) GetValue() interface{} /*uint64*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "TimestampMonotonic").Store(&r)
	if err == nil && r.Signature().String() == "t" {
		return r.Value().(uint64)
	} else {
		fmt.Println("dbusProperty:TimestampMonotonic error:", err, "at org.freedesktop.login1.Session")
		return *new(uint64)
	}
}
func (this *dbusPropertySessionTimestampMonotonic) GetType() reflect.Type {
	return reflect.TypeOf((*uint64)(nil)).Elem()
}

type dbusPropertySessionDefaultControlGroup struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionDefaultControlGroup) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.DefaultControlGroup is not writable")
}

func (this *dbusPropertySessionDefaultControlGroup) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionDefaultControlGroup) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "DefaultControlGroup").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:DefaultControlGroup error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionDefaultControlGroup) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionVTNr struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionVTNr) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.VTNr is not writable")
}

func (this *dbusPropertySessionVTNr) Get() uint32 {
	return this.GetValue().(uint32)
}
func (this *dbusPropertySessionVTNr) GetValue() interface{} /*uint32*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "VTNr").Store(&r)
	if err == nil && r.Signature().String() == "u" {
		return r.Value().(uint32)
	} else {
		fmt.Println("dbusProperty:VTNr error:", err, "at org.freedesktop.login1.Session")
		return *new(uint32)
	}
}
func (this *dbusPropertySessionVTNr) GetType() reflect.Type {
	return reflect.TypeOf((*uint32)(nil)).Elem()
}

type dbusPropertySessionSeat struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionSeat) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Seat is not writable")
}

func (this *dbusPropertySessionSeat) Get() []interface{} {
	return this.GetValue().([]interface{})
}
func (this *dbusPropertySessionSeat) GetValue() interface{} /*[]interface {}*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Seat").Store(&r)
	if err == nil && r.Signature().String() == "(so)" {
		return r.Value().([]interface{})
	} else {
		fmt.Println("dbusProperty:Seat error:", err, "at org.freedesktop.login1.Session")
		return *new([]interface{})
	}
}
func (this *dbusPropertySessionSeat) GetType() reflect.Type {
	return reflect.TypeOf((*[]interface{})(nil)).Elem()
}

type dbusPropertySessionTTY struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionTTY) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.TTY is not writable")
}

func (this *dbusPropertySessionTTY) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionTTY) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "TTY").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:TTY error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionTTY) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionDisplay struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionDisplay) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Display is not writable")
}

func (this *dbusPropertySessionDisplay) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionDisplay) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Display").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:Display error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionDisplay) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionRemote struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionRemote) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Remote is not writable")
}

func (this *dbusPropertySessionRemote) Get() bool {
	return this.GetValue().(bool)
}
func (this *dbusPropertySessionRemote) GetValue() interface{} /*bool*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Remote").Store(&r)
	if err == nil && r.Signature().String() == "b" {
		return r.Value().(bool)
	} else {
		fmt.Println("dbusProperty:Remote error:", err, "at org.freedesktop.login1.Session")
		return *new(bool)
	}
}
func (this *dbusPropertySessionRemote) GetType() reflect.Type {
	return reflect.TypeOf((*bool)(nil)).Elem()
}

type dbusPropertySessionRemoteHost struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionRemoteHost) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.RemoteHost is not writable")
}

func (this *dbusPropertySessionRemoteHost) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionRemoteHost) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "RemoteHost").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:RemoteHost error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionRemoteHost) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionRemoteUser struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionRemoteUser) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.RemoteUser is not writable")
}

func (this *dbusPropertySessionRemoteUser) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionRemoteUser) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "RemoteUser").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:RemoteUser error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionRemoteUser) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionService struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionService) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Service is not writable")
}

func (this *dbusPropertySessionService) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionService) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Service").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:Service error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionService) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionLeader struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionLeader) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Leader is not writable")
}

func (this *dbusPropertySessionLeader) Get() uint32 {
	return this.GetValue().(uint32)
}
func (this *dbusPropertySessionLeader) GetValue() interface{} /*uint32*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Leader").Store(&r)
	if err == nil && r.Signature().String() == "u" {
		return r.Value().(uint32)
	} else {
		fmt.Println("dbusProperty:Leader error:", err, "at org.freedesktop.login1.Session")
		return *new(uint32)
	}
}
func (this *dbusPropertySessionLeader) GetType() reflect.Type {
	return reflect.TypeOf((*uint32)(nil)).Elem()
}

type dbusPropertySessionAudit struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionAudit) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Audit is not writable")
}

func (this *dbusPropertySessionAudit) Get() uint32 {
	return this.GetValue().(uint32)
}
func (this *dbusPropertySessionAudit) GetValue() interface{} /*uint32*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Audit").Store(&r)
	if err == nil && r.Signature().String() == "u" {
		return r.Value().(uint32)
	} else {
		fmt.Println("dbusProperty:Audit error:", err, "at org.freedesktop.login1.Session")
		return *new(uint32)
	}
}
func (this *dbusPropertySessionAudit) GetType() reflect.Type {
	return reflect.TypeOf((*uint32)(nil)).Elem()
}

type dbusPropertySessionType struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionType) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Type is not writable")
}

func (this *dbusPropertySessionType) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionType) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Type").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:Type error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionType) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionClass struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionClass) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Class is not writable")
}

func (this *dbusPropertySessionClass) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionClass) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Class").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:Class error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionClass) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionActive struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionActive) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Active is not writable")
}

func (this *dbusPropertySessionActive) Get() bool {
	return this.GetValue().(bool)
}
func (this *dbusPropertySessionActive) GetValue() interface{} /*bool*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Active").Store(&r)
	if err == nil && r.Signature().String() == "b" {
		return r.Value().(bool)
	} else {
		fmt.Println("dbusProperty:Active error:", err, "at org.freedesktop.login1.Session")
		return *new(bool)
	}
}
func (this *dbusPropertySessionActive) GetType() reflect.Type {
	return reflect.TypeOf((*bool)(nil)).Elem()
}

type dbusPropertySessionState struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionState) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.State is not writable")
}

func (this *dbusPropertySessionState) Get() string {
	return this.GetValue().(string)
}
func (this *dbusPropertySessionState) GetValue() interface{} /*string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "State").Store(&r)
	if err == nil && r.Signature().String() == "s" {
		return r.Value().(string)
	} else {
		fmt.Println("dbusProperty:State error:", err, "at org.freedesktop.login1.Session")
		return *new(string)
	}
}
func (this *dbusPropertySessionState) GetType() reflect.Type {
	return reflect.TypeOf((*string)(nil)).Elem()
}

type dbusPropertySessionControllers struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionControllers) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.Controllers is not writable")
}

func (this *dbusPropertySessionControllers) Get() []string {
	return this.GetValue().([]string)
}
func (this *dbusPropertySessionControllers) GetValue() interface{} /*[]string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "Controllers").Store(&r)
	if err == nil && r.Signature().String() == "as" {
		return r.Value().([]string)
	} else {
		fmt.Println("dbusProperty:Controllers error:", err, "at org.freedesktop.login1.Session")
		return *new([]string)
	}
}
func (this *dbusPropertySessionControllers) GetType() reflect.Type {
	return reflect.TypeOf((*[]string)(nil)).Elem()
}

type dbusPropertySessionResetControllers struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionResetControllers) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.ResetControllers is not writable")
}

func (this *dbusPropertySessionResetControllers) Get() []string {
	return this.GetValue().([]string)
}
func (this *dbusPropertySessionResetControllers) GetValue() interface{} /*[]string*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "ResetControllers").Store(&r)
	if err == nil && r.Signature().String() == "as" {
		return r.Value().([]string)
	} else {
		fmt.Println("dbusProperty:ResetControllers error:", err, "at org.freedesktop.login1.Session")
		return *new([]string)
	}
}
func (this *dbusPropertySessionResetControllers) GetType() reflect.Type {
	return reflect.TypeOf((*[]string)(nil)).Elem()
}

type dbusPropertySessionKillProcesses struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionKillProcesses) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.KillProcesses is not writable")
}

func (this *dbusPropertySessionKillProcesses) Get() bool {
	return this.GetValue().(bool)
}
func (this *dbusPropertySessionKillProcesses) GetValue() interface{} /*bool*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "KillProcesses").Store(&r)
	if err == nil && r.Signature().String() == "b" {
		return r.Value().(bool)
	} else {
		fmt.Println("dbusProperty:KillProcesses error:", err, "at org.freedesktop.login1.Session")
		return *new(bool)
	}
}
func (this *dbusPropertySessionKillProcesses) GetType() reflect.Type {
	return reflect.TypeOf((*bool)(nil)).Elem()
}

type dbusPropertySessionIdleHint struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionIdleHint) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.IdleHint is not writable")
}

func (this *dbusPropertySessionIdleHint) Get() bool {
	return this.GetValue().(bool)
}
func (this *dbusPropertySessionIdleHint) GetValue() interface{} /*bool*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "IdleHint").Store(&r)
	if err == nil && r.Signature().String() == "b" {
		return r.Value().(bool)
	} else {
		fmt.Println("dbusProperty:IdleHint error:", err, "at org.freedesktop.login1.Session")
		return *new(bool)
	}
}
func (this *dbusPropertySessionIdleHint) GetType() reflect.Type {
	return reflect.TypeOf((*bool)(nil)).Elem()
}

type dbusPropertySessionIdleSinceHint struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionIdleSinceHint) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.IdleSinceHint is not writable")
}

func (this *dbusPropertySessionIdleSinceHint) Get() uint64 {
	return this.GetValue().(uint64)
}
func (this *dbusPropertySessionIdleSinceHint) GetValue() interface{} /*uint64*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "IdleSinceHint").Store(&r)
	if err == nil && r.Signature().String() == "t" {
		return r.Value().(uint64)
	} else {
		fmt.Println("dbusProperty:IdleSinceHint error:", err, "at org.freedesktop.login1.Session")
		return *new(uint64)
	}
}
func (this *dbusPropertySessionIdleSinceHint) GetType() reflect.Type {
	return reflect.TypeOf((*uint64)(nil)).Elem()
}

type dbusPropertySessionIdleSinceHintMonotonic struct {
	*property.BaseObserver
	core *dbus.Object
}

func (this *dbusPropertySessionIdleSinceHintMonotonic) SetValue(notwritable interface{}) {
	fmt.Println("org.freedesktop.login1.Session.IdleSinceHintMonotonic is not writable")
}

func (this *dbusPropertySessionIdleSinceHintMonotonic) Get() uint64 {
	return this.GetValue().(uint64)
}
func (this *dbusPropertySessionIdleSinceHintMonotonic) GetValue() interface{} /*uint64*/ {
	var r dbus.Variant
	err := this.core.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.login1.Session", "IdleSinceHintMonotonic").Store(&r)
	if err == nil && r.Signature().String() == "t" {
		return r.Value().(uint64)
	} else {
		fmt.Println("dbusProperty:IdleSinceHintMonotonic error:", err, "at org.freedesktop.login1.Session")
		return *new(uint64)
	}
}
func (this *dbusPropertySessionIdleSinceHintMonotonic) GetType() reflect.Type {
	return reflect.TypeOf((*uint64)(nil)).Elem()
}

func NewSession(destName string, path dbus.ObjectPath) (*Session, error) {
	if !path.IsValid() {
		return nil, errors.New("The path of '" + string(path) + "' is invalid.")
	}

	core := getBus().Object(destName, path)

	obj := &Session{Path: path, DestName: destName, core: core, signals: make(map[<-chan *dbus.Signal]struct{})}

	obj.Id = &dbusPropertySessionId{&property.BaseObserver{}, core}
	obj.User = &dbusPropertySessionUser{&property.BaseObserver{}, core}
	obj.Name = &dbusPropertySessionName{&property.BaseObserver{}, core}
	obj.Timestamp = &dbusPropertySessionTimestamp{&property.BaseObserver{}, core}
	obj.TimestampMonotonic = &dbusPropertySessionTimestampMonotonic{&property.BaseObserver{}, core}
	obj.DefaultControlGroup = &dbusPropertySessionDefaultControlGroup{&property.BaseObserver{}, core}
	obj.VTNr = &dbusPropertySessionVTNr{&property.BaseObserver{}, core}
	obj.Seat = &dbusPropertySessionSeat{&property.BaseObserver{}, core}
	obj.TTY = &dbusPropertySessionTTY{&property.BaseObserver{}, core}
	obj.Display = &dbusPropertySessionDisplay{&property.BaseObserver{}, core}
	obj.Remote = &dbusPropertySessionRemote{&property.BaseObserver{}, core}
	obj.RemoteHost = &dbusPropertySessionRemoteHost{&property.BaseObserver{}, core}
	obj.RemoteUser = &dbusPropertySessionRemoteUser{&property.BaseObserver{}, core}
	obj.Service = &dbusPropertySessionService{&property.BaseObserver{}, core}
	obj.Leader = &dbusPropertySessionLeader{&property.BaseObserver{}, core}
	obj.Audit = &dbusPropertySessionAudit{&property.BaseObserver{}, core}
	obj.Type = &dbusPropertySessionType{&property.BaseObserver{}, core}
	obj.Class = &dbusPropertySessionClass{&property.BaseObserver{}, core}
	obj.Active = &dbusPropertySessionActive{&property.BaseObserver{}, core}
	obj.State = &dbusPropertySessionState{&property.BaseObserver{}, core}
	obj.Controllers = &dbusPropertySessionControllers{&property.BaseObserver{}, core}
	obj.ResetControllers = &dbusPropertySessionResetControllers{&property.BaseObserver{}, core}
	obj.KillProcesses = &dbusPropertySessionKillProcesses{&property.BaseObserver{}, core}
	obj.IdleHint = &dbusPropertySessionIdleHint{&property.BaseObserver{}, core}
	obj.IdleSinceHint = &dbusPropertySessionIdleSinceHint{&property.BaseObserver{}, core}
	obj.IdleSinceHintMonotonic = &dbusPropertySessionIdleSinceHintMonotonic{&property.BaseObserver{}, core}

	getBus().BusObject().Call("org.freedesktop.DBus.AddMatch", 0, "type='signal',path='"+string(path)+"',interface='org.freedesktop.DBus.Properties',sender='"+destName+"',member='PropertiesChanged'")
	getBus().BusObject().Call("org.freedesktop.DBus.AddMatch", 0, "type='signal',path='"+string(path)+"',interface='org.freedesktop.login1.Session',sender='"+destName+"',member='PropertiesChanged'")
	sigChan := obj._createSignalChan()
	go func() {
		typeString := reflect.TypeOf("")
		typeKeyValues := reflect.TypeOf(map[string]dbus.Variant{})
		typeArrayValues := reflect.TypeOf([]string{})
		for v := range sigChan {
			if v.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" &&
				len(v.Body) == 3 &&
				reflect.TypeOf(v.Body[0]) == typeString &&
				reflect.TypeOf(v.Body[1]) == typeKeyValues &&
				reflect.TypeOf(v.Body[2]) == typeArrayValues &&
				v.Body[0].(string) == "org.freedesktop.login1.Session" {
				props := v.Body[1].(map[string]dbus.Variant)
				for key, _ := range props {
					if false {
					} else if key == "Id" {
						obj.Id.Notify()

					} else if key == "User" {
						obj.User.Notify()

					} else if key == "Name" {
						obj.Name.Notify()

					} else if key == "Timestamp" {
						obj.Timestamp.Notify()

					} else if key == "TimestampMonotonic" {
						obj.TimestampMonotonic.Notify()

					} else if key == "DefaultControlGroup" {
						obj.DefaultControlGroup.Notify()

					} else if key == "VTNr" {
						obj.VTNr.Notify()

					} else if key == "Seat" {
						obj.Seat.Notify()

					} else if key == "TTY" {
						obj.TTY.Notify()

					} else if key == "Display" {
						obj.Display.Notify()

					} else if key == "Remote" {
						obj.Remote.Notify()

					} else if key == "RemoteHost" {
						obj.RemoteHost.Notify()

					} else if key == "RemoteUser" {
						obj.RemoteUser.Notify()

					} else if key == "Service" {
						obj.Service.Notify()

					} else if key == "Leader" {
						obj.Leader.Notify()

					} else if key == "Audit" {
						obj.Audit.Notify()

					} else if key == "Type" {
						obj.Type.Notify()

					} else if key == "Class" {
						obj.Class.Notify()

					} else if key == "Active" {
						obj.Active.Notify()

					} else if key == "State" {
						obj.State.Notify()

					} else if key == "Controllers" {
						obj.Controllers.Notify()

					} else if key == "ResetControllers" {
						obj.ResetControllers.Notify()

					} else if key == "KillProcesses" {
						obj.KillProcesses.Notify()

					} else if key == "IdleHint" {
						obj.IdleHint.Notify()

					} else if key == "IdleSinceHint" {
						obj.IdleSinceHint.Notify()

					} else if key == "IdleSinceHintMonotonic" {
						obj.IdleSinceHintMonotonic.Notify()
					}
				}
			} else if v.Name == "org.freedesktop.login1.Session.PropertiesChanged" && len(v.Body) == 1 && reflect.TypeOf(v.Body[0]) == typeKeyValues {
				for key, _ := range v.Body[0].(map[string]dbus.Variant) {
					if false {
					} else if key == "Id" {
						obj.Id.Notify()

					} else if key == "User" {
						obj.User.Notify()

					} else if key == "Name" {
						obj.Name.Notify()

					} else if key == "Timestamp" {
						obj.Timestamp.Notify()

					} else if key == "TimestampMonotonic" {
						obj.TimestampMonotonic.Notify()

					} else if key == "DefaultControlGroup" {
						obj.DefaultControlGroup.Notify()

					} else if key == "VTNr" {
						obj.VTNr.Notify()

					} else if key == "Seat" {
						obj.Seat.Notify()

					} else if key == "TTY" {
						obj.TTY.Notify()

					} else if key == "Display" {
						obj.Display.Notify()

					} else if key == "Remote" {
						obj.Remote.Notify()

					} else if key == "RemoteHost" {
						obj.RemoteHost.Notify()

					} else if key == "RemoteUser" {
						obj.RemoteUser.Notify()

					} else if key == "Service" {
						obj.Service.Notify()

					} else if key == "Leader" {
						obj.Leader.Notify()

					} else if key == "Audit" {
						obj.Audit.Notify()

					} else if key == "Type" {
						obj.Type.Notify()

					} else if key == "Class" {
						obj.Class.Notify()

					} else if key == "Active" {
						obj.Active.Notify()

					} else if key == "State" {
						obj.State.Notify()

					} else if key == "Controllers" {
						obj.Controllers.Notify()

					} else if key == "ResetControllers" {
						obj.ResetControllers.Notify()

					} else if key == "KillProcesses" {
						obj.KillProcesses.Notify()

					} else if key == "IdleHint" {
						obj.IdleHint.Notify()

					} else if key == "IdleSinceHint" {
						obj.IdleSinceHint.Notify()

					} else if key == "IdleSinceHintMonotonic" {
						obj.IdleSinceHintMonotonic.Notify()
					}
				}
			}
		}
	}()

	runtime.SetFinalizer(obj, func(_obj *Session) { DestroySession(_obj) })
	return obj, nil
}