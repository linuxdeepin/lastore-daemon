/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package main

// AudioVideo/Audio/Video -> multimedia
// Development -> Development
// Education -> Education
// Game -> Games
// Graphics -> Graphics
// Network -> Network
// System/settings -> System
// Utility -> Utilities
// office -> Productivity
// -> Industry

func GenerateXCategories(fpath string) error {
	const (
		internet    = "internet"
		office      = "office"
		development = "development"
		reading     = "reading"
		graphics    = "graphics"
		game        = "game"
		music       = "music"
		system      = "system"
		video       = "video"
		chat        = "chat"
		others      = "others"
	)

	var xCategoryNameIdMap map[string]string = map[string]string{
		"network":           internet,
		"webbrowser":        internet,
		"email":             internet,
		"contactmanagement": chat,
		"filetransfer":      internet,
		"p2p":               internet,
		"instantmessaging":  chat,
		"chat":              chat,
		"ircclient":         chat,
		"news":              reading,
		"remoteaccess":      system,

		"tv":                video,
		"multimedia":        video,
		"audio":             music,
		"video":             video,
		"audiovideo":        video,
		"audiovideoediting": video,
		"discburning":       system,
		"midi":              music,
		"mixer":             music,
		"player":            music,
		"music":             music,
		"recorder":          music,
		"sequencer":         music,
		"tuner":             music,

		"game":          game,
		"amusement":     game,
		"actiongame":    game,
		"adventuregame": game,
		"arcadegame":    game,
		"emulator":      game,
		"simulation":    game,
		"kidsgame":      game,
		"logicgame":     game,
		"roleplaying":   game,
		"sportsgame":    game,
		"strategygame":  game,

		"graphics":        graphics,
		"2dgraphics":      graphics,
		"3dgraphics":      graphics,
		"imageprocessing": graphics,
		"ocr":             graphics,
		"photography":     graphics,
		"rastergraphics":  graphics,
		"vectorgraphics":  graphics,
		"viewer":          graphics,

		"office":            office,
		"spreadsheet":       office,
		"wordprocessor":     office,
		"projectmanagement": office,
		"chart":             office,
		"numericalanalysis": office,
		"presentation":      office,
		"scanning":          office,
		"printing":          office,

		"engineering":     system,
		"telephonytools":  system,
		"telephony":       system,
		"finance":         office,
		"hamradio":        office,
		"medicalsoftware": office,
		"publishing":      office,

		"education":              office,
		"art":                    office,
		"literature":             office,
		"dictionary":             office,
		"artificialintelligence": office,
		"electricity":            office,
		"robotics":               office,
		"geography":              office,
		"computerscience":        office,
		"math":                   office,
		"biology":                office,
		"physics":                office,
		"chemistry":              office,
		"electronics":            office,
		"geology":                office,
		"astronomy":              office,
		"science":                office,

		"development":    development,
		"debugger":       development,
		"ide":            development,
		"building":       development,
		"guidesigner":    development,
		"webdevelopment": development,
		"profiling":      development,
		"transiation":    development,

		"system":         system,
		"settings":       system,
		"monitor":        system,
		"dialup":         system,
		"packagemanager": system,
		"filesystem":     system,

		"utility":          system,
		"pda":              system,
		"accessibility":    system,
		"clock":            system,
		"calendar":         system,
		"calculator":       system,
		"documentation":    office,
		"archiving":        system,
		"compression":      system,
		"filemanager":      system,
		"filetools":        system,
		"terminalemulator": system,
		"texteditor":       office,
		"texttools":        office,
	}

	var extraXCategoryNameIdMap map[string]string = map[string]string{
		"internet":        internet,
		"videoconference": internet,

		"x-jack":           music,
		"x-alsa":           music,
		"x-multitrack":     music,
		"x-sound":          music,
		"cd":               music,
		"x-midi":           music,
		"x-sequencers":     music,
		"x-suse-sequencer": music,

		"boardgame":                       game,
		"cardgame":                        game,
		"x-debian-applications-emulators": game,
		"puzzlegame":                      game,
		"blocksgame":                      game,
		"x-suse-core-game":                game,

		"x-geeqie": graphics,

		"x-suse-core-office":           office,
		"x-mandrivalinux-office-other": office,
		"x-turbolinux-office":          office,

		"technical":                    others,
		"x-mandriva-office-publishing": others,

		"x-kde-edu-misc":     reading,
		"translation":        reading,
		"x-religion":         reading,
		"x-bible":            reading,
		"x-islamic-software": reading,
		"x-quran":            reading,
		"geoscience":         others,
		"meteorology":        others,

		"revisioncontrol": development,

		"trayicon":                    system,
		"x-lxde-settings":             system,
		"x-xfce-toplevel":             system,
		"x-xfcesettingsdialog":        system,
		"x-xfce":                      system,
		"x-kde-utilities-pim":         system,
		"x-kde-internet":              system,
		"x-kde-more":                  system,
		"x-kde-utilities-peripherals": system,
		"kde": system,
		"x-kde-utilities-file":                    system,
		"x-kde-utilities-desktop":                 system,
		"x-gnome-networksettings":                 system,
		"gnome":                                   system,
		"x-gnome-settings-panel":                  system,
		"x-gnome-personalsettings":                system,
		"x-gnome-systemsettings":                  system,
		"desktoputility":                          system,
		"x-misc":                                  system,
		"x-suse-core":                             system,
		"x-red-hat-base-only":                     system,
		"x-novell-main":                           system,
		"x-red-hat-extra":                         system,
		"x-suse-yast":                             system,
		"x-sun-supported":                         system,
		"x-suse-yast-high_availability":           system,
		"x-suse-controlcenter-lookandfeel":        system,
		"x-suse-controlcenter-system":             system,
		"x-red-hat-serverconfig":                  system,
		"x-mandrivalinux-system-archiving-backup": system,
		"x-suse-backup":                           system,
		"x-red-hat-base":                          system,
		"panel":                                   system,
		"x-gnustep":                               system,
		"x-bluetooth":                             system,
		"x-ximian-main":                           system,
		"x-synthesis":                             system,
		"x-digital_processing":                    system,
		"desktopsettings":                         system,
		"x-mandrivalinux-internet-other":          system,
		"systemsettings":                          system,
		"hardwaresettings":                        system,
		"advancedsettings":                        system,
		"x-enlightenment":                         system,
		"compiz":                                  system,

		"consoleonly": others,
		"core":        others,
		"favorites":   others,
		"pim":         others,
		"gpe":         others,
		"motif":       others,
		"applet":      others,
		"accessories": others,
		"wine":        others,
		"wine-programs-accessories": others,
		"playonlinux":               others,
		"screensaver":               others,
		"editors":                   others,
	}
	var data = make(map[string]string)
	for old, deepin := range xCategoryNameIdMap {
		data[old] = deepin
	}
	for old, deepin := range extraXCategoryNameIdMap {
		data[old] = deepin
	}
	return writeData(fpath, data)
}
