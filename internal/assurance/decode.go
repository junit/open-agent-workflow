package assurance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const maximumAssuranceInputBytes = 4 << 20

func DecodeIssueRequest(raw []byte) (IssueRequest, error) {
	var request IssueRequest
	if err := decodeStrict(raw, &request); err != nil {
		return IssueRequest{}, assuranceError("ASSURANCE_INPUT_INVALID", "decode Issue Request", err)
	}
	if err := validateIssueRequestShape(request); err != nil {
		return IssueRequest{}, err
	}
	request.Claims = cloneClaims(request.Claims)
	return request, nil
}

func DecodeOverlay(raw []byte) (Overlay, error) {
	var overlay Overlay
	if err := decodeStrict(raw, &overlay); err != nil {
		return Overlay{}, assuranceError("OVERLAY_INVALID", "decode Assurance Overlay", err)
	}
	overlay.Claims = cloneClaims(overlay.Claims)
	return overlay, nil
}

func decodeStrict(raw []byte, target any) error {
	if len(raw) == 0 {
		return errors.New("input is empty")
	}
	if len(raw) > maximumAssuranceInputBytes {
		return fmt.Errorf("input exceeds %d bytes", maximumAssuranceInputBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("input contains multiple JSON values")
		}
		return err
	}
	return nil
}
