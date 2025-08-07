package cache

import (
	"fmt"
	"reflect"
	"testing"
)

func TestMergeConfig(t *testing.T) {

	olddata := UpdateInfo{
		RepoBackend: []RepoInfo{
			RepoInfo{Name: "test-name", Type: "test-type", URL: "test-url", Components: []string{"test-com1", "test-comp2"}, FilePath: "test-file-path", HashSha256: "test-sha256"},
			RepoInfo{Name: "test2-name", Type: "test2-type", URL: "test2-url", Components: []string{"test2-com1", "test2-comp2"}, FilePath: "test2-file-path", HashSha256: "test2-sha256"},
		},
		PkgList: []AppInfo{
			AppInfo{Name: "app-name", Version: "app-Version", HashSha256: "app-sha256"},
			AppInfo{Name: "app2-name", Version: "app2-Version"},
		},
		SysCoreList: []AppInfo{
			AppInfo{Name: "core1", Version: "core1-Version"},
			AppInfo{Name: "core2", Version: "core2-Version"},
		},
		UUID:       "UUID-111",
		Time:       "Time-111",
		PkgDebPath: "Time-111",
	}

	newdata := UpdateInfo{
		RepoBackend: []RepoInfo{
			RepoInfo{Name: "test-name", Type: "test-type", URL: "test-url", Components: []string{"test-comp1", "test-comp2"}, FilePath: "test-file-path", HashSha256: "test-sha256"},
			RepoInfo{Name: "test3-name", Type: "test3-type", URL: "test2-url", Components: []string{"test2-com1", "test2-comp2"}, FilePath: "test2-file-path", HashSha256: "test2-sha256"},
		},
		PkgList: []AppInfo{
			AppInfo{Name: "app-name", Version: "app-Version"},
			AppInfo{Name: "app3-name", Version: "app3-Version"},
		},
		SysCoreList: []AppInfo{
			AppInfo{Name: "core1", Version: "core1-Version"},
			AppInfo{Name: "core2", Version: "core2-Version"},
		},
		UUID:       "UUID-111",
		Time:       "Time-222",
		PkgDebPath: "Time-111",
	}

	mergedata := UpdateInfo{
		RepoBackend: []RepoInfo{
			RepoInfo{Name: "test-name", Type: "test-type", URL: "test-url", Components: []string{"test-comp1", "test-comp2"}, FilePath: "test-file-path", HashSha256: "test-sha256"},
			RepoInfo{Name: "test3-name", Type: "test3-type", URL: "test2-url", Components: []string{"test2-com1", "test2-comp2"}, FilePath: "test2-file-path", HashSha256: "test2-sha256"},
		},
		PkgList: []AppInfo{
			AppInfo{Name: "app-name", Version: "app-Version", HashSha256: "app-sha256"},
			AppInfo{Name: "app3-name", Version: "app3-Version"},
		},
		SysCoreList: []AppInfo{
			AppInfo{Name: "core1", Version: "core1-Version"},
			AppInfo{Name: "core2", Version: "core2-Version"},
		},
		UUID:       "UUID-111",
		Time:       "Time-222",
		PkgDebPath: "Time-111",
	}

	fmt.Printf("olddata: %v\n", olddata.RepoBackend)

	t.Run("MergeConfig", func(t *testing.T) {
		olddata.MergeConfig(newdata)
		fmt.Printf("RepoBackend: \nold: %v \nmerge: %v\n", olddata.RepoBackend, mergedata.RepoBackend)
		if !reflect.DeepEqual(olddata.RepoBackend, newdata.RepoBackend) {
			t.Error("RepoBackend not equal")
		}

		fmt.Printf("Pkglist: \nold: %v \nmerge: %v\n", olddata.PkgList, mergedata.PkgList)
		if !reflect.DeepEqual(olddata.PkgList, newdata.PkgList) {
			t.Error("Pkglist not equal")
		}
	})
}
