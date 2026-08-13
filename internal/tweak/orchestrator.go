package tweak

import (
	"fmt"
	"strings"

	"winforge/internal/audit"
	"winforge/internal/config"
)

// Effect records the outcome of a single operation within a tweak.
type Effect struct {
	OperationType string
	Target        string
	PreviousValue string
	NewValue      string
	// Changed is true when applying would alter (or has altered) system state.
	Changed bool
	Applied bool
	Err     error
}

// Result is the aggregate outcome of applying (or dry-running) one tweak.
type Result struct {
	TweakID   string
	DryRun    bool
	Effects   []Effect
	Succeeded int
	Failed    int
	Changed   int
}

// Orchestrator applies tweaks through an Executor and records every mutation
// in the audit log so it can be surfaced in History and undone later.
type Orchestrator struct {
	exec Executor
	log  *audit.Logger
}

// NewOrchestrator creates an orchestrator. log may be nil to disable auditing.
func NewOrchestrator(exec Executor, log *audit.Logger) *Orchestrator {
	return &Orchestrator{exec: exec, log: log}
}

// Apply executes every operation in a tweak. When dryRun is true, it reads
// current state and reports what would change without mutating anything.
func (o *Orchestrator) Apply(t config.Tweak, dryRun bool) Result {
	res := Result{TweakID: t.ID, DryRun: dryRun}
	for i := range t.Operations {
		op := t.Operations[i]
		eff := o.applyOp(t, op, dryRun)
		res.Effects = append(res.Effects, eff)
		if eff.Err != nil {
			res.Failed++
		} else {
			res.Succeeded++
			if eff.Changed || eff.Applied {
				res.Changed++
			}
		}
	}
	return res
}

// applyOp executes (or simulates) a single operation.
func (o *Orchestrator) applyOp(t config.Tweak, op config.Operation, dryRun bool) Effect {
	eff := Effect{OperationType: op.Type, Target: opTarget(op)}
	switch op.Type {
	case config.OpRegistrySetDword:
		val, err := op.DwordValue()
		if err != nil {
			eff.Err = err
			return eff
		}
		cur, exists, err := o.exec.RegistryGetDword(op.Hive, op.Path, op.Name)
		if err != nil {
			eff.Err = err
			return eff
		}
		eff.PreviousValue = fmt.Sprintf("%d", cur)
		eff.NewValue = fmt.Sprintf("%d", val)
		if !exists || cur != val {
			eff.Changed = true
		}
		if !dryRun && eff.Changed {
			if err := o.exec.RegistrySetDword(op.Hive, op.Path, op.Name, val); err != nil {
				eff.Err = err
				return eff
			}
			eff.Applied = true
			o.record(t, op, eff, nil)
		}
		return eff

	case config.OpRegistrySetString:
		val, err := op.StringValue()
		if err != nil {
			eff.Err = err
			return eff
		}
		cur, exists, err := o.exec.RegistryGetString(op.Hive, op.Path, op.Name)
		if err != nil {
			eff.Err = err
			return eff
		}
		eff.PreviousValue = cur
		eff.NewValue = val
		if !exists || cur != val {
			eff.Changed = true
		}
		if !dryRun && eff.Changed {
			if err := o.exec.RegistrySetString(op.Hive, op.Path, op.Name, val); err != nil {
				eff.Err = err
				return eff
			}
			eff.Applied = true
			o.record(t, op, eff, nil)
		}
		return eff

	case config.OpRegistryDelete:
		if o.valueExists(op.Hive, op.Path, op.Name) {
			eff.Changed = true
		}
		if !dryRun && eff.Changed {
			if err := o.exec.RegistryDeleteValue(op.Hive, op.Path, op.Name); err != nil {
				eff.Err = err
				return eff
			}
			eff.Applied = true
			o.record(t, op, eff, nil)
		}
		return eff

	case config.OpServiceStartMode:
		mode, err := op.StringValue()
		if err != nil {
			eff.Err = err
			return eff
		}
		cur, err := o.exec.ServiceGetStartMode(op.Name)
		if err != nil {
			eff.Err = err
			return eff
		}
		eff.PreviousValue = cur
		eff.NewValue = mode
		if !strings.EqualFold(cur, mode) {
			eff.Changed = true
		}
		if !dryRun && eff.Changed {
			if err := o.exec.ServiceSetStartMode(op.Name, mode); err != nil {
				eff.Err = err
				return eff
			}
			eff.Applied = true
			o.record(t, op, eff, nil)
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
		if err := o.execUnary(op); err != nil {
			eff.Err = err
			return eff
		}
		eff.Applied = true
		o.record(t, op, eff, nil)
		return eff

	default:
		eff.Err = fmt.Errorf("unknown operation type %q", op.Type)
		return eff
	}
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

func (o *Orchestrator) record(t config.Tweak, op config.Operation, eff Effect, execErr error) {
	if o.log == nil {
		return
	}
	e := audit.Entry{
		OperationType: op.Type,
		Target:        eff.Target,
		PreviousValue: eff.PreviousValue,
		NewValue:      eff.NewValue,
		Success:       execErr == nil,
		CanUndo:       t.Reversible,
		TweakID:       t.ID,
	}
	if isRegistryOp(op.Type) {
		e.RegistryHive = op.Hive
		e.RegistryPath = op.Path
		e.RegistryName = op.Name
	}
	if execErr != nil {
		e.ErrorMessage = execErr.Error()
	}
	_ = o.log.Append(e)
}

func isRegistryOp(opType string) bool {
	return opType == config.OpRegistrySetDword ||
		opType == config.OpRegistrySetString ||
		opType == config.OpRegistryDelete
}

// UndoEntry reverses a single recorded registry operation, restoring the value
// captured at application time. Non-registry entries return an error.
func (o *Orchestrator) UndoEntry(e audit.Entry) error {
	if e.RegistryHive == "" || e.RegistryPath == "" || e.RegistryName == "" {
		return fmt.Errorf("operation %s cannot be undone (no registry location)", e.ID)
	}
	switch e.OperationType {
	case config.OpRegistrySetDword:
		if e.PreviousValue == "" {
			return o.exec.RegistryDeleteValue(e.RegistryHive, e.RegistryPath, e.RegistryName)
		}
		var v uint32
		if _, err := fmt.Sscanf(e.PreviousValue, "%d", &v); err != nil {
			return fmt.Errorf("cannot parse previous dword %q: %w", e.PreviousValue, err)
		}
		return o.exec.RegistrySetDword(e.RegistryHive, e.RegistryPath, e.RegistryName, v)
	case config.OpRegistrySetString:
		if e.PreviousValue == "" {
			return o.exec.RegistryDeleteValue(e.RegistryHive, e.RegistryPath, e.RegistryName)
		}
		return o.exec.RegistrySetString(e.RegistryHive, e.RegistryPath, e.RegistryName, e.PreviousValue)
	case config.OpRegistryDelete:
		return fmt.Errorf("deleted value for %s was not captured; cannot undo", e.Target)
	default:
		return fmt.Errorf("operation %s cannot be undone", e.OperationType)
	}
}

// Undo reverses a previously applied tweak using its explicit revert list.
// When no revert list is present, registry set operations are best-effort
// reversed by deleting the value.
func (o *Orchestrator) Undo(t config.Tweak) Result {
	res := Result{TweakID: t.ID}
	revert := t.Revert
	if len(revert) == 0 {
		revert = deriveRevert(t.Operations)
	}
	for i := range revert {
		op := revert[i]
		eff := Effect{OperationType: op.Type, Target: opTarget(op)}
		switch op.Type {
		case config.OpRegistrySetDword:
			val, err := op.DwordValue()
			if err != nil {
				eff.Err = err
			} else if err := o.exec.RegistrySetDword(op.Hive, op.Path, op.Name, val); err != nil {
				eff.Err = err
			} else {
				eff.Applied = true
			}
		case config.OpRegistrySetString:
			val, err := op.StringValue()
			if err != nil {
				eff.Err = err
			} else if err := o.exec.RegistrySetString(op.Hive, op.Path, op.Name, val); err != nil {
				eff.Err = err
			} else {
				eff.Applied = true
			}
		case config.OpRegistryDelete:
			if err := o.exec.RegistryDeleteValue(op.Hive, op.Path, op.Name); err != nil {
				eff.Err = err
			} else {
				eff.Applied = true
			}
		default:
			if err := o.execUnary(op); err != nil {
				eff.Err = err
			} else {
				eff.Applied = true
			}
		}
		res.Effects = append(res.Effects, eff)
		if eff.Err != nil {
			res.Failed++
		} else {
			res.Succeeded++
			if eff.Applied {
				res.Changed++
			}
		}
		if eff.Applied && o.log != nil {
			o.record(t, op, eff, eff.Err)
		}
	}
	return res
}

// IsApplied reports whether every stateful operation in a tweak already
// matches its target state.
func (o *Orchestrator) IsApplied(t config.Tweak) bool {
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
		case config.OpRegistryDelete:
			if o.valueExists(op.Hive, op.Path, op.Name) {
				return false
			}
		default:
			// Stateful/non-readable operations are never considered "applied".
			return false
		}
	}
	return true
}

// valueExists reports whether a registry value exists with either a string or
// DWORD type.
func (o *Orchestrator) valueExists(hive, path, name string) bool {
	if _, ok, err := o.exec.RegistryGetString(hive, path, name); err == nil && ok {
		return true
	}
	if _, ok, err := o.exec.RegistryGetDword(hive, path, name); err == nil && ok {
		return true
	}
	return false
}

// deriveRevert builds a best-effort revert for registry set operations.
func deriveRevert(ops []config.Operation) []config.Operation {
	var out []config.Operation
	for _, op := range ops {
		switch op.Type {
		case config.OpRegistrySetDword, config.OpRegistrySetString:
			out = append(out, config.Operation{Type: config.OpRegistryDelete, Hive: op.Hive, Path: op.Path, Name: op.Name})
		}
	}
	return out
}

// opTarget builds a human-readable target string for logs and UI.
func opTarget(op config.Operation) string {
	switch op.Type {
	case config.OpRegistrySetDword, config.OpRegistrySetString, config.OpRegistryDelete:
		return fmt.Sprintf("%s\\%s\\%s", op.Hive, op.Path, op.Name)
	case config.OpServiceStartMode, config.OpServiceStart, config.OpServiceStop:
		return "service:" + op.Name
	case config.OpTaskDisable, config.OpTaskEnable, config.OpTaskDelete:
		return "task:" + op.Path
	case config.OpAppxRemove:
		return "appx:" + op.Name
	case config.OpCommand:
		s, _ := op.StringValue()
		return "cmd:" + s
	default:
		return op.Type
	}
}
