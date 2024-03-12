package utils

var (
	continents = map[string]string{
		"亚洲":  "AP",
		"大洋洲": "OA",
		"北美洲": "NA",
		"南美洲": "LA",
		"欧洲":  "EU",
		"非洲":  "AF",
		"南极洲": "AQ",
	}
)

func GetContinentCodeByName(name string) string {
	if v, ok := continents[name]; ok {
		return v
	}
	return name
}
