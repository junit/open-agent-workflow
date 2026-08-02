package check

import (
	"fmt"
	"io"
	"os"
)

const posixCRCPolynomial uint32 = 0x04c11db7

var posixCRCTable = buildPOSIXCRCTable()

func checksumBytes(data []byte) string {
	var checksum posixChecksum
	_, _ = checksum.Write(data)
	return checksum.String()
}

func checksumFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("checksum source is not a regular file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var checksum posixChecksum
	if _, err := io.Copy(&checksum, file); err != nil {
		return "", err
	}
	return checksum.String(), nil
}

type posixChecksum struct {
	crc    uint32
	length uint64
}

func (checksum *posixChecksum) Write(data []byte) (int, error) {
	for _, value := range data {
		checksum.crc = checksum.crc<<8 ^ posixCRCTable[byte(checksum.crc>>24)^value]
	}
	checksum.length += uint64(len(data))
	return len(data), nil
}

func (checksum posixChecksum) String() string {
	crc := checksum.crc
	for length := checksum.length; length != 0; length >>= 8 {
		crc = crc<<8 ^ posixCRCTable[byte(crc>>24)^byte(length)]
	}
	return fmt.Sprintf("%d:%d", ^crc, checksum.length)
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
