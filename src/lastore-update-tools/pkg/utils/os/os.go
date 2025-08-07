package os

type SystemOS interface {
	GetSystemInfo() string
	GetPackageList() string
}
