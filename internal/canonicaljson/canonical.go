package canonicaljson

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
)

func Marshal(value any) ([]byte, error) {
	if err := validateValue(reflect.ValueOf(value), make(map[visit]struct{})); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("CANONICAL_JSON_ENCODE_FAILED: %w", err)
	}
	return encoded, nil
}

func Digest(value any) (string, []byte, error) {
	encoded, err := Marshal(value)
	if err != nil {
		return "", nil, err
	}
	return DigestBytes(encoded), encoded, nil
}

func DigestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

type visit struct {
	typeOf reflect.Type
	ptr    uintptr
}

func validateValue(value reflect.Value, seen map[visit]struct{}) error {
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return validateValue(value.Elem(), seen)
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		key := visit{typeOf: value.Type(), ptr: value.Pointer()}
		if _, exists := seen[key]; exists {
			return unsupported(value, "cyclic pointer")
		}
		seen[key] = struct{}{}
		defer delete(seen, key)
		return validateValue(value.Elem(), seen)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Type().Field(i)
			if field.PkgPath != "" || field.Tag.Get("json") == "-" {
				continue
			}
			if err := validateValue(value.Field(i), seen); err != nil {
				return err
			}
		}
		return nil
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := validateValue(value.Index(i), seen); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if value.IsNil() {
			return nil
		}
		key := visit{typeOf: value.Type(), ptr: value.Pointer()}
		if _, exists := seen[key]; exists {
			return unsupported(value, "cyclic slice")
		}
		seen[key] = struct{}{}
		defer delete(seen, key)
		for i := 0; i < value.Len(); i++ {
			if err := validateValue(value.Index(i), seen); err != nil {
				return err
			}
		}
		return nil
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return nil
	default:
		return unsupported(value, value.Kind().String())
	}
}

func unsupported(value reflect.Value, reason string) error {
	return fmt.Errorf("CANONICAL_JSON_UNSUPPORTED: %s (%s)", value.Type(), reason)
}
