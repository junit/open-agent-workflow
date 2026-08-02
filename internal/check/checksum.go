package check

import (
	"fmt"
	"os"
)

const posixCRCPolynomial uint32 = 0x04c11db7

var posixCRCTable = buildPOSIXCRCTable()

func checksumBytes(data []byte) string {
	var crc uint32
	for _, value := range data {
		crc = crc<<8 ^ posixCRCTable[byte(crc>>24)^value]
	}
	for length := uint64(len(data)); length != 0; length >>= 8 {
		crc = crc<<8 ^ posixCRCTable[byte(crc>>24)^byte(length)]
	}
	return fmt.Sprintf("%d:%d", ^crc, len(data))
}

func checksumFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return checksumBytes(data), nil
}

func buildPOSIXCRCTable() [256]uint32 {
	var table [256]uint32
	for index := range table {
		value := uint32(index) << 24
		for range 8 {
			if value&0x80000000 != 0 {
				value = value<<1 ^ posixCRCPolynomial
			} else {
				value <<= 1
			}
		}
		table[index] = value
	}
	return table
}
