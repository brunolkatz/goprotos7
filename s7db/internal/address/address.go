package address

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	fullAddressRE  = regexp.MustCompile(`^DB(\d+)\.(S|STRING|DB[SXBWDbwxd])(\d+)(?:\.(\d+))?$`)
	shortAddressRE = regexp.MustCompile(`^(S|STRING|[SXBWDbwxd])(\d+)(?:\.(\d+))?$`)
)

type Address struct {
	DB   int
	Area string
	Byte int
	Bit  int
}

func (a Address) Canonical() string {
	if a.Area == "X" || a.Area == "DBX" {
		return fmt.Sprintf("DB%d.DBX%d.%d", a.DB, a.Byte, a.Bit)
	}
	if a.Area == "S" || a.Area == "STRING" || a.Area == "DBS" {
		return fmt.Sprintf("DB%d.%s%d.%d", a.DB, a.Area, a.Byte, a.Bit)
	}
	return fmt.Sprintf("DB%d.%s%d", a.DB, a.Area, a.Byte)
}

func (a Address) CanonicalShort() string {
	if a.Area == "X" {
		return fmt.Sprintf("X%d.%d", a.Byte, a.Bit)
	}
	return fmt.Sprintf("%s%d", a.Area, a.Byte)
}

func (a Address) StartBit() int {
	if a.Area == "S" || a.Area == "STRING" || a.Area == "DBS" {
		return a.Byte * 8
	}
	return a.Byte*8 + a.Bit
}

func Parse(input string, defaultDB *int) (Address, error) {
	in := strings.TrimSpace(input)
	if in == "" {
		return Address{}, fmt.Errorf("empty address")
	}
	if m := fullAddressRE.FindStringSubmatch(in); m != nil {
		return fromParts(m[1], m[2], m[3], m[4], false)
	}
	if m := shortAddressRE.FindStringSubmatch(in); m != nil {
		if defaultDB == nil {
			return Address{}, fmt.Errorf("short address %q requires default DB (--db)", input)
		}
		addr, err := fromParts(strconv.Itoa(*defaultDB), m[1], m[2], m[3], true)
		if err != nil {
			return Address{}, err
		}
		return addr, nil
	}
	return Address{}, fmt.Errorf("invalid address %q", input)
}

func fromParts(dbRaw, areaRaw, byteRaw, bitRaw string, short bool) (Address, error) {
	db, err := strconv.Atoi(dbRaw)
	if err != nil || db < 0 {
		return Address{}, fmt.Errorf("invalid DB number %q", dbRaw)
	}
	b, err := strconv.Atoi(byteRaw)
	if err != nil || b < 0 {
		return Address{}, fmt.Errorf("invalid byte offset %q - byteRaw - parts: %v", byteRaw, []string{dbRaw, areaRaw, byteRaw, bitRaw})
	}
	area := strings.ToUpper(areaRaw)
	addr := Address{DB: db, Area: area, Byte: b}
	if area == "STRING" || area == "S" || area == "DBS" {
		bit, err := strconv.Atoi(bitRaw)
		if err != nil || bit < 0 || bit > 254 {
			return Address{}, fmt.Errorf("invalid bit offset %q", bitRaw)
		}
		addr.Bit = bit
		return addr, nil
	}
	if area == "X" || area == "DBX" {
		if bitRaw == "" {
			return Address{}, fmt.Errorf("bit index is required for BOOL address")
		}
		bit, err := strconv.Atoi(bitRaw)
		if err != nil || bit < 0 || bit > 7 {
			return Address{}, fmt.Errorf("invalid bit offset %q", bitRaw)
		}
		addr.Bit = bit
		return addr, nil
	}
	if bitRaw != "" {
		if short {
			return Address{}, fmt.Errorf("bit offset is only valid for X addresses")
		}
		return Address{}, fmt.Errorf("bit offset is only valid for DBX addresses")
	}
	return addr, nil
}
