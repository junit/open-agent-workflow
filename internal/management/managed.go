package management

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

func renderManagedFileWithoutBlock(current []byte) ([]byte, error) {
	view := installPathSnapshot{kind: installPathRegular, data: bytes.Clone(current)}
	status, _ := managedInstallStatus(view)
	if status != "present" {
		return nil, integrityError("managed markers are invalid")
	}
	lines := managedLineSpans(current)
	beginIndex, endIndex := -1, -1
	for index, line := range lines {
		switch string(current[line.start:line.contentEnd]) {
		case beginMarker:
			beginIndex = index
		case endMarker:
			endIndex = index
		}
	}
	result := make([]byte, 0, len(current))
	result = append(result, current[:lines[beginIndex].start]...)
	result = append(result, current[lines[endIndex].end:]...)
	return result, nil
}

const (
	beginMarker = "<!-- BEGIN OPEN AGENT WORKFLOW -->"
	endMarker   = "<!-- END OPEN AGENT WORKFLOW -->"
)

func managedBlock(path string) (string, string) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "absent", ""
		}
		return "drift", ""
	}
	if !info.Mode().IsRegular() {
		return "drift", ""
	}
	file, err := os.Open(path)
	if err != nil {
		return "drift", ""
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return "drift", ""
	}
	if !info.Mode().IsRegular() {
		return "drift", ""
	}
	reader := bufio.NewReader(file)
	beginCount, endCount := 0, 0
	beginLine, endLine := -1, -1
	lineIndex := 0
	copying := false
	var checksum posixChecksum
	for {
		line, found, readErr := readManagedLine(reader, copying, &checksum)
		if readErr != nil {
			return "drift", ""
		}
		if !found {
			break
		}
		wasCopying := copying
		switch line {
		case beginMarker:
			beginCount++
			if beginLine == -1 {
				beginLine = lineIndex
			}
			if !wasCopying {
				_, _ = checksum.Write([]byte(beginMarker + "\n"))
				copying = true
			}
		case endMarker:
			endCount++
			if endLine == -1 {
				endLine = lineIndex
			}
			if wasCopying {
				copying = false
			}
		}
		lineIndex++
	}
	if beginCount == 0 && endCount == 0 {
		return "absent", ""
	}
	if beginCount != 1 || endCount != 1 || beginLine >= endLine {
		return "drift", ""
	}
	return "present", checksum.String()
}

func readManagedLine(reader *bufio.Reader, copying bool, checksum *posixChecksum) (string, bool, error) {
	maximumMarkerLength := len(beginMarker)
	if len(endMarker) > maximumMarkerLength {
		maximumMarkerLength = len(endMarker)
	}
	candidate := make([]byte, 0, maximumMarkerLength)
	candidateValid := true
	found := false
	for {
		fragment, prefix, err := reader.ReadLine()
		if err != nil && err != io.EOF {
			return "", false, err
		}
		if len(fragment) != 0 || prefix || err != io.EOF {
			found = true
		}
		if candidateValid {
			if len(candidate)+len(fragment) <= maximumMarkerLength {
				candidate = append(candidate, fragment...)
			} else {
				candidateValid = false
			}
		}
		if copying {
			_, _ = checksum.Write(fragment)
		}
		if !prefix {
			if copying && found {
				_, _ = checksum.Write([]byte{'\n'})
			}
			if !found {
				return "", false, nil
			}
			if !candidateValid {
				return "", true, nil
			}
			return string(candidate), true, nil
		}
	}
}
