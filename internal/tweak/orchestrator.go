package tweak

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"winforge/internal/audit"
	"winforge/internal/config"
	"winforge/internal/power"
	"winforge/internal/service"
)

// Effect records the outcome of a single operation within a tweak.
type Effect struct {
	OperationType         string `json:"operationType"`
	Target                string `json:"target"`
	PreviousValue         string `json:"previousValue,omitempty"`
	PreviousValueCaptured bool   `json:"previousValueCaptured,omitempty"`
	PreviousValueExists   bool   `json:"previousValueExists,omitempty"`
	PreviousValueType     string `json:"previousValueType,omitempty"`
	NewValue              string `json:"newValue,omitempty"`
	// Changed is true when applying would alter (or has altered) system state.
	Changed bool   `json:"changed"`
	Applied bool   `json:"applied"`
	Error   string `json:"error,omitempty"`
	Err     error  `json:"-"`

	attempted bool
}

// Result is the aggregate outcome of applying (or dry-running) one tweak.
type Result struct {
	TweakID   string   `json:"tweakId"`
	DryRun    bool     `json:"dryRun"`
	Effects   []Effect `json:"effects"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Changed   int      `json:"changed"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Failure returns an aggregate error when one or more effects failed.
func (r Result) Failure() error {
	if r.Failed == 0 {
		return nil
	}
	errs := make([]error, 0, r.Failed)
	for _, effect := range r.Effects {
		if effect.Err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", effect.OperationType, effect.Target, effect.Err))
		}
	}
	if len(errs) == 0 {
		return fmt.Errorf("%d operation(s) failed", r.Failed)
	}
	return errors.Join(errs...)
}

// Orchestrator applies tweaks through an Executor and records every attempted
// mutation in the audit log so failures are visible and successful operations
// can be undone later.
type Orchestrator struct {
	exec       Executor
	log        *audit.Logger
	mutationMu sync.Mutex
}

// NewOrchestrator creates an orchestrator. log may be nil to disable auditing.
func NewOrchestrator(exec Executor, log *audit.Logger) *Orchestrator {
	return &Orchestrator{exec: exec, log: log}
}

// Apply executes every operation in a tweak. When dryRun is true, it reads
// current state and reports what would change without mutating anything.
func (o *Orchestrator) Apply(t config.Tweak, dryRun bool) Result {
	if !dryRun {
		o.mutationMu.Lock()
		defer o.mutationMu.Unlock()
	}
	res := Result{TweakID: t.ID, DryRun: dryRun}
	for i := range t.Operations {
		op := t.Operations[i]
		eff := o.applyOp(op, dryRun)
		res.Effects = append(res.Effects, eff)
		if eff.Err != nil {
			res.Failed++
		} else {
			res.Succeeded++
			if eff.Changed || eff.Applied {
				res.Changed++
			}
		}
		if !dryRun && eff.attempted {
			canUndo := eff.Err == nil && t.Reversible && eff.PreviousValueCaptured
			if err := o.record(t, op, eff, eff.Err, canUndo); err != nil {
				res.Warnings = append(res.Warnings, "audit log: "+err.Error())
			}
		}
	}
	return res
}

// applyOp executes (or simulates) a single operation.
func (o *Orchestrator) applyOp(op config.Operation, dryRun bool) Effect {
	eff := Effect{OperationType: op.Type, Target: opTarget(op)}
	switch op.Type {
	case config.OpRegistrySetDword:
		val, err := op.DwordValue()
		if err != nil {
			return effectError(eff, err)
		}
		previous, previousType, exists, err := o.registrySnapshot(op.Hive, op.Path, op.Name)
		if err != nil {
			return effectError(eff, err)
		}
		eff.PreviousValueCaptured = true
		eff.PreviousValueExists = exists
		eff.PreviousValueType = previousType
		eff.PreviousValue = previous
		eff.NewValue = strconv.FormatUint(uint64(val), 10)
		eff.Changed = !exists || previousType != "dword" || previous != eff.NewValue
		if !dryRun && eff.Changed {
			eff.attempted = true
			if err := o.exec.RegistrySetDword(op.Hive, op.Path, op.Name, val); err != nil {
				return effectError(eff, err)
			}
			eff.Applied = true
		}
		return eff

	case config.OpRegistrySetQword:
		val, err := op.QwordValue()
		if err != nil {
			return effectError(eff, err)
		}
		previous, previousType, exists, err := o.registrySnapshot(op.Hive, op.Path, op.Name)
		if err != nil {
			return effectError(eff, err)
		}
		eff.PreviousValueCaptured = true
		eff.PreviousValueExists = exists
		eff.PreviousValueType = previousType
		eff.PreviousValue = previous
		eff.NewValue = strconv.FormatUint(val, 10)
		eff.Changed = !exists || previousType != "qword" || previous != eff.NewValue
		if !dryRun && eff.Changed {
			eff.attempted = true
			if err := o.exec.RegistrySetQword(op.Hive, op.Path, op.Name, val); err != nil {
				return effectError(eff, err)
			}
			eff.Applied = true
		}
		return eff

	case config.OpRegistrySetString:
		val, err := op.StringValue()
		if err != nil {
			return effectError(eff, err)
		}
		previous, previousType, exists, err := o.registrySnapshot(op.Hive, op.Path, op.Name)
		if err != nil {
			return effectError(eff, err)
		}
		eff.PreviousValueCaptured = true
		eff.PreviousValueExists = exists
		eff.PreviousValueType = previousType
		eff.PreviousValue = previous
		eff.NewValue = val
		eff.Changed = !exists || previousType != "string" || previous != val
		if !dryRun && eff.Changed {
			eff.attempted = true
			if err := o.exec.RegistrySetString(op.Hive, op.Path, op.Name, val); err != nil {
				return effectError(eff, err)
			}
			eff.Applied = true
		}
		return eff

	case config.OpRegistryDelete:
		value, valueType, exists, err := o.registrySnapshot(op.Hive, op.Path, op.Name)
		if err != nil {
			return effectError(eff, err)
		}
		eff.PreviousValueCaptured = true
		eff.PreviousValueExists = exists
		eff.PreviousValueType = valueType
		eff.PreviousValue = value
		eff.Changed = exists
		if !dryRun && eff.Changed {
			eff.attempted = true
			if err := o.exec.RegistryDeleteValue(op.Hive, op.Path, op.Name); err != nil {
				return effectError(eff, err)
			}
			eff.Applied = true
		}
		return eff

	case config.OpServiceStartMode:
		mode, err := normalizedServiceStartMode(op)
		if err != nil {
			return effectError(eff, err)
		}
		cur, err := o.exec.ServiceGetStartMode(op.Name)
		if err != nil {
			return effectError(eff, err)
		}
		eff.PreviousValueCaptured = true
		eff.PreviousValueExists = true
		eff.PreviousValueType = "service_start_mode"
		eff.PreviousValue = cur
		eff.NewValue = mode
		eff.Changed = !strings.EqualFold(cur, mode)
		if !dryRun && eff.Changed {
			eff.attempted = true
			if err := o.exec.ServiceSetStartMode(op.Name, mode); err != nil {
				return effectError(eff, err)
			}
			eff.Applied = true
		}
		return eff

	case config.OpPowerScheme:
		val, err := op.StringValue()
		if err != nil {
			return effectError(eff, err)
		}
		guid, err := power.Resolve(val)
		if err != nil {
			return effectError(eff, err)
		}
		cur, err := o.exec.PowerGetActive()
		if err != nil {
			return effectError(eff, err)
		}
		eff.PreviousValueCaptured = true
		eff.PreviousValueExists = true
		eff.PreviousValueType = "power_scheme"
		eff.PreviousValue = cur
		eff.NewValue = guid
		eff.Changed = !strings.EqualFold(cur, guid)
		if !dryRun && eff.Changed {
			eff.attempted = true
			if err := o.exec.PowerSetActive(guid); err != nil {
				return effectError(eff, err)
			}
			eff.Applied = true
		}
		return eff

	case config.OpServiceStart, config.OpServiceStop, config.OpTaskDisable,
		config.OpTaskEnable, config.OpTaskDelete, config.OpAppxRemove, config.OpCommand:
		// These operations either cannot be cheaply read, or are inherently
		// stateful. Dry-run reports them as "would execute".
		eff.Changed = true
		if dryRun {
			return eff
		}
		eff.attempted = true
		if err := o.execUnary(op); err != nil {
			return effectError(eff, err)
		}
		eff.Applied = true
		return eff

	default:
		return effectError(eff, fmt.Errorf("unknown operation type %q", op.Type))
	}
}

func effectError(eff Effect, err error) Effect {
	eff.Err = err
	if err != nil {
		eff.Error = err.Error()
	}
	return eff
}

func (o *Orchestrator) execUnary(op config.Operation) error {
	switch op.Type {
	case config.OpServiceStart:
		return o.exec.ServiceStart(op.Name)
	case config.OpServiceStop:
		return o.exec.ServiceStop(op.Name)
	case config.OpTaskDisable:
		return o.exec.TaskDisable(op.Path)
	case config.OpTaskEnable:
		return o.exec.TaskEnable(op.Path)
	case config.OpTaskDelete:
		return o.exec.TaskDelete(op.Path)
	case config.OpAppxRemove:
		return o.exec.AppxRemove(op.Name)
	case config.OpCommand:
		s, err := op.StringValue()
		if err != nil {
			return err
		}
		return o.exec.RunCommand(s, op.Args)
	default:
		return fmt.Errorf("unhandled unary operation %q", op.Type)
	}
}

func (o *Orchestrator) record(t config.Tweak, op config.Operation, eff Effect, execErr error, canUndo bool) error {
	if o.log == nil {
		return nil
	}
	e := audit.Entry{
		OperationType:         op.Type,
		Target:                eff.Target,
		PreviousValue:         eff.PreviousValue,
		PreviousValueCaptured: eff.PreviousValueCaptured,
		PreviousValueExists:   eff.PreviousValueExists,
		PreviousValueType:     eff.PreviousValueType,
		NewValue:              eff.NewValue,
		Success:               execErr == nil,
		CanUndo:               canUndo,
		TweakID:               t.ID,
	}
	if isRegistryOp(op.Type) {
		e.RegistryHive = op.Hive
		e.RegistryPath = op.Path
		e.RegistryName = op.Name
	}
	if execErr != nil {
		e.ErrorMessage = execErr.Error()
	}
	return o.log.Append(e)
}

func isRegistryOp(opType string) bool {
	return opType == config.OpRegistrySetDword ||
		opType == config.OpRegistrySetString ||
		opType == config.OpRegistrySetQword ||
		opType == config.OpRegistryDelete
}

// UndoEntry reverses a single recorded operation, restoring the state captured
// at application time. Legacy entries are supported where doing so is safe.
func (o *Orchestrator) UndoEntry(e audit.Entry) error {
	o.mutationMu.Lock()
	defer o.mutationMu.Unlock()

	if !e.Success || !e.CanUndo {
		return fmt.Errorf("operation %s cannot be undone", e.ID)
	}
	if o.log != nil {
		entries, err := o.log.ReadAll()
		if err != nil {
			return err
		}
		for _, candidate := range entries {
			if candidate.Success && candidate.UndoOf == e.ID {
				return fmt.Errorf("operation %s has already been undone", e.ID)
			}
		}
	}

	err := o.restoreEntry(e)
	if o.log != nil {
		undo := audit.Entry{
			OperationType: "undo",
			Target:        e.Target,
			Success:       err == nil,
			CanUndo:       false,
			TweakID:       e.TweakID,
			UndoOf:        e.ID,
		}
		if err != nil {
			undo.ErrorMessage = err.Error()
		}
		if logErr := o.log.Append(undo); logErr != nil && err == nil {
			return fmt.Errorf("operation restored, but recording the undo failed: %w", logErr)
		}
	}
	return err
}

func validateRegistryEntryTarget(e audit.Entry) error {
	if !isRegistryOp(e.OperationType) {
		return fmt.Errorf("operation type %q is inconsistent with a registry snapshot", e.OperationType)
	}
	switch e.RegistryHive {
	case "HKLM", "HKCU", "HKCR", "HKU":
	default:
		return fmt.Errorf("invalid registry hive %q", e.RegistryHive)
	}
	if strings.TrimSpace(e.RegistryPath) == "" {
		return errors.New("registry snapshot has no path")
	}
	expectedTarget := fmt.Sprintf("%s\\%s\\%s", e.RegistryHive, e.RegistryPath, e.RegistryName)
	if e.Target != expectedTarget {
		return fmt.Errorf("registry snapshot target %q does not match structured target %q", e.Target, expectedTarget)
	}
	return nil
}

func validateCapturedEntry(e audit.Entry) error {
	if !e.PreviousValueExists {
		if err := validateRegistryEntryTarget(e); err != nil {
			return err
		}
		if e.PreviousValue != "" || e.PreviousValueType != "" {
			return errors.New("missing registry snapshot unexpectedly contains a value or type")
		}
		return nil
	}

	switch e.PreviousValueType {
	case "dword", "qword", "string", "expand_string":
		return validateRegistryEntryTarget(e)
	case "service_start_mode":
		if e.OperationType != config.OpServiceStartMode || !strings.HasPrefix(e.Target, "service:") || strings.TrimSpace(strings.TrimPrefix(e.Target, "service:")) == "" {
			return errors.New("service start-mode snapshot has an inconsistent target")
		}
		if e.RegistryHive != "" || e.RegistryPath != "" || e.RegistryName != "" {
			return errors.New("service start-mode snapshot unexpectedly contains a registry target")
		}
		_, err := service.ParseStartMode(e.PreviousValue)
		return err
	case "power_scheme":
		if e.OperationType != config.OpPowerScheme || !strings.HasPrefix(e.Target, "power:") {
			return errors.New("power-scheme snapshot has an inconsistent target")
		}
		if e.RegistryHive != "" || e.RegistryPath != "" || e.RegistryName != "" {
			return errors.New("power-scheme snapshot unexpectedly contains a registry target")
		}
		if _, err := power.Resolve(strings.TrimPrefix(e.Target, "power:")); err != nil {
			return fmt.Errorf("power-scheme snapshot has invalid requested scheme: %w", err)
		}
		_, err := power.Resolve(e.PreviousValue)
		return err
	default:
		return fmt.Errorf("operation %s has unknown previous value type %q", e.ID, e.PreviousValueType)
	}
}

func (o *Orchestrator) restoreEntry(e audit.Entry) error {
	if e.PreviousValueCaptured {
		if err := validateCapturedEntry(e); err != nil {
			return fmt.Errorf("operation %s has inconsistent snapshot metadata: %w", e.ID, err)
		}
		if !e.PreviousValueExists {
			if e.RegistryHive == "" {
				return fmt.Errorf("operation %s has no restorable target", e.ID)
			}
			_, _, exists, err := o.registrySnapshot(e.RegistryHive, e.RegistryPath, e.RegistryName)
			if err != nil {
				return err
			}
			if !exists {
				return nil
			}
			return o.exec.RegistryDeleteValue(e.RegistryHive, e.RegistryPath, e.RegistryName)
		}
		switch e.PreviousValueType {
		case "dword":
			v, err := strconv.ParseUint(e.PreviousValue, 10, 32)
			if err != nil {
				return fmt.Errorf("cannot parse previous DWORD %q: %w", e.PreviousValue, err)
			}
			return o.exec.RegistrySetDword(e.RegistryHive, e.RegistryPath, e.RegistryName, uint32(v))
		case "qword":
			v, err := strconv.ParseUint(e.PreviousValue, 10, 64)
			if err != nil {
				return fmt.Errorf("cannot parse previous QWORD %q: %w", e.PreviousValue, err)
			}
			return o.exec.RegistrySetQword(e.RegistryHive, e.RegistryPath, e.RegistryName, v)
		case "string":
			return o.exec.RegistrySetString(e.RegistryHive, e.RegistryPath, e.RegistryName, e.PreviousValue)
		case "expand_string":
			return o.exec.RegistrySetExpandString(e.RegistryHive, e.RegistryPath, e.RegistryName, e.PreviousValue)
		case "service_start_mode":
			return o.exec.ServiceSetStartMode(strings.TrimPrefix(e.Target, "service:"), e.PreviousValue)
		case "power_scheme":
			return o.exec.PowerSetActive(e.PreviousValue)
		default:
			return fmt.Errorf("operation %s has unknown previous value type %q", e.ID, e.PreviousValueType)
		}
	}

	// Compatibility with audit entries created before explicit existence/type
	// metadata was added. Empty strings and registry deletions were ambiguous in
	// that format and are deliberately not guessed.
	if err := validateRegistryEntryTarget(e); err != nil {
		return fmt.Errorf("operation %s has inconsistent legacy snapshot metadata: %w", e.ID, err)
	}
	switch e.OperationType {
	case config.OpRegistrySetDword:
		v, err := strconv.ParseUint(e.PreviousValue, 10, 32)
		if err != nil {
			return fmt.Errorf("cannot parse previous DWORD %q: %w", e.PreviousValue, err)
		}
		return o.exec.RegistrySetDword(e.RegistryHive, e.RegistryPath, e.RegistryName, uint32(v))
	case config.OpRegistrySetQword:
		v, err := strconv.ParseUint(e.PreviousValue, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse previous QWORD %q: %w", e.PreviousValue, err)
		}
		return o.exec.RegistrySetQword(e.RegistryHive, e.RegistryPath, e.RegistryName, v)
	case config.OpRegistrySetString:
		if e.PreviousValue == "" {
			return errors.New("legacy audit entry cannot distinguish an empty string from a missing value")
		}
		return o.exec.RegistrySetString(e.RegistryHive, e.RegistryPath, e.RegistryName, e.PreviousValue)
	default:
		return fmt.Errorf("operation %s cannot be safely undone from its legacy audit entry", e.OperationType)
	}
}

// Undo reverses a previously applied tweak using its explicit revert list.
// Refusing to guess protects registry values that existed before WinForge ran.
func (o *Orchestrator) Undo(t config.Tweak) Result {
	o.mutationMu.Lock()
	defer o.mutationMu.Unlock()

	res := Result{TweakID: t.ID}
	if !t.Reversible {
		eff := effectError(Effect{Target: t.ID}, errors.New("tweak is not reversible"))
		res.Effects = []Effect{eff}
		res.Failed = 1
		return res
	}

	revert := t.Revert
	if len(revert) == 0 {
		eff := effectError(Effect{Target: t.ID}, errors.New("tweak has no explicit revert operations"))
		res.Effects = []Effect{eff}
		res.Failed = 1
		return res
	}
	for i := range revert {
		op := revert[i]
		eff := Effect{OperationType: op.Type, Target: opTarget(op), Changed: true, attempted: true}
		var err error
		switch op.Type {
		case config.OpRegistrySetDword:
			var value uint32
			value, err = op.DwordValue()
			if err == nil {
				err = o.exec.RegistrySetDword(op.Hive, op.Path, op.Name, value)
			}
		case config.OpRegistrySetString:
			var value string
			value, err = op.StringValue()
			if err == nil {
				err = o.exec.RegistrySetString(op.Hive, op.Path, op.Name, value)
			}
		case config.OpRegistrySetQword:
			var value uint64
			value, err = op.QwordValue()
			if err == nil {
				err = o.exec.RegistrySetQword(op.Hive, op.Path, op.Name, value)
			}
		case config.OpRegistryDelete:
			err = o.exec.RegistryDeleteValue(op.Hive, op.Path, op.Name)
		case config.OpServiceStartMode:
			var mode string
			mode, err = normalizedServiceStartMode(op)
			if err == nil {
				err = o.exec.ServiceSetStartMode(op.Name, mode)
			}
		case config.OpPowerScheme:
			var value, guid string
			value, err = op.StringValue()
			if err == nil {
				guid, err = power.Resolve(value)
			}
			if err == nil {
				err = o.exec.PowerSetActive(guid)
			}
		default:
			err = o.execUnary(op)
		}
		if err != nil {
			eff = effectError(eff, err)
		} else {
			eff.Applied = true
		}
		res.Effects = append(res.Effects, eff)
		if eff.Err != nil {
			res.Failed++
		} else {
			res.Succeeded++
			res.Changed++
		}
		if logErr := o.record(t, op, eff, eff.Err, false); logErr != nil {
			res.Warnings = append(res.Warnings, "audit log: "+logErr.Error())
		}
	}
	return res
}

// EnsureApplied re-applies every verifiable tweak that is not currently in its
// target state. Tweaks containing commands or other intrinsically unreadable
// operations are skipped so scheduled maintenance never repeats one-way work.
func (o *Orchestrator) EnsureApplied(tweaks []config.Tweak) (applied []string, errs []error) {
	for i := range tweaks {
		t := tweaks[i]
		if !CanVerify(t) || o.IsApplied(t) {
			continue
		}
		res := o.Apply(t, false)
		if err := res.Failure(); err != nil {
			errs = append(errs, fmt.Errorf("tweak %s: %w", t.ID, err))
		} else if res.Changed > 0 {
			applied = append(applied, t.ID)
		}
	}
	return applied, errs
}

// CanVerify reports whether every operation in t exposes readable target state.
func CanVerify(t config.Tweak) bool {
	for _, op := range t.Operations {
		switch op.Type {
		case config.OpRegistrySetDword, config.OpRegistrySetString,
			config.OpRegistrySetQword, config.OpRegistryDelete,
			config.OpServiceStartMode, config.OpPowerScheme:
		default:
			return false
		}
	}
	return true
}

// IsApplied reports whether every operation in a verifiable tweak currently
// matches its target state. It returns false rather than guessing for commands,
// task mutations, Appx removal, or runtime service state.
func (o *Orchestrator) IsApplied(t config.Tweak) bool {
	if !CanVerify(t) {
		return false
	}
	for i := range t.Operations {
		op := t.Operations[i]
		switch op.Type {
		case config.OpRegistrySetDword:
			val, err := op.DwordValue()
			if err != nil {
				return false
			}
			cur, exists, err := o.exec.RegistryGetDword(op.Hive, op.Path, op.Name)
			if err != nil || !exists || cur != val {
				return false
			}
		case config.OpRegistrySetString:
			val, err := op.StringValue()
			if err != nil {
				return false
			}
			cur, exists, err := o.exec.RegistryGetString(op.Hive, op.Path, op.Name)
			if err != nil || !exists || cur != val {
				return false
			}
		case config.OpRegistrySetQword:
			val, err := op.QwordValue()
			if err != nil {
				return false
			}
			cur, exists, err := o.exec.RegistryGetQword(op.Hive, op.Path, op.Name)
			if err != nil || !exists || cur != val {
				return false
			}
		case config.OpRegistryDelete:
			exists, err := o.valueExists(op.Hive, op.Path, op.Name)
			if err != nil || exists {
				return false
			}
		case config.OpServiceStartMode:
			mode, err := normalizedServiceStartMode(op)
			if err != nil {
				return false
			}
			cur, err := o.exec.ServiceGetStartMode(op.Name)
			if err != nil || !strings.EqualFold(cur, mode) {
				return false
			}
		case config.OpPowerScheme:
			val, err := op.StringValue()
			if err != nil {
				return false
			}
			guid, err := power.Resolve(val)
			if err != nil {
				return false
			}
			cur, err := o.exec.PowerGetActive()
			if err != nil || !strings.EqualFold(cur, guid) {
				return false
			}
		}
	}
	return true
}

func (o *Orchestrator) registrySnapshot(hive, path, name string) (value, valueType string, exists bool, err error) {
	var readErrors []error
	if v, ok, readErr := o.exec.RegistryGetString(hive, path, name); readErr == nil {
		if ok {
			return v, "string", true, nil
		}
	} else {
		readErrors = append(readErrors, readErr)
	}
	if v, ok, readErr := o.exec.RegistryGetExpandString(hive, path, name); readErr == nil {
		if ok {
			return v, "expand_string", true, nil
		}
	} else {
		readErrors = append(readErrors, readErr)
	}
	if v, ok, readErr := o.exec.RegistryGetDword(hive, path, name); readErr == nil {
		if ok {
			return strconv.FormatUint(uint64(v), 10), "dword", true, nil
		}
	} else {
		readErrors = append(readErrors, readErr)
	}
	if v, ok, readErr := o.exec.RegistryGetQword(hive, path, name); readErr == nil {
		if ok {
			return strconv.FormatUint(v, 10), "qword", true, nil
		}
	} else {
		readErrors = append(readErrors, readErr)
	}
	if len(readErrors) > 0 {
		return "", "", false, fmt.Errorf("read registry value: %w", errors.Join(readErrors...))
	}
	return "", "", false, nil
}

func (o *Orchestrator) valueExists(hive, path, name string) (bool, error) {
	_, _, exists, err := o.registrySnapshot(hive, path, name)
	return exists, err
}

func normalizedServiceStartMode(op config.Operation) (string, error) {
	value, err := op.StringValue()
	if err != nil {
		return "", err
	}
	mode, err := service.ParseStartMode(value)
	if err != nil {
		return "", err
	}
	return mode.String(), nil
}

// opTarget builds a human-readable target string for logs and UI.
func opTarget(op config.Operation) string {
	switch op.Type {
	case config.OpRegistrySetDword, config.OpRegistrySetString, config.OpRegistrySetQword, config.OpRegistryDelete:
		return fmt.Sprintf("%s\\%s\\%s", op.Hive, op.Path, op.Name)
	case config.OpServiceStartMode, config.OpServiceStart, config.OpServiceStop:
		return "service:" + op.Name
	case config.OpTaskDisable, config.OpTaskEnable, config.OpTaskDelete:
		return "task:" + op.Path
	case config.OpAppxRemove:
		return "appx:" + op.Name
	case config.OpPowerScheme:
		s, _ := op.StringValue()
		return "power:" + s
	case config.OpCommand:
		s, _ := op.StringValue()
		return "cmd:" + s
	default:
		return op.Type
	}
}
