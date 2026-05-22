package encoding

import (
	"encoding/json"
	"fmt"

	"forge/internal/gateway/store"
)

// MarshalStringMap serialises a string→string map to JSON.
// Returns "{}" for nil or empty maps.
func MarshalStringMap(m map[string]string) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("store: marshal string map: %w", err)
	}
	return string(b), nil
}

// UnmarshalStringMap deserialises a JSON string to a string→string map.
func UnmarshalStringMap(raw string) (map[string]string, error) {
	if raw == "" || raw == "{}" || raw == "null" {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("store: unmarshal string map: %w", err)
	}
	return m, nil
}

// MarshalRefFiles serialises a RefFile slice to JSON.
func MarshalRefFiles(rf []store.RefFile) (string, error) {
	if len(rf) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(rf)
	if err != nil {
		return "", fmt.Errorf("store: marshal ref_files: %w", err)
	}
	return string(b), nil
}

// UnmarshalRefFiles deserialises a JSON string to a RefFile slice.
func UnmarshalRefFiles(raw string) ([]store.RefFile, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var rf []store.RefFile
	if err := json.Unmarshal([]byte(raw), &rf); err != nil {
		return nil, fmt.Errorf("store: unmarshal ref_files: %w", err)
	}
	return rf, nil
}

// MarshalPackageList serialises a PackageList to JSON.
func MarshalPackageList(p store.PackageList) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("store: marshal packages: %w", err)
	}
	return string(b), nil
}

// UnmarshalPackageList deserialises a JSON string to a PackageList.
func UnmarshalPackageList(raw string) (store.PackageList, error) {
	if raw == "" || raw == "{}" {
		return store.PackageList{}, nil
	}
	var p store.PackageList
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return store.PackageList{}, fmt.Errorf("store: unmarshal packages: %w", err)
	}
	return p, nil
}

// MarshalNetworking serialises a NetworkingConfig to JSON.
func MarshalNetworking(n store.NetworkingConfig) (string, error) {
	b, err := json.Marshal(n)
	if err != nil {
		return "", fmt.Errorf("store: marshal networking: %w", err)
	}
	return string(b), nil
}

// UnmarshalNetworking deserialises a JSON string to a NetworkingConfig.
func UnmarshalNetworking(raw string) (store.NetworkingConfig, error) {
	if raw == "" || raw == "{}" {
		return store.NetworkingConfig{}, nil
	}
	var n store.NetworkingConfig
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return store.NetworkingConfig{}, fmt.Errorf("store: unmarshal networking: %w", err)
	}
	return n, nil
}

// MarshalAny serialises any value to a JSON string.
func MarshalAny(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("store: marshal: %w", err)
	}
	return string(b), nil
}
