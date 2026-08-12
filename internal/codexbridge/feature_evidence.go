package codexbridge

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const (
	SessionFeatureEvidenceTTL       = 10 * time.Minute
	sessionFeatureEvidenceSchemaV1  = "oaw.codex-session-feature-evidence/v1"
	maximumSessionFeatureRecordSize = 16 << 10
)

type SessionFeatureEvidenceOptions struct {
	Root string
	TTL  time.Duration
	Now  func() time.Time
}

type FeatureEvidenceResult struct {
	Observations []host.FeatureObservation
	Diagnostics  []Diagnostic
}

type FeatureObserver interface {
	ObserveFeatures(HookContext) FeatureEvidenceResult
}

type SessionFeatureEvidenceStore struct {
	root string
	ttl  time.Duration
	now  func() time.Time
}

type sessionFeatureEvidenceRecord struct {
	SchemaVersion string         `json:"schema_version"`
	Feature       host.FeatureID `json:"feature"`
	SessionDigest string         `json:"session_digest"`
	CWDDigest     string         `json:"cwd_digest"`
	ObservedAt    int64          `json:"observed_at_unix"`
	ExpiresAt     int64          `json:"expires_at_unix"`
	Digest        string         `json:"digest"`
}

func NewSessionFeatureEvidenceStore(options SessionFeatureEvidenceOptions) (*SessionFeatureEvidenceStore, error) {
	if options.TTL == 0 {
		options.TTL = SessionFeatureEvidenceTTL
	}
	if !filepath.IsAbs(options.Root) || filepath.Clean(options.Root) != options.Root || options.TTL <= 0 || options.TTL > 30*time.Minute {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "session feature evidence options are invalid", nil)
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	store := &SessionFeatureEvidenceStore{root: options.Root, ttl: options.TTL, now: options.Now}
	root, err := store.openRoot(true)
	if err != nil {
		return nil, err
	}
	if err := root.Close(); err != nil {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "close session feature evidence root", err)
	}
	return store, nil
}

func (store *SessionFeatureEvidenceStore) RecordChildDelegation(context HookContext) error {
	sessionDigest, cwdDigest, err := ContextDigestHeaders(context)
	if err != nil {
		return err
	}
	now := store.now().UTC()
	record := sessionFeatureEvidenceRecord{
		SchemaVersion: sessionFeatureEvidenceSchemaV1, Feature: host.FeatureChildDelegation,
		SessionDigest: sessionDigest, CWDDigest: cwdDigest, ObservedAt: now.Unix(), ExpiresAt: now.Add(store.ttl).Unix(),
	}
	record.Digest, _, err = canonicaljson.Digest(record)
	if err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "digest session feature evidence", err)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "encode session feature evidence", err)
	}
	root, err := store.openRoot(true)
	if err != nil {
		return err
	}
	defer root.Close()
	name := featureEvidenceName(sessionDigest, cwdDigest)
	if info, inspectErr := root.Lstat(name); inspectErr == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "session feature evidence destination is unsafe", nil)
	} else if inspectErr != nil && !errors.Is(inspectErr, fs.ErrNotExist) {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "inspect session feature evidence destination", inspectErr)
	}
	return writeFeatureEvidence(root, name, encoded)
}

func (store *SessionFeatureEvidenceStore) ObserveFeatures(context HookContext) FeatureEvidenceResult {
	sessionDigest, cwdDigest, err := ContextDigestHeaders(context)
	if err != nil {
		return invalidFeatureEvidenceResult()
	}
	root, err := store.openRoot(false)
	if errors.Is(err, fs.ErrNotExist) {
		return FeatureEvidenceResult{}
	}
	if err != nil {
		return invalidFeatureEvidenceResult()
	}
	defer root.Close()
	name := featureEvidenceName(sessionDigest, cwdDigest)
	record, state := readFeatureEvidence(root, name)
	if state == featureEvidenceMissing {
		return FeatureEvidenceResult{}
	}
	if state != featureEvidenceValid || !store.validRecord(record, sessionDigest, cwdDigest) {
		return invalidFeatureEvidenceResult()
	}
	if store.now().UTC().Unix() >= record.ExpiresAt {
		_ = root.Remove(name)
		return FeatureEvidenceResult{}
	}
	observation, err := host.NewFeatureObservation(host.FeatureObservation{
		Feature: host.FeatureChildDelegation, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/cooperative-subagent-start/" + record.Digest,
	})
	if err != nil {
		return invalidFeatureEvidenceResult()
	}
	return FeatureEvidenceResult{Observations: []host.FeatureObservation{observation}}
}

func (store *SessionFeatureEvidenceStore) validRecord(record sessionFeatureEvidenceRecord, sessionDigest, cwdDigest string) bool {
	provided := record.Digest
	record.Digest = ""
	digest, _, err := canonicaljson.Digest(record)
	return err == nil && record.SchemaVersion == sessionFeatureEvidenceSchemaV1 && record.Feature == host.FeatureChildDelegation &&
		subtle.ConstantTimeCompare([]byte(record.SessionDigest), []byte(sessionDigest)) == 1 &&
		subtle.ConstantTimeCompare([]byte(record.CWDDigest), []byte(cwdDigest)) == 1 &&
		record.ObservedAt > 0 && record.ExpiresAt > record.ObservedAt && record.ExpiresAt-record.ObservedAt <= int64(store.ttl/time.Second) &&
		record.ObservedAt <= store.now().UTC().Add(time.Minute).Unix() && subtle.ConstantTimeCompare([]byte(provided), []byte(digest)) == 1
}

func (store *SessionFeatureEvidenceStore) evidencePath(context HookContext) (string, error) {
	sessionDigest, cwdDigest, err := ContextDigestHeaders(context)
	if err != nil {
		return "", err
	}
	return filepath.Join(store.root, featureEvidenceName(sessionDigest, cwdDigest)), nil
}

func featureEvidenceName(sessionDigest, cwdDigest string) string {
	return "child-" + sessionDigest + "-" + cwdDigest + ".json"
}

type featureEvidenceReadState uint8

const (
	featureEvidenceMissing featureEvidenceReadState = iota
	featureEvidenceValid
	featureEvidenceInvalid
)

func readFeatureEvidence(root *os.Root, name string) (sessionFeatureEvidenceRecord, featureEvidenceReadState) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return sessionFeatureEvidenceRecord{}, featureEvidenceMissing
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() < 1 || info.Size() > maximumSessionFeatureRecordSize {
		return sessionFeatureEvidenceRecord{}, featureEvidenceInvalid
	}
	file, err := root.Open(name)
	if err != nil {
		return sessionFeatureEvidenceRecord{}, featureEvidenceInvalid
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return sessionFeatureEvidenceRecord{}, featureEvidenceInvalid
	}
	decoder := json.NewDecoder(io.LimitReader(file, maximumSessionFeatureRecordSize+1))
	decoder.DisallowUnknownFields()
	var record sessionFeatureEvidenceRecord
	if err := decoder.Decode(&record); err != nil {
		return sessionFeatureEvidenceRecord{}, featureEvidenceInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return sessionFeatureEvidenceRecord{}, featureEvidenceInvalid
	}
	return record, featureEvidenceValid
}

func writeFeatureEvidence(root *os.Root, name string, encoded []byte) error {
	var random [8]byte
	if _, err := io.ReadFull(rand.Reader, random[:]); err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "create session feature evidence temporary name", err)
	}
	temporaryName := ".oaw-feature-" + hex.EncodeToString(random[:])
	temporary, err := root.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "create temporary session feature evidence", err)
	}
	keep := true
	defer func() {
		_ = temporary.Close()
		if keep {
			_ = root.Remove(temporaryName)
		}
	}()
	if _, err := temporary.Write(encoded); err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "write session feature evidence", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "restrict session feature evidence", err)
	}
	if err := temporary.Sync(); err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "sync session feature evidence", err)
	}
	if err := temporary.Close(); err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "close session feature evidence", err)
	}
	if err := root.Rename(temporaryName, name); err != nil {
		return NewError("HOST_BRIDGE_UNAVAILABLE", "publish session feature evidence", err)
	}
	keep = false
	return nil
}

func (store *SessionFeatureEvidenceStore) openRoot(create bool) (*os.Root, error) {
	base, relative, err := featureRootCoordinates(store.root)
	if err != nil {
		return nil, err
	}
	if create {
		if err := os.MkdirAll(base, 0o700); err != nil {
			return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "create session feature XDG root", err)
		}
	}
	baseRoot, err := openFeatureBaseRoot(base)
	if err != nil {
		return nil, err
	}
	defer baseRoot.Close()
	components := strings.Split(relative, "/")
	consumed := ""
	for index, component := range components {
		consumed = path.Join(consumed, component)
		info, inspectErr := baseRoot.Lstat(consumed)
		if create && errors.Is(inspectErr, fs.ErrNotExist) {
			if err := baseRoot.Mkdir(consumed, 0o700); err != nil {
				return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "create session feature directory", err)
			}
			info, inspectErr = baseRoot.Lstat(consumed)
		}
		if inspectErr != nil {
			return nil, inspectErr
		}
		privateRoot := index == len(components)-1
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() ||
			(!privateRoot && info.Mode().Perm()&0o022 != 0) ||
			(privateRoot && info.Mode().Perm() != 0o700) {
			return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "session feature evidence directory is unsafe", nil)
		}
	}
	info, err := baseRoot.Lstat(relative)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "session feature evidence root is unsafe", err)
	}
	root, err := baseRoot.OpenRoot(relative)
	if err != nil {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "open session feature evidence root", err)
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		root.Close()
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "session feature evidence root changed while opening", err)
	}
	return root, nil
}

func featureRootCoordinates(root string) (string, string, error) {
	base := filepath.Dir(filepath.Dir(filepath.Dir(root)))
	relative, err := filepath.Rel(base, root)
	if err != nil || filepath.ToSlash(relative) != "open-agent-workflow/codex-bridge/features" {
		return "", "", NewError("HOST_BRIDGE_UNAVAILABLE", "session feature evidence root has an invalid layout", err)
	}
	return base, filepath.ToSlash(relative), nil
}

func openFeatureBaseRoot(base string) (*os.Root, error) {
	info, err := os.Lstat(base)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "session feature XDG root is unsafe", nil)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "open session feature XDG root", err)
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		root.Close()
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "session feature XDG root changed while opening", err)
	}
	return root, nil
}

func invalidFeatureEvidenceResult() FeatureEvidenceResult {
	return FeatureEvidenceResult{Diagnostics: []Diagnostic{
		NewDiagnostic("HOST_OBSERVATION_PARTIAL", "evidence", "live child-delegation evidence was unavailable", true),
	}}
}

var _ FeatureObserver = (*SessionFeatureEvidenceStore)(nil)
