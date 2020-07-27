package disk

import (
	"testing"
)

func Test_GetRootMountInfo_1(t *testing.T) {
	var str = `{
		"blockdevices": [
		   {"name":"sdb", "serial":"68f9ds8fd7f8d52c8a2dj78fg79ss9c", "type":"disk", "size":15548554655, "vendor":"VMware, ", "model":"VMware_Virtual_S", "mountpoint":null, "uuid":null,
			  "children": [
				 {"name":"sda1", "serial":null, "type":"part", "size":1610612736, "vendor":null, "model":null, "mountpoint":"/", "uuid":"c41673e5-638f-4f3c-b52d-cd1667e024b3"},
				 {"name":"sda3", "serial":null, "type":"part", "size":2147483648, "vendor":null, "model":null, "mountpoint":"[SWAP]", "uuid":"b8604489-15fc-40e8-bd69-74bad4045624",
			 		"children": [
				 		{"name":"sda4", "serial":null, "type":"part1", "size":85894356591, "vendor":null, "model":null, "mountpoint":"/bin", "uuid":"dd52f15b-876a-4cb1-8eec-013c974c568a"}
			  		]
			 	 }
			  ]
		   },
		   {"name":"adc", "serial":"10c34x45c45155d024f55sd4a4ba0001", "type":"rom", "size":2226057216, "vendor":"NECVMWar", "model":"VMware_Virtual_IDE_CDROM_Drive", "mountpoint":"/media/kyrie/uos 20", "uuid":"2020-01-14-08-15-26-00"}
		]
	 }`
	info := []byte(str)
	disk, err := newDiskListFromOutput(info)
	if err != nil {
		t.Error("json格式不对")
	}
	for _, v := range disk {
		if v.RootMounted == true {
			println("Name:", v.Name)
			println("Size:", v.Size)
			println("Serial:", v.Serial)
			println("RootMounted:", v.RootMounted)
		}

	}
}

func Test_GetRootMountInfo_2(t *testing.T) {
	var str = `{
		"blockdevices": [
		   {"name":"sda", "serial":"6000c29bc5b8d52c8a280d5bea8c2959", "type":"disk", "size":128849018880, "vendor":"VMware, ", "model":"VMware_Virtual_S", "mountpoint":null, "uuid":null,
			  "children": [
				 {"name":"sda1", "serial":null, "type":"part", "size":1610612736, "vendor":null, "model":null, "mountpoint":"/boot", "uuid":"c41673e5-638f-4f3c-b52d-cd1667e024b3"},
				 {"name":"sda2", "serial":null, "type":"part", "size":85899345920, "vendor":null, "model":null, "mountpoint":"/root", "uuid":"dd52f15b-876a-4cb1-8eec-013c974c568a"},
				 {"name":"sda3", "serial":null, "type":"part", "size":2147483648, "vendor":null, "model":null, "mountpoint":"[SWAP]", "uuid":"b8604489-15fc-40e8-bd69-74bad4045624",
			 		"children": [
				 		{"name":"sda4", "serial":null, "type":"part1", "size":85894356591, "vendor":null, "model":null, "mountpoint":"/bin", "uuid":"dd52f15b-876a-4cb1-8eec-013c974c568a"},
				 		{"name":"sda5", "serial":null, "type":"part1", "size":855555555, "vendor":null, "model":null, "mountpoint":"/tmp", "uuid":"dd52f15b-876a-4cb1-8eec-013c974c568a",
				 			"children": [
					 			{"name":"sda6", "serial":null, "type":"part1", "size":85755451, "vendor":null, "model":null, "mountpoint":"/data", "uuid":"dd52f15b-876a-4cb1-8eec-013c974c568a",
					 				"children": [
							 			{"name":"sda7", "serial":null, "type":"part1", "size":258924521, "vendor":null, "model":null, "mountpoint":"/", "uuid":"dd52f15b-876a-4cb1-8eec-013c974c568a"}
					 				]
	 							}
							]
				 		}
			  		]
			 	}
			  ]
		   },
		   {"name":"adb", "serial":"10c34x45c45155d024f55sd4a4ba0001", "type":"rom", "size":2226057216, "vendor":"NECVMWar", "model":"VMware_Virtual_IDE_CDROM_Drive", "mountpoint":"/media/kyrie/uos 20", "uuid":"2020-01-14-08-15-26-00"}
		]
	 }`
	info := []byte(str)
	disk, err := newDiskListFromOutput(info)
	if err != nil {
		t.Error("json格式不对")
	}
	for _, v := range disk {
		if v.RootMounted == true {
			println("Name:", v.Name)
			println("Size:", v.Size)
			println("Serial:", v.Serial)
			println("RootMounted:", v.RootMounted)
		}

	}

}
