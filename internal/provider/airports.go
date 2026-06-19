package provider

// Hardcoded mapping of airport codes to city names for Indonesian airports.
var AirportCity = map[string]string{
	"CGK": "Jakarta",
	"HLP": "Jakarta",
	"DPS": "Denpasar",
	"SUB": "Surabaya",
	"UPG": "Makassar",
	"SOC": "Solo",
	"JOG": "Yogyakarta",
	"BDO": "Bandung",
	"MDC": "Manado",
	"BPN": "Balikpapan",
	"PLM": "Palembang",
	"PDG": "Padang",
	"PKU": "Pekanbaru",
	"KNO": "Medan",
	"BTH": "Batam",
	"SRG": "Semarang",
	"LOP": "Lombok",
	"AMQ": "Ambon",
	"DJJ": "Jayapura",
	"PNK": "Pontianak",
}

// CityForAirport returns the city name for a code, or the code itself if unknown.
func CityForAirport(code string) string {
	if city, ok := AirportCity[code]; ok {
		return city
	}
	return code
}
