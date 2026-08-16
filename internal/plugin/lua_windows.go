//go:build windows

// Package plugin: Windows Lua host.
//
// This file binds lua54.dll through syscall.NewLazyDLL-style loading — the
// same cgo-free pattern used by internal/registry and internal/winapi —
// keeping the engine stdlib-only and CGO_ENABLED=0. The DLL is loaded by
// absolute path from one of two WinForge-controlled directories (next to the
// executable, then the data directory); it is never resolved through PATH or
// the normal DLL search order.
//
// Hostile-script containment relies on: (1) the platform-independent
// validation in lua.go — every proposed operation runs through
// config.validateOperation; (2) a whitelisted API surface with os/io/debug/
// package/loadfile removed; (3) an instruction-count hook (lua_sethook
// LUA_MASKCOUNT) that longjmps out of runaway scripts.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"winforge/internal/config"
)

// Lua 5.4 constants.
const (
	luaOk = 0

	luaTNil        = 0
	luaTBoolean    = 1
	luaTNumber     = 3
	luaTString     = 4
	luaTTable      = 5
	luaTStringMask = 0xFF
	luaMaskCount   = 1 << 2
)

// luaMultiRet is LUA_MULTRET (-1). It is declared as a uintptr-sized value so
// it can be passed directly to syscall.Proc.Call without a negative-to-uintptr
// conversion that overflows at compile time.
var luaMultiRet = uintptr(^uintptr(0))

// windowsHost is a ScriptHost backed by lua54.dll.
type windowsHost struct {
	dll *syscall.DLL
}

// currentHost is the host of the Run currently on this goroutine. Plugin
// discovery runs sequentially on one goroutine, and every callback runs on
// that same OS thread (Run locks the thread), so a package variable is enough
// for the static C callbacks to recover the host.
var currentHost *windowsHost

// stateTable maps a lua_State pointer to its *luaAPI, so the static C
// callbacks can recover the request context.
var (
	stateMu    sync.Mutex
	stateTable = map[uintptr]*luaAPI{}
)

// callback addresses (allocated once per process).
var (
	cbOnce sync.Once
	cbRegistrySet, cbRegistryDelete, cbServiceSetStartMode, cbRevert, cbLog,
	cbTweak, cbCommit, cbHook uintptr
)

// loadLuaHost locates lua54.dll under dllDirs (in order), loads it by
// absolute path, and verifies every required export is present.
func loadLuaHost(dllDirs []string) (ScriptHost, error) {
	var path string
	for _, dir := range dllDirs {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, "lua54.dll")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			path = candidate
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("%w: lua54.dll not found in %v", ErrLuaUnavailable, dllDirs)
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, fmt.Errorf("%w: load lua54.dll: %v", ErrLuaUnavailable, err)
	}
	required := []string{
		"luaL_newstate", "luaL_openlibs", "luaL_loadstring",
		"lua_pcallk", "lua_getfield", "lua_setfield", "lua_createtable",
		"lua_settop", "lua_gettop", "lua_type", "lua_tolstring",
		"lua_tointegerx", "lua_toboolean", "lua_next", "lua_pushnil",
		"lua_pushcclosure", "lua_pushstring", "lua_pushinteger",
		"lua_pushboolean", "lua_setglobal", "lua_error", "lua_sethook",
		"lua_close",
	}
	for _, name := range required {
		if _, err := dll.FindProc(name); err != nil {
			dll.Release()
			return nil, fmt.Errorf("%w: lua54.dll missing %s: %v", ErrLuaUnavailable, name, err)
		}
	}
	return &windowsHost{dll: dll}, nil
}

func (h *windowsHost) p(name string) *syscall.Proc {
	p, _ := h.dll.FindProc(name)
	return p
}

// Run evaluates source in a fresh sandboxed Lua state and returns the tweaks
// the script proposed.
func (h *windowsHost) Run(source string) ([]config.Tweak, []string, error) {
	// A C callback may longjmp (via lua_error) out of a Go frame. Pin the
	// goroutine to one OS thread for the lifetime of the state so any stack
	// that the longjmp abandons stays within the thread Lua is using.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	prevHost := currentHost
	currentHost = h
	defer func() { currentHost = prevHost }()

	l, _, _ := h.p("luaL_newstate").Call()
	if l == 0 {
		return nil, nil, fmt.Errorf("luaL_newstate returned null")
	}
	defer h.p("lua_close").Call(l)

	api := newLuaAPI()
	stateMu.Lock()
	stateTable[l] = api
	stateMu.Unlock()
	defer func() {
		stateMu.Lock()
		delete(stateTable, l)
		stateMu.Unlock()
	}()

	h.p("luaL_openlibs").Call(l)
	// Remove dangerous globals: filesystem/process access, the module loader,
	// and bytecode/source loaders so a pack cannot escape the interpreter.
	for _, name := range []string{"os", "io", "debug", "package", "dofile", "loadfile", "load", "loadstring", "require"} {
		h.pushNil(l)
		h.setGlobal(l, name)
	}
	// Redirect print to the bounded winforge.log channel.
	h.pushCClosure(l, h.callback("log"), 0)
	h.setGlobal(l, "print")

	h.installCallbacks()
	h.installHook(l)

	if err := h.registerAPI(l); err != nil {
		return nil, nil, err
	}

	srcPtr, free, err := lstringPtr(source)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid script source: %w", err)
	}
	defer free()

	if r, _, _ := h.p("luaL_loadstring").Call(l, srcPtr); r != luaOk {
		return nil, nil, fmt.Errorf("load script: %s", h.toString(l, -1))
	}
	// pcallk(L, nargs=0, nresults=LUA_MULTRET, errfunc=0, ctx=0, k=NULL)
	if r, _, _ := h.p("lua_pcallk").Call(l, 0, uintptr(luaMultiRet), 0, 0, 0); r != luaOk {
		return nil, append([]string(nil), api.logs...), fmt.Errorf("run script: %s", h.toString(l, -1))
	}

	return api.tweaks(), append([]string(nil), api.logs...), nil
}

func (h *windowsHost) installCallbacks() {
	cbOnce.Do(func() {
		cbRegistrySet = mustNewCallback(hostRegistrySet)
		cbRegistryDelete = mustNewCallback(hostRegistryDelete)
		cbServiceSetStartMode = mustNewCallback(hostServiceSetStartMode)
		cbRevert = mustNewCallback(hostRevert)
		cbLog = mustNewCallback(hostLog)
		cbTweak = mustNewCallback(hostTweak)
		cbCommit = mustNewCallback(hostCommit)
		cbHook = mustNewCallback(hostHook)
	})
}

func (h *windowsHost) callback(name string) uintptr {
	switch name {
	case "registry.set":
		return cbRegistrySet
	case "registry.delete":
		return cbRegistryDelete
	case "service.set_start_mode":
		return cbServiceSetStartMode
	case "revert":
		return cbRevert
	case "log", "print":
		return cbLog
	case "tweak":
		return cbTweak
	case "commit":
		return cbCommit
	case "hook":
		return cbHook
	default:
		panic("unknown lua callback " + name)
	}
}

func mustNewCallback(fn any) uintptr {
	// On Windows syscall.NewCallback never returns an error; it panics if the
	// function has an unsupported signature. The build tag makes this file
	// Windows-only so the Linux stub (which does not reference it) still builds.
	return syscall.NewCallback(fn)
}

// registerAPI builds the global "winforge" table with the registry/service
// subtables and the log/tweak/revert functions. Stack discipline: every push
// is matched by a setfield (which pops its value).
func (h *windowsHost) registerAPI(l uintptr) error {
	// winforge table (narr=0, nrec=5).
	h.p("lua_createtable").Call(l, 0, 5)

	// winforge.registry = { set, delete }
	h.p("lua_createtable").Call(l, 0, 2)
	h.pushCClosure(l, h.callback("registry.set"), 0)
	h.setField(l, -2, "set")
	h.pushCClosure(l, h.callback("registry.delete"), 0)
	h.setField(l, -2, "delete")
	h.setField(l, -2, "registry")

	// winforge.service = { set_start_mode }
	h.p("lua_createtable").Call(l, 0, 1)
	h.pushCClosure(l, h.callback("service.set_start_mode"), 0)
	h.setField(l, -2, "set_start_mode")
	h.setField(l, -2, "service")

	// winforge.log, winforge.tweak, winforge.revert
	h.pushCClosure(l, h.callback("log"), 0)
	h.setField(l, -2, "log")
	h.pushCClosure(l, h.callback("tweak"), 0)
	h.setField(l, -2, "tweak")
	h.pushCClosure(l, h.callback("revert"), 0)
	h.setField(l, -2, "revert")

	// Pop the winforge table into the globals.
	if err := h.setGlobal(l, "winforge"); err != nil {
		return err
	}
	return nil
}

func (h *windowsHost) installHook(l uintptr) {
	h.p("lua_sethook").Call(l, h.callback("hook"), uintptr(luaMaskCount), uintptr(luaInstructionBudget))
}

// --- stack primitives ---

func (h *windowsHost) pushNil(l uintptr) { h.p("lua_pushnil").Call(l) }

func (h *windowsHost) pushBool(l uintptr, b bool) {
	var v uintptr
	if b {
		v = 1
	}
	h.p("lua_pushboolean").Call(l, v)
}

func (h *windowsHost) pushInt(l uintptr, n int64) {
	h.p("lua_pushinteger").Call(l, uintptr(n))
}

func (h *windowsHost) pushString(l uintptr, s string) error {
	p, free, err := lstringPtr(s)
	if err != nil {
		return err
	}
	defer free()
	h.p("lua_pushstring").Call(l, p)
	return nil
}

func (h *windowsHost) setField(l uintptr, idx int, name string) {
	p, free, _ := lstringPtr(name)
	defer free()
	h.p("lua_setfield").Call(l, uintptr(idx), p)
}

func (h *windowsHost) setGlobal(l uintptr, name string) error {
	p, free, err := lstringPtr(name)
	if err != nil {
		return err
	}
	defer free()
	h.p("lua_setglobal").Call(l, p)
	return nil
}

func (h *windowsHost) pushCClosure(l, fn uintptr, nup uintptr) {
	h.p("lua_pushcclosure").Call(l, fn, nup)
}

func (h *windowsHost) settop(l uintptr, idx int) { h.p("lua_settop").Call(l, uintptr(idx)) }

func (h *windowsHost) gettop(l uintptr) int {
	r, _, _ := h.p("lua_gettop").Call(l)
	return int(r)
}

func (h *windowsHost) luaType(l uintptr, idx int) int {
	r, _, _ := h.p("lua_type").Call(l, uintptr(idx))
	return int(r)
}

func (h *windowsHost) isString(l uintptr, idx int) bool {
	return h.luaType(l, idx)&luaTStringMask == luaTString
}

func (h *windowsHost) toString(l uintptr, idx int) string {
	var n uintptr
	r, _, _ := h.p("lua_tolstring").Call(l, uintptr(idx), uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		return ""
	}
	return goStringN(r, int(n))
}

func (h *windowsHost) toBool(l uintptr, idx int) bool {
	r, _, _ := h.p("lua_toboolean").Call(l, uintptr(idx))
	return r != 0
}

func (h *windowsHost) toInt(l uintptr, idx int) (int64, bool) {
	var isnum int32
	r, _, _ := h.p("lua_tointegerx").Call(l, uintptr(idx), uintptr(unsafe.Pointer(&isnum)))
	return int64(r), isnum != 0
}

// raiseError pushes msg and calls lua_error, which longjmps and does not
// return through Go. The method "returns" to satisfy host code paths that
// follow it; those paths are unreachable.
func (h *windowsHost) raiseError(l uintptr, msg string) {
	_ = h.pushString(l, msg)
	h.p("lua_error").Call(l)
}

// checkStringArg reads a string argument, raising a Lua error if absent.
func (h *windowsHost) checkStringArg(l uintptr, arg int, what string) string {
	if !h.isString(l, arg) {
		h.raiseError(l, fmt.Sprintf("%s expects a string argument", what))
		return ""
	}
	return h.toString(l, arg)
}

// tableToMap decodes the Lua table at idx into a Go map. Nested tables,
// strings, integers, and booleans are supported; functions/userdata/threads
// are rejected (a proposal must be plain data). Non-string keys are skipped.
// On return, the stack is unchanged (the table remains at idx).
func (h *windowsHost) tableToMap(l uintptr, idx int) map[string]any {
	t := idx
	if idx < 0 {
		t = idx - 1 // account for the key pushed by lua_pushnil below
	}
	out := map[string]any{}
	h.pushNil(l)
	for {
		r, _, _ := h.p("lua_next").Call(l, uintptr(t))
		if r == 0 {
			break
		}
		// key at -2, value at -1.
		if !h.isString(l, -2) {
			h.settop(l, -2) // pop value, keep key
			continue
		}
		key := h.toString(l, -2)
		val, ok := h.decodeValue(l, -1)
		h.settop(l, -2) // pop value, keep key for next iteration
		if !ok {
			return nil
		}
		out[key] = val
	}
	return out
}

// decodeValue decodes the Lua value at idx without removing it.
func (h *windowsHost) decodeValue(l uintptr, idx int) (any, bool) {
	switch h.luaType(l, idx) & luaTStringMask {
	case luaTNil:
		return nil, true
	case luaTBoolean:
		return h.toBool(l, idx), true
	case luaTString:
		return h.toString(l, idx), true
	case luaTNumber:
		n, ok := h.toInt(l, idx)
		if !ok {
			return nil, false // non-integral numbers are not accepted in proposals
		}
		return n, true
	case luaTTable:
		return h.tableToMap(l, idx), true
	default:
		return nil, false
	}
}

// pushValue pushes a Go value (nil/bool/string/integral-number/table) onto
// the Lua stack.
func (h *windowsHost) pushValue(l uintptr, v any) error {
	switch x := v.(type) {
	case nil:
		h.pushNil(l)
	case bool:
		h.pushBool(l, x)
	case string:
		return h.pushString(l, x)
	case int64:
		h.pushInt(l, x)
	case int:
		h.pushInt(l, int64(x))
	case uint32:
		h.pushInt(l, int64(x))
	case uint64:
		h.pushInt(l, int64(x))
	case float64:
		h.pushInt(l, int64(x)) // proposals only carry integral numbers
	case map[string]any:
		h.p("lua_createtable").Call(l, 0, uintptr(len(x)))
		for k, vv := range x {
			if err := h.pushValue(l, vv); err != nil {
				return err
			}
			h.setField(l, -2, k)
		}
	default:
		return fmt.Errorf("cannot push value of type %T to Lua", v)
	}
	return nil
}

// --- C callbacks. Each is `int (*)(lua_State*)`; the uintptr result is the
// number of values left on the stack. Validation errors call raiseError,
// which longjmps and never returns through Go. ---

func stateAPI(l uintptr) *luaAPI {
	stateMu.Lock()
	api := stateTable[l]
	stateMu.Unlock()
	return api
}

//export hostRegistrySet
func hostRegistrySet(l uintptr) uintptr {
	api := stateAPI(l)
	h := currentHost
	if h == nil || api == nil {
		return 0
	}
	if h.gettop(l) < 5 {
		h.raiseError(l, "registry.set requires (hive, path, name, kind, value)")
		return 0
	}
	hive := h.checkStringArg(l, 1, "registry.set hive")
	path := h.checkStringArg(l, 2, "registry.set path")
	if !h.isString(l, 3) {
		h.raiseError(l, "registry.set name must be a string")
		return 0
	}
	name := h.toString(l, 3)
	kind := h.checkStringArg(l, 4, "registry.set kind")
	val, ok := h.decodeValue(l, 5)
	if !ok {
		h.raiseError(l, "registry.set value must be a string, integer, boolean, or table")
		return 0
	}
	handle, err := api.registrySet(hive, path, name, kind, val)
	if err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	if err := h.pushValue(l, handle); err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	return 1
}

//export hostRegistryDelete
func hostRegistryDelete(l uintptr) uintptr {
	api := stateAPI(l)
	h := currentHost
	if h == nil || api == nil {
		return 0
	}
	if h.gettop(l) < 3 {
		h.raiseError(l, "registry.delete requires (hive, path, name)")
		return 0
	}
	hive := h.checkStringArg(l, 1, "registry.delete hive")
	path := h.checkStringArg(l, 2, "registry.delete path")
	name := h.checkStringArg(l, 3, "registry.delete name")
	handle, err := api.registryDelete(hive, path, name)
	if err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	if err := h.pushValue(l, handle); err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	return 1
}

//export hostServiceSetStartMode
func hostServiceSetStartMode(l uintptr) uintptr {
	api := stateAPI(l)
	h := currentHost
	if h == nil || api == nil {
		return 0
	}
	if h.gettop(l) < 2 {
		h.raiseError(l, "service.set_start_mode requires (name, mode)")
		return 0
	}
	name := h.checkStringArg(l, 1, "service.set_start_mode name")
	mode := h.checkStringArg(l, 2, "service.set_start_mode mode")
	handle, err := api.serviceSetStartMode(name, mode)
	if err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	if err := h.pushValue(l, handle); err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	return 1
}

//export hostRevert
func hostRevert(l uintptr) uintptr {
	api := stateAPI(l)
	h := currentHost
	if h == nil || api == nil {
		return 0
	}
	if h.luaType(l, 1)&luaTStringMask != luaTTable {
		h.raiseError(l, "winforge.revert expects an operation handle")
		return 0
	}
	handle := h.tableToMap(l, 1)
	if handle == nil {
		h.raiseError(l, "winforge.revert: invalid operation handle")
		return 0
	}
	if _, err := api.revert(handle); err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	if err := h.pushValue(l, handle); err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	return 1
}

//export hostLog
func hostLog(l uintptr) uintptr {
	api := stateAPI(l)
	h := currentHost
	if h == nil || api == nil {
		return 0
	}
	var msg string
	if h.gettop(l) >= 1 && h.isString(l, 1) {
		msg = h.toString(l, 1)
	}
	if err := api.log(msg); err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	return 0
}

//export hostTweak
func hostTweak(l uintptr) uintptr {
	api := stateAPI(l)
	h := currentHost
	if h == nil || api == nil {
		return 0
	}
	if h.luaType(l, 1)&luaTStringMask != luaTTable {
		h.raiseError(l, "winforge.tweak expects a table of fields")
		return 0
	}
	fields := h.tableToMap(l, 1)
	if fields == nil {
		h.raiseError(l, "winforge.tweak: invalid fields table")
		return 0
	}
	if _, err := api.beginTweak(fields); err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	// Build a tweak handle: a table { commit = closure }. The closure has no
	// upvalues; beginTweak enforces one open tweak at a time, so commit
	// finalizes api.current.
	h.p("lua_createtable").Call(l, 0, 1)
	h.pushCClosure(l, h.callback("commit"), 0)
	h.setField(l, -2, "commit")
	return 1
}

//export hostCommit
func hostCommit(l uintptr) uintptr {
	api := stateAPI(l)
	h := currentHost
	if h == nil || api == nil {
		return 0
	}
	if api.current == nil {
		h.raiseError(l, "commit called without an open tweak")
		return 0
	}
	if err := api.commit(api.current); err != nil {
		h.raiseError(l, err.Error())
		return 0
	}
	return 0
}

//export hostHook
func hostHook(l uintptr) uintptr {
	h := currentHost
	if h != nil {
		h.raiseError(l, "script exceeded the instruction budget")
	}
	return 0
}

// --- C-string helpers ---

// cptr converts a C-returned uintptr into an unsafe.Pointer. The indirect
// read satisfies go vet's unsafeptr checker (which rejects a direct
// uintptr->Pointer conversion outside the syscall return statement), matching
// the pattern used by golang.org/x/sys for pointers returned from C APIs. The
// underlying memory is owned by lua54.dll (C/heap) and not moved by the Go GC.
func cptr(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

// goStringN copies a known-length C string into a Go string.
func goStringN(p uintptr, n int) string {
	if p == 0 || n <= 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(cptr(p)), n))
}

// lstringPtr returns a pointer to a NUL-terminated copy of s plus a release
// function that keeps the backing allocation alive until the caller is done
// with it. Embedded NULs are rejected so a hostile script cannot truncate a
// registry path mid-string.
func lstringPtr(s string) (uintptr, func(), error) {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return 0, func() {}, fmt.Errorf("string contains a NUL byte at %d", i)
		}
	}
	b := append([]byte(s), 0)
	return uintptr(unsafe.Pointer(&b[0])), func() { _ = b }, nil
}
