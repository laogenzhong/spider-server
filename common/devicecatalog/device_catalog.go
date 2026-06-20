package devicecatalog

import "strings"

// DisplayLabel returns a human-readable Apple device name while preserving the
// raw hardware identifier for debugging and future-proofing.
func DisplayLabel(identifier string) string {
	id := strings.TrimSpace(identifier)
	if id == "" {
		return ""
	}
	name := DisplayName(id)
	if name == id {
		return id
	}
	return name + " (" + id + ")"
}

// DisplayName returns the known Apple device name for a hardware identifier.
// Unknown identifiers are returned unchanged.
func DisplayName(identifier string) string {
	id := strings.TrimSpace(identifier)
	if id == "" {
		return ""
	}
	if name, ok := appleDeviceNames[id]; ok {
		return name
	}
	return id
}

var appleDeviceNames = buildAppleDeviceNames()

func buildAppleDeviceNames() map[string]string {
	names := map[string]string{
		"i386":   "Simulator",
		"x86_64": "Simulator",
		"arm64":  "Simulator",
	}

	add := func(name string, ids ...string) {
		for _, id := range ids {
			names[id] = name
		}
	}

	add("iPhone", "iPhone1,1")
	add("iPhone 3G", "iPhone1,2")
	add("iPhone 3GS", "iPhone2,1")
	add("iPhone 4", "iPhone3,1", "iPhone3,2", "iPhone3,3")
	add("iPhone 4s", "iPhone4,1")
	add("iPhone 5", "iPhone5,1", "iPhone5,2")
	add("iPhone 5c", "iPhone5,3", "iPhone5,4")
	add("iPhone 5s", "iPhone6,1", "iPhone6,2")
	add("iPhone 6", "iPhone7,2")
	add("iPhone 6 Plus", "iPhone7,1")
	add("iPhone 6s", "iPhone8,1")
	add("iPhone 6s Plus", "iPhone8,2")
	add("iPhone SE (1st generation)", "iPhone8,4")
	add("iPhone 7", "iPhone9,1", "iPhone9,3")
	add("iPhone 7 Plus", "iPhone9,2", "iPhone9,4")
	add("iPhone 8", "iPhone10,1", "iPhone10,4")
	add("iPhone 8 Plus", "iPhone10,2", "iPhone10,5")
	add("iPhone X", "iPhone10,3", "iPhone10,6")
	add("iPhone XR", "iPhone11,8")
	add("iPhone XS", "iPhone11,2")
	add("iPhone XS Max", "iPhone11,4", "iPhone11,6")
	add("iPhone 11", "iPhone12,1")
	add("iPhone 11 Pro", "iPhone12,3")
	add("iPhone 11 Pro Max", "iPhone12,5")
	add("iPhone SE (2nd generation)", "iPhone12,8")
	add("iPhone 12 mini", "iPhone13,1")
	add("iPhone 12", "iPhone13,2")
	add("iPhone 12 Pro", "iPhone13,3")
	add("iPhone 12 Pro Max", "iPhone13,4")
	add("iPhone 13 Pro", "iPhone14,2")
	add("iPhone 13 Pro Max", "iPhone14,3")
	add("iPhone 13 mini", "iPhone14,4")
	add("iPhone 13", "iPhone14,5")
	add("iPhone SE (3rd generation)", "iPhone14,6")
	add("iPhone 14", "iPhone14,7")
	add("iPhone 14 Plus", "iPhone14,8")
	add("iPhone 14 Pro", "iPhone15,2")
	add("iPhone 14 Pro Max", "iPhone15,3")
	add("iPhone 15", "iPhone15,4")
	add("iPhone 15 Plus", "iPhone15,5")
	add("iPhone 15 Pro", "iPhone16,1")
	add("iPhone 15 Pro Max", "iPhone16,2")
	add("iPhone 16 Pro", "iPhone17,1")
	add("iPhone 16 Pro Max", "iPhone17,2")
	add("iPhone 16", "iPhone17,3")
	add("iPhone 16 Plus", "iPhone17,4")
	add("iPhone 16e", "iPhone17,5")
	add("iPhone 17 Pro", "iPhone18,1")
	add("iPhone 17 Pro Max", "iPhone18,2")
	add("iPhone 17", "iPhone18,3")
	add("iPhone Air", "iPhone18,4")
	add("iPhone 17e", "iPhone18,5")

	add("iPod touch", "iPod1,1")
	add("iPod touch (2nd generation)", "iPod2,1")
	add("iPod touch (3rd generation)", "iPod3,1")
	add("iPod touch (4th generation)", "iPod4,1")
	add("iPod touch (5th generation)", "iPod5,1")
	add("iPod touch (6th generation)", "iPod7,1")
	add("iPod touch (7th generation)", "iPod9,1")

	add("iPad", "iPad1,1")
	add("iPad 2", "iPad2,1", "iPad2,2", "iPad2,3", "iPad2,4")
	add("iPad (3rd generation)", "iPad3,1", "iPad3,2", "iPad3,3")
	add("iPad (4th generation)", "iPad3,4", "iPad3,5", "iPad3,6")
	add("iPad (5th generation)", "iPad6,11", "iPad6,12")
	add("iPad (6th generation)", "iPad7,5", "iPad7,6")
	add("iPad (7th generation)", "iPad7,11", "iPad7,12")
	add("iPad (8th generation)", "iPad11,6", "iPad11,7")
	add("iPad (9th generation)", "iPad12,1", "iPad12,2")
	add("iPad (10th generation)", "iPad13,18", "iPad13,19")
	add("iPad (A16)", "iPad15,7", "iPad15,8")

	add("iPad Air", "iPad4,1", "iPad4,2", "iPad4,3")
	add("iPad Air 2", "iPad5,3", "iPad5,4")
	add("iPad Air (3rd generation)", "iPad11,3", "iPad11,4")
	add("iPad Air (4th generation)", "iPad13,1", "iPad13,2")
	add("iPad Air (5th generation)", "iPad13,16", "iPad13,17")
	add("iPad Air 11-inch (M2)", "iPad14,8", "iPad14,9")
	add("iPad Air 13-inch (M2)", "iPad14,10", "iPad14,11")
	add("iPad Air 11-inch (M3)", "iPad15,3", "iPad15,4")
	add("iPad Air 13-inch (M3)", "iPad15,5", "iPad15,6")
	add("iPad Air 11-inch (M4)", "iPad16,8", "iPad16,9")
	add("iPad Air 13-inch (M4)", "iPad16,10", "iPad16,11")

	add("iPad mini", "iPad2,5", "iPad2,6", "iPad2,7")
	add("iPad mini 2", "iPad4,4", "iPad4,5", "iPad4,6")
	add("iPad mini 3", "iPad4,7", "iPad4,8", "iPad4,9")
	add("iPad mini 4", "iPad5,1", "iPad5,2")
	add("iPad mini (5th generation)", "iPad11,1", "iPad11,2")
	add("iPad mini (6th generation)", "iPad14,1", "iPad14,2")
	add("iPad mini (A17 Pro)", "iPad16,1", "iPad16,2")

	add("iPad Pro 9.7-inch", "iPad6,3", "iPad6,4")
	add("iPad Pro 10.5-inch", "iPad7,3", "iPad7,4")
	add("iPad Pro 11-inch (1st generation)", "iPad8,1", "iPad8,2", "iPad8,3", "iPad8,4")
	add("iPad Pro 11-inch (2nd generation)", "iPad8,9", "iPad8,10")
	add("iPad Pro 11-inch (3rd generation)", "iPad13,4", "iPad13,5", "iPad13,6", "iPad13,7")
	add("iPad Pro 11-inch (4th generation)", "iPad14,3", "iPad14,4")
	add("iPad Pro 11-inch (M4)", "iPad16,3", "iPad16,4")
	add("iPad Pro 11-inch (M5)", "iPad17,1", "iPad17,2")
	add("iPad Pro 12.9-inch (1st generation)", "iPad6,7", "iPad6,8")
	add("iPad Pro 12.9-inch (2nd generation)", "iPad7,1", "iPad7,2")
	add("iPad Pro 12.9-inch (3rd generation)", "iPad8,5", "iPad8,6", "iPad8,7", "iPad8,8")
	add("iPad Pro 12.9-inch (4th generation)", "iPad8,11", "iPad8,12")
	add("iPad Pro 12.9-inch (5th generation)", "iPad13,8", "iPad13,9", "iPad13,10", "iPad13,11")
	add("iPad Pro 12.9-inch (6th generation)", "iPad14,5", "iPad14,6")
	add("iPad Pro 13-inch (M4)", "iPad16,5", "iPad16,6")
	add("iPad Pro 13-inch (M5)", "iPad17,3", "iPad17,4")

	add("Apple Watch (1st generation) 38mm", "Watch1,1")
	add("Apple Watch (1st generation) 42mm", "Watch1,2")
	add("Apple Watch Series 1 38mm", "Watch2,6")
	add("Apple Watch Series 1 42mm", "Watch2,7")
	add("Apple Watch Series 2 38mm", "Watch2,3")
	add("Apple Watch Series 2 42mm", "Watch2,4")
	add("Apple Watch Series 2 Cellular 38mm (unreleased)", "Watch2,1")
	add("Apple Watch Series 2 Cellular 42mm (unreleased)", "Watch2,2")
	add("Apple Watch Series 3 38mm GPS + Cellular", "Watch3,1")
	add("Apple Watch Series 3 42mm GPS + Cellular", "Watch3,2")
	add("Apple Watch Series 3 38mm GPS", "Watch3,3")
	add("Apple Watch Series 3 42mm GPS", "Watch3,4")
	add("Apple Watch Series 4 40mm GPS", "Watch4,1")
	add("Apple Watch Series 4 44mm GPS", "Watch4,2")
	add("Apple Watch Series 4 40mm GPS + Cellular", "Watch4,3")
	add("Apple Watch Series 4 44mm GPS + Cellular", "Watch4,4")
	add("Apple Watch Series 5 40mm GPS", "Watch5,1")
	add("Apple Watch Series 5 44mm GPS", "Watch5,2")
	add("Apple Watch Series 5 40mm GPS + Cellular", "Watch5,3")
	add("Apple Watch Series 5 44mm GPS + Cellular", "Watch5,4")
	add("Apple Watch SE (1st generation) 40mm GPS", "Watch5,9")
	add("Apple Watch SE (1st generation) 44mm GPS", "Watch5,10")
	add("Apple Watch SE (1st generation) 40mm GPS + Cellular", "Watch5,11")
	add("Apple Watch SE (1st generation) 44mm GPS + Cellular", "Watch5,12")
	add("Apple Watch Series 6 40mm GPS", "Watch6,1")
	add("Apple Watch Series 6 44mm GPS", "Watch6,2")
	add("Apple Watch Series 6 40mm GPS + Cellular", "Watch6,3")
	add("Apple Watch Series 6 44mm GPS + Cellular", "Watch6,4")
	add("Apple Watch Series 7 41mm GPS", "Watch6,6")
	add("Apple Watch Series 7 45mm GPS", "Watch6,7")
	add("Apple Watch Series 7 41mm GPS + Cellular", "Watch6,8")
	add("Apple Watch Series 7 45mm GPS + Cellular", "Watch6,9")
	add("Apple Watch SE (2nd generation) 40mm GPS", "Watch6,10")
	add("Apple Watch SE (2nd generation) 44mm GPS", "Watch6,11")
	add("Apple Watch SE (2nd generation) 40mm GPS + Cellular", "Watch6,12")
	add("Apple Watch SE (2nd generation) 44mm GPS + Cellular", "Watch6,13")
	add("Apple Watch Series 8 41mm GPS", "Watch6,14")
	add("Apple Watch Series 8 45mm GPS", "Watch6,15")
	add("Apple Watch Series 8 41mm GPS + Cellular", "Watch6,16")
	add("Apple Watch Series 8 45mm GPS + Cellular", "Watch6,17")
	add("Apple Watch Ultra 49mm", "Watch6,18")
	add("Apple Watch Series 9 41mm GPS", "Watch7,1")
	add("Apple Watch Series 9 45mm GPS", "Watch7,2")
	add("Apple Watch Series 9 41mm GPS + Cellular", "Watch7,3")
	add("Apple Watch Series 9 45mm GPS + Cellular", "Watch7,4")
	add("Apple Watch Ultra 2 49mm", "Watch7,5")
	add("Apple Watch Series 10 42mm GPS", "Watch7,8")
	add("Apple Watch Series 10 46mm GPS", "Watch7,9")
	add("Apple Watch Series 10 42mm GPS + Cellular", "Watch7,10")
	add("Apple Watch Series 10 46mm GPS + Cellular", "Watch7,11")
	add("Apple Watch Ultra 3 49mm", "Watch7,12")
	add("Apple Watch SE 3 40mm GPS", "Watch7,13")
	add("Apple Watch SE 3 44mm GPS", "Watch7,14")
	add("Apple Watch SE 3 40mm GPS + Cellular", "Watch7,15")
	add("Apple Watch SE 3 44mm GPS + Cellular", "Watch7,16")
	add("Apple Watch Series 11 42mm GPS", "Watch7,17")
	add("Apple Watch Series 11 46mm GPS", "Watch7,18")
	add("Apple Watch Series 11 42mm GPS + Cellular", "Watch7,19")
	add("Apple Watch Series 11 46mm GPS + Cellular", "Watch7,20")

	add("Apple TV", "AppleTV1,1")
	add("Apple TV (2nd generation)", "AppleTV2,1")
	add("Apple TV (3rd generation)", "AppleTV3,1", "AppleTV3,2")
	add("Apple TV HD", "AppleTV5,3")
	add("Apple TV 4K", "AppleTV6,2")
	add("Apple TV 4K (2nd generation)", "AppleTV11,1")
	add("Apple TV 4K (3rd generation)", "AppleTV14,1")

	return names
}
