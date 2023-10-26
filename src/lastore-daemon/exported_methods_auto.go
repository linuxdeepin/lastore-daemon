// Code generated by "dbusutil-gen em -type Manager,Updater"; DO NOT EDIT.

package main

import (
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (v *Manager) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name:    "CheckUpgrade",
			Fn:      v.CheckUpgrade,
			InArgs:  []string{"checkMode", "checkOrder"},
			OutArgs: []string{"job"},
		},
		{
			Name:    "ClassifiedUpgrade",
			Fn:      v.ClassifiedUpgrade,
			InArgs:  []string{"updateType"},
			OutArgs: []string{"outArg0"},
		},
		{
			Name:    "CleanArchives",
			Fn:      v.CleanArchives,
			OutArgs: []string{"job"},
		},
		{
			Name:   "CleanJob",
			Fn:     v.CleanJob,
			InArgs: []string{"jobId"},
		},
		{
			Name:    "DistUpgrade",
			Fn:      v.DistUpgrade,
			OutArgs: []string{"job"},
		},
		{
			Name:    "DistUpgradePartly",
			Fn:      v.DistUpgradePartly,
			InArgs:  []string{"mode", "needBackup"},
			OutArgs: []string{"job"},
		},
		{
			Name:    "FixError",
			Fn:      v.FixError,
			InArgs:  []string{"errType"},
			OutArgs: []string{"job"},
		},
		{
			Name:    "GetArchivesInfo",
			Fn:      v.GetArchivesInfo,
			OutArgs: []string{"info"},
		},
		{
			Name:    "GetUpdateLogs",
			Fn:      v.GetUpdateLogs,
			InArgs:  []string{"updateType", "hasHistory"},
			OutArgs: []string{"changeLogs"},
		},
		{
			Name:   "HandleSystemEvent",
			Fn:     v.HandleSystemEvent,
			InArgs: []string{"eventType"},
		},
		{
			Name:    "InstallPackage",
			Fn:      v.InstallPackage,
			InArgs:  []string{"jobName", "packages"},
			OutArgs: []string{"job"},
		},
		{
			Name:    "PackageDesktopPath",
			Fn:      v.PackageDesktopPath,
			InArgs:  []string{"pkgId"},
			OutArgs: []string{"desktopPath"},
		},
		{
			Name:    "PackageExists",
			Fn:      v.PackageExists,
			InArgs:  []string{"pkgId"},
			OutArgs: []string{"exist"},
		},
		{
			Name:    "PackageInstallable",
			Fn:      v.PackageInstallable,
			InArgs:  []string{"pkgId"},
			OutArgs: []string{"installable"},
		},
		{
			Name:    "PackagesDownloadSize",
			Fn:      v.PackagesDownloadSize,
			InArgs:  []string{"packages"},
			OutArgs: []string{"outArg0"},
		},
		{
			Name:    "PackagesSize",
			Fn:      v.PackagesSize,
			InArgs:  []string{"packages"},
			OutArgs: []string{"outArg0"},
		},
		{
			Name:   "PauseJob",
			Fn:     v.PauseJob,
			InArgs: []string{"jobId"},
		},
		{
			Name:    "PrepareDistUpgrade",
			Fn:      v.PrepareDistUpgrade,
			OutArgs: []string{"job"},
		},
		{
			Name:    "PrepareDistUpgradePartly",
			Fn:      v.PrepareDistUpgradePartly,
			InArgs:  []string{"mode"},
			OutArgs: []string{"job"},
		},
		{
			Name:   "PrepareFullScreenUpgrade",
			Fn:     v.PrepareFullScreenUpgrade,
			InArgs: []string{"option"},
		},
		{
			Name:    "QueryAllSizeWithSource",
			Fn:      v.QueryAllSizeWithSource,
			InArgs:  []string{"mode"},
			OutArgs: []string{"outArg0"},
		},
		{
			Name:   "RegisterAgent",
			Fn:     v.RegisterAgent,
			InArgs: []string{"path"},
		},
		{
			Name:    "RemovePackage",
			Fn:      v.RemovePackage,
			InArgs:  []string{"jobName", "packages"},
			OutArgs: []string{"job"},
		},
		{
			Name:   "SetAutoClean",
			Fn:     v.SetAutoClean,
			InArgs: []string{"enable"},
		},
		{
			Name:   "SetRegion",
			Fn:     v.SetRegion,
			InArgs: []string{"region"},
		},
		{
			Name:   "StartJob",
			Fn:     v.StartJob,
			InArgs: []string{"jobId"},
		},
		{
			Name:   "UnRegisterAgent",
			Fn:     v.UnRegisterAgent,
			InArgs: []string{"path"},
		},
		{
			Name:    "UpdatablePackages",
			Fn:      v.UpdatablePackages,
			InArgs:  []string{"updateType"},
			OutArgs: []string{"pkgs"},
		},
		{
			Name:    "UpdateOfflineSource",
			Fn:      v.UpdateOfflineSource,
			InArgs:  []string{"paths", "option"},
			OutArgs: []string{"job"},
		},
		{
			Name:    "UpdatePackage",
			Fn:      v.UpdatePackage,
			InArgs:  []string{"jobName", "packages"},
			OutArgs: []string{"job"},
		},
		{
			Name:    "UpdateSource",
			Fn:      v.UpdateSource,
			OutArgs: []string{"job"},
		},
	}
}
func (v *Updater) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name:    "ApplicationUpdateInfos",
			Fn:      v.ApplicationUpdateInfos,
			InArgs:  []string{"lang"},
			OutArgs: []string{"updateInfos"},
		},
		{
			Name:    "GetCheckIntervalAndTime",
			Fn:      v.GetCheckIntervalAndTime,
			OutArgs: []string{"interval", "checkTime"},
		},
		{
			Name:    "ListMirrorSources",
			Fn:      v.ListMirrorSources,
			InArgs:  []string{"lang"},
			OutArgs: []string{"mirrorSources"},
		},
		{
			Name: "RestoreSystemSource",
			Fn:   v.RestoreSystemSource,
		},
		{
			Name:   "SetAutoCheckUpdates",
			Fn:     v.SetAutoCheckUpdates,
			InArgs: []string{"enable"},
		},
		{
			Name:   "SetAutoDownloadUpdates",
			Fn:     v.SetAutoDownloadUpdates,
			InArgs: []string{"enable"},
		},
		{
			Name:   "SetDownloadSpeedLimit",
			Fn:     v.SetDownloadSpeedLimit,
			InArgs: []string{"limitConfig"},
		},
		{
			Name:   "SetIdleDownloadConfig",
			Fn:     v.SetIdleDownloadConfig,
			InArgs: []string{"idleConfig"},
		},
		{
			Name:   "SetMirrorSource",
			Fn:     v.SetMirrorSource,
			InArgs: []string{"id"},
		},
		{
			Name:   "SetUpdateNotify",
			Fn:     v.SetUpdateNotify,
			InArgs: []string{"enable"},
		},
	}
}
