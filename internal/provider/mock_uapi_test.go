package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// mockUAPI is an in-process fake of the uapi curated API, good enough to drive
// the provider end to end (CRUD, adopt, ETag/If-Match, singletons, a couple of
// non-uci specials) without a real router. It is JSON-native, mirroring what
// uapi's fromUci emits (real booleans, arrays), not uci's "1"/"0" strings.
type mockUAPI struct {
	*httptest.Server
	mu sync.Mutex
	// store[collection][id] = object
	store   map[string]map[string]map[string]any
	counter int
	// runtime injected on GET for these collections
	runtime map[string]map[string]any
}

func newMockUAPI() *mockUAPI {
	m := &mockUAPI{
		store:   map[string]map[string]map[string]any{},
		runtime: map[string]map[string]any{},
	}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

func etagOf(obj map[string]any) string {
	cp := map[string]any{}
	for k, v := range obj {
		if k == "runtime" {
			continue
		}
		cp[k] = v
	}
	b, _ := json.Marshal(cp) // Go sorts map keys: deterministic
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:])[:12] + `"`
}

func (m *mockUAPI) ifMatch(r *http.Request) string {
	if v := r.Header.Get("If-Match"); v != "" {
		return v
	}
	return r.URL.Query().Get("if_match")
}

func writeJSON(w http.ResponseWriter, status int, body map[string]any, etag string) {
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func apiErr(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"code": code, "message": code}, "")
}

// seedUnmanaged inserts a pre-existing anonymous section (managed=false) for adopt tests.
func (m *mockUAPI) seedUnmanaged(coll, id string, obj map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store[coll] == nil {
		m.store[coll] = map[string]map[string]any{}
	}
	obj["id"] = id
	obj["managed"] = false
	m.store[coll][id] = obj
}

func (m *mockUAPI) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := r.URL.Path

	// Specials.
	switch {
	case path == "/system/password" && r.Method == http.MethodPost:
		w.WriteHeader(http.StatusNoContent)
		return
	case path == "/dhcp/leases" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"mac": "aa:bb:cc:dd:ee:ff", "ip": "192.168.1.50", "hostname": "nuc", "expires_at": 1893456000, "duid": nil},
		})
		return
	case path == "/dhcp/leases6" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"duid": "00010203", "iaid": "abcd", "ia_type": "IA_PD", "ip": "2001:db8::", "prefix_length": 60, "expires_at": 1893456000},
		})
		return
	case path == "/system/authorized_keys":
		m.handleAuthKeys(w, r, "")
		return
	case strings.HasPrefix(path, "/system/authorized_keys/"):
		m.handleAuthKeys(w, r, strings.TrimPrefix(path, "/system/authorized_keys/"))
		return
	}

	// Generic routing by path shape + method. The provider's access patterns are
	// disjoint: it only GET/PATCHes a 2-segment path for singletons, only POSTs a
	// 2-segment path to create in a collection, and uses 3 segments for items.
	segs := strings.Split(strings.Trim(path, "/"), "/")
	switch len(segs) {
	case 1: // /system
		m.handleSingleton(w, r, path)
	case 2: // /a/b
		if r.Method == http.MethodPost {
			m.handleCollectionCreate(w, r, path)
		} else {
			m.handleSingleton(w, r, path)
		}
	case 3: // /a/b/c
		m.handleItem(w, r, "/"+segs[0]+"/"+segs[1], segs[2])
	case 4: // /a/b/c/adopt
		if segs[3] == "adopt" && r.Method == http.MethodPost {
			m.handleAdopt(w, r, "/"+segs[0]+"/"+segs[1], segs[2])
		} else {
			apiErr(w, http.StatusNotFound, "not_found")
		}
	default:
		apiErr(w, http.StatusNotFound, "not_found")
	}
}

func (m *mockUAPI) decode(r *http.Request) map[string]any {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body == nil {
		body = map[string]any{}
	}
	return body
}

func (m *mockUAPI) handleCollectionCreate(w http.ResponseWriter, r *http.Request, coll string) {
	body := m.decode(r)
	m.counter++
	id := fmt.Sprintf("x_%d", m.counter)
	body["id"] = id
	body["managed"] = true
	if m.store[coll] == nil {
		m.store[coll] = map[string]map[string]any{}
	}
	m.store[coll][id] = body
	writeJSON(w, http.StatusOK, m.withRuntime(coll, body), etagOf(body))
}

func (m *mockUAPI) handleItem(w http.ResponseWriter, r *http.Request, coll, id string) {
	obj := m.store[coll][id]
	switch r.Method {
	case http.MethodGet:
		if obj == nil {
			apiErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, m.withRuntime(coll, obj), etagOf(obj))
	case http.MethodPut, http.MethodPatch:
		if obj == nil {
			apiErr(w, http.StatusNotFound, "not_found")
			return
		}
		if im := m.ifMatch(r); im != "" && im != etagOf(obj) {
			apiErr(w, http.StatusPreconditionFailed, "precondition_failed")
			return
		}
		body := m.decode(r)
		if r.Method == http.MethodPut {
			body["id"] = id
			body["managed"] = obj["managed"]
			m.store[coll][id] = body
		} else {
			for k, v := range body {
				obj[k] = v
			}
		}
		writeJSON(w, http.StatusOK, m.withRuntime(coll, m.store[coll][id]), etagOf(m.store[coll][id]))
	case http.MethodDelete:
		if obj == nil {
			apiErr(w, http.StatusNotFound, "not_found")
			return
		}
		if im := m.ifMatch(r); im != "" && im != etagOf(obj) {
			apiErr(w, http.StatusPreconditionFailed, "precondition_failed")
			return
		}
		delete(m.store[coll], id)
		w.WriteHeader(http.StatusNoContent)
	default:
		apiErr(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func (m *mockUAPI) handleAdopt(w http.ResponseWriter, r *http.Request, coll, id string) {
	obj := m.store[coll][id]
	if obj == nil {
		apiErr(w, http.StatusNotFound, "not_found")
		return
	}
	m.counter++
	newID := fmt.Sprintf("x_%d", m.counter)
	delete(m.store[coll], id)
	obj["id"] = newID
	obj["managed"] = true
	m.store[coll][newID] = obj
	writeJSON(w, http.StatusOK, obj, etagOf(obj))
}

func (m *mockUAPI) handleSingleton(w http.ResponseWriter, r *http.Request, path string) {
	if m.store[path] == nil {
		m.store[path] = map[string]map[string]any{"_": {"id": "cfg_sys", "managed": false}}
	}
	obj := m.store[path]["_"]
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, obj, etagOf(obj))
	case http.MethodPatch:
		if im := m.ifMatch(r); im != "" && im != etagOf(obj) {
			apiErr(w, http.StatusPreconditionFailed, "precondition_failed")
			return
		}
		for k, v := range m.decode(r) {
			obj[k] = v
		}
		writeJSON(w, http.StatusOK, obj, etagOf(obj))
	default:
		apiErr(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func (m *mockUAPI) handleAuthKeys(w http.ResponseWriter, r *http.Request, id string) {
	coll := "/system/authorized_keys"
	if m.store[coll] == nil {
		m.store[coll] = map[string]map[string]any{}
	}
	switch {
	case id == "" && r.Method == http.MethodPost:
		body := m.decode(r)
		line, _ := body["key"].(string)
		parts := strings.Fields(line)
		typ, comment := "", ""
		if len(parts) > 0 {
			typ = parts[0]
		}
		if len(parts) > 2 {
			comment = parts[2]
		}
		sum := sha256.Sum256([]byte(line))
		kid := hex.EncodeToString(sum[:])[:12]
		obj := map[string]any{"id": kid, "type": typ, "comment": comment}
		m.store[coll][kid] = obj
		writeJSON(w, http.StatusOK, obj, etagOf(obj))
	case id != "" && r.Method == http.MethodGet:
		obj := m.store[coll][id]
		if obj == nil {
			apiErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, obj, etagOf(obj))
	case id != "" && r.Method == http.MethodDelete:
		delete(m.store[coll], id)
		w.WriteHeader(http.StatusNoContent)
	default:
		apiErr(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

// withRuntime injects a canned runtime block so runtime data-source tests have data.
func (m *mockUAPI) withRuntime(coll string, obj map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range obj {
		out[k] = v
	}
	switch coll {
	case "/network/interfaces":
		if _, ok := out["private_key"]; ok {
			out["has_private_key"] = true
			delete(out, "private_key")
		}
		out["runtime"] = map[string]any{
			"up": true, "pending": false, "available": true,
			"l3_device": "br-lan", "uptime": 1234,
			"ipv4-address": []any{map[string]any{"address": "192.168.1.1", "mask": 24}},
			"route":        []any{map[string]any{"target": "0.0.0.0", "mask": 0, "nexthop": "192.168.1.254"}},
		}
	case "/wireless/interfaces":
		if _, ok := out["key"]; ok {
			out["has_key"] = true
			delete(out, "key")
		}
		out["runtime"] = map[string]any{"ifname": "wlan0", "channel": 36, "signal": -52, "assoclist_count": 3}
	}
	return out
}
