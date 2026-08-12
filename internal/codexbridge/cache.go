package codexbridge

import (
	"container/list"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

type EvidenceStore interface {
	Put(HookContext, Facts) (HostEvidenceHandle, error)
	Get(HostEvidenceHandle) (Facts, error)
	GetWithContext(HostEvidenceHandle) (Facts, HookContext, error)
	Reset()
}

type CacheOptions struct {
	Now            func() time.Time
	TTL            time.Duration
	MaximumEntries int
	Random         io.Reader
}

type evidenceStore struct {
	mu       sync.Mutex
	now      func() time.Time
	ttl      time.Duration
	maximum  int
	random   io.Reader
	sequence uint64
	entries  map[string]*list.Element
	lru      *list.List
}

type cacheEntry struct {
	handle    HostEvidenceHandle
	facts     Facts
	sessionID string
	cwd       string
	issued    time.Time
	expires   time.Time
}

func NewEvidenceStore(options CacheOptions) *evidenceStore {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	ttl := options.TTL
	if ttl < 30*time.Second {
		ttl = 30 * time.Second
	}
	if ttl > 15*time.Minute {
		ttl = 15 * time.Minute
	}
	maximum := options.MaximumEntries
	if maximum < 1 {
		maximum = 1
	}
	if maximum > 256 {
		maximum = 256
	}
	randomSource := options.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	return &evidenceStore{
		now: now, ttl: ttl, maximum: maximum, random: randomSource,
		entries: make(map[string]*list.Element), lru: list.New(),
	}
}

func (store *evidenceStore) Put(context HookContext, facts Facts) (HostEvidenceHandle, error) {
	if context.SchemaVersion != HookContextSchemaV2 || context.BridgeProtocolVersion != BridgeProtocolVersion {
		return HostEvidenceHandle{}, NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook context schema or Bridge protocol is unsupported", nil)
	}
	sessionDigest, cwdDigest, err := ContextDigestHeaders(context)
	if err != nil {
		return HostEvidenceHandle{}, err
	}
	if facts.Session.SessionID != context.SessionID {
		return HostEvidenceHandle{}, NewError("HOST_EVIDENCE_SESSION_MISMATCH", "Host facts belong to another session", nil)
	}
	if err := validateFacts(facts); err != nil {
		return HostEvidenceHandle{}, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	issued := store.now()
	token, err := store.newToken(context.SessionID, context.CWD)
	if err != nil {
		return HostEvidenceHandle{}, NewError("HOST_EVIDENCE_HANDLE_INVALID", "issue Host evidence handle", err)
	}
	handle := HostEvidenceHandle{Version: EvidenceHandleVersion, SessionDigest: sessionDigest, CWDDigest: cwdDigest, Token: token}
	entry := &cacheEntry{
		handle: handle, facts: cloneFacts(facts), sessionID: context.SessionID,
		cwd: canonicalPath(context.CWD), issued: issued, expires: issued.Add(store.ttl),
	}
	store.entries[token] = store.lru.PushFront(entry)
	for store.lru.Len() > store.maximum {
		store.removeElement(store.lru.Back())
	}
	return handle, nil
}

func (store *evidenceStore) Get(handle HostEvidenceHandle) (Facts, error) {
	facts, _, err := store.GetWithContext(handle)
	return facts, err
}

func (store *evidenceStore) GetWithContext(handle HostEvidenceHandle) (Facts, HookContext, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	element, found := store.entries[handle.Token]
	if !found || handle.Token == "" {
		return Facts{}, HookContext{}, NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host evidence handle is unknown", nil)
	}
	entry := element.Value.(*cacheEntry)
	if handle.Version != entry.handle.Version ||
		subtle.ConstantTimeCompare([]byte(handle.SessionDigest), []byte(digestHeader("session", entry.sessionID))) != 1 ||
		subtle.ConstantTimeCompare([]byte(handle.CWDDigest), []byte(digestHeader("cwd", entry.cwd))) != 1 {
		return Facts{}, HookContext{}, NewError("HOST_EVIDENCE_SESSION_MISMATCH", "handle headers do not match the cached Host context", nil)
	}
	now := store.now()
	if now.Before(entry.issued) || !now.Before(entry.expires) {
		store.removeElement(element)
		return Facts{}, HookContext{}, NewError("HOST_EVIDENCE_EXPIRED", "Host evidence handle has expired", nil)
	}
	store.lru.MoveToFront(element)
	return cloneFacts(entry.facts), HookContext{
		SchemaVersion: HookContextSchemaV2, BridgeProtocolVersion: BridgeProtocolVersion,
		SessionID: entry.sessionID, CWD: entry.cwd,
	}, nil
}

func (store *evidenceStore) Reset() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.entries = make(map[string]*list.Element)
	store.lru.Init()
}

func (store *evidenceStore) removeElement(element *list.Element) {
	if element == nil {
		return
	}
	entry := element.Value.(*cacheEntry)
	delete(store.entries, entry.handle.Token)
	store.lru.Remove(element)
}

func (store *evidenceStore) newToken(sessionID, cwd string) (string, error) {
	material := make([]byte, 24)
	if _, err := io.ReadFull(store.random, material[:16]); err != nil {
		return "", err
	}
	store.sequence++
	binary.BigEndian.PutUint64(material[16:], store.sequence)
	suffixInput := sha256.Sum256([]byte(sessionID + "\x00" + cwd + "\x00" + BridgeProtocolVersion))
	return "oawh1." + base64.RawURLEncoding.EncodeToString(material) + "." + hex.EncodeToString(suffixInput[:])[:16], nil
}

func ContextDigestHeaders(context HookContext) (string, string, error) {
	if !utf8.ValidString(context.SessionID) || context.SessionID == "" || len(context.SessionID) > 512 || strings.IndexFunc(context.SessionID, unicode.IsControl) >= 0 {
		return "", "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "invalid Codex session identity", nil)
	}
	cwd, err := canonicalCWD(context.CWD)
	if err != nil {
		return "", "", err
	}
	return digestHeader("session", context.SessionID), digestHeader("cwd", cwd), nil
}

func ValidateHandleContext(handle HostEvidenceHandle, context HookContext) error {
	if context.SchemaVersion != HookContextSchemaV2 || context.BridgeProtocolVersion != BridgeProtocolVersion {
		return NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "Hook context schema or Bridge protocol is unsupported", nil)
	}
	sessionDigest, cwdDigest, err := ContextDigestHeaders(context)
	if err != nil {
		return err
	}
	if handle.Version != EvidenceHandleVersion ||
		subtle.ConstantTimeCompare([]byte(handle.SessionDigest), []byte(sessionDigest)) != 1 ||
		subtle.ConstantTimeCompare([]byte(handle.CWDDigest), []byte(cwdDigest)) != 1 {
		return NewError("HOST_EVIDENCE_SESSION_MISMATCH", "handle headers do not match the current Host context", nil)
	}
	return nil
}

func digestHeader(kind, value string) string {
	digest := sha256.Sum256([]byte(EvidenceHandleVersion + "\x00" + kind + "\x00" + value))
	return hex.EncodeToString(digest[:])
}

func canonicalCWD(value string) (string, error) {
	if !utf8.ValidString(value) || value == "" || strings.IndexFunc(value, unicode.IsControl) >= 0 || !filepath.IsAbs(value) {
		return "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "cwd must be an absolute path without controls", nil)
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "canonicalize cwd", err)
	}
	clean := filepath.Clean(absolute)
	if !filepath.IsAbs(clean) {
		return "", NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "canonical cwd is not absolute", nil)
	}
	return clean, nil
}

func canonicalPath(value string) string {
	clean, err := canonicalCWD(value)
	if err != nil {
		return value
	}
	return clean
}

var _ EvidenceStore = (*evidenceStore)(nil)
